package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

const agentName = "ingest"

// textContentType is the content type recorded for extracted-text objects.
const textContentType = "text/plain; charset=utf-8"

// Fetcher retrieves a document from a URL. The production implementation is an
// HTTP client; tests inject a fake. It returns the raw bytes, the served content
// type, and a filename (from the URL or Content-Disposition).
type Fetcher interface {
	Fetch(ctx context.Context, url string) (data []byte, contentType, filename string, err error)
}

// ObjectStore persists raw bytes and extracted text to object storage (Google
// Cloud Storage in production). Object names are bucket-relative paths; URIs are
// returned as gs:// strings recorded on the SolicitationDoc.
type ObjectStore interface {
	// Stat returns the SHA-256 (hex) previously recorded for object, if it exists.
	// It powers idempotent re-ingestion: an unchanged document is not re-uploaded.
	Stat(ctx context.Context, object string) (sha256hex string, exists bool, err error)
	// Put writes data under object, recording sha256hex as its checksum, and
	// returns the gs:// URI.
	Put(ctx context.Context, object string, data []byte, contentType, sha256hex string) (uri string, err error)
	// Read returns the bytes previously stored under object. It is used only on
	// the idempotent re-ingest path, to load already-extracted text back into
	// memory without re-running extraction.
	Read(ctx context.Context, object string) ([]byte, error)
	// URI returns the gs:// URI for object without writing anything.
	URI(object string) string
}

// Extractor turns raw document bytes into plain text. The production
// implementation is Document AI (documentai.go); DOCX is handled by the stdlib
// extractor (docx.go). Implementations must not fabricate text: when a document
// has no extractable text they return an empty string (not an error).
type Extractor interface {
	ExtractText(ctx context.Context, raw []byte, contentType string) (string, error)
}

// Agent is the document-ingestion stage. Construct it with New and call Ingest
// once per Opportunity.
type Agent struct {
	fetcher Fetcher
	store   ObjectStore
	extract Extractor
}

// New constructs an ingestion Agent from its three collaborators.
func New(f Fetcher, s ObjectStore, e Extractor) *Agent {
	return &Agent{fetcher: f, store: s, extract: e}
}

// Ingest fetches, stores, and extracts every attachment on opp.
//
// It returns one SolicitationDoc per successfully fetched attachment (the raw
// file is always saved, even when text extraction yields nothing), a texts map
// of filename -> extracted text for the documents that produced text (so the
// Manager can thread it straight to the Zone-2 agents without re-reading GCS),
// and an agent.Result describing the outcome:
//
//   - success     — every attachment was fetched and produced text (or there were
//     no attachments to ingest).
//   - needs_human — some attachments failed to fetch, or were fetched but produced
//     no extractable text (e.g. scanned PDFs); a person should look.
//   - failed      — there were attachments but none could be fetched.
//
// Ingest returns a Go error only for invalid input (nil opportunity) or an
// ObjectStore failure it cannot recover from; per-attachment problems are carried
// as result flags so the Manager can route the run without crashing the pipeline.
func (a *Agent) Ingest(ctx context.Context, opp *opportunity.Opportunity) ([]opportunity.SolicitationDoc, map[string]string, *agent.Result, error) {
	if opp == nil {
		return nil, nil, nil, fmt.Errorf("ingest: opportunity must not be nil")
	}

	var (
		docs   []opportunity.SolicitationDoc
		issues []string
	)
	texts := make(map[string]string)

	for i, url := range opp.Attachments {
		doc, text, issue, err := a.ingestOne(ctx, opp.ID, i, url)
		if err != nil {
			// An ObjectStore failure is not recoverable per-document; surface it.
			return nil, nil, a.result(opp.ID, docs, append(issues, err.Error()), agent.StatusFailed), err
		}
		if issue != "" {
			issues = append(issues, issue)
		}
		if doc != nil {
			docs = append(docs, *doc)
			if text != "" {
				texts[doc.Filename] = text
			}
		}
	}

	status := a.classify(len(opp.Attachments), len(docs), len(issues))
	res := a.result(opp.ID, docs, issues, status)
	return docs, texts, res, nil
}

// ingestOne handles a single attachment. It returns the recorded doc (nil if the
// attachment could not be fetched), the extracted text (empty when none), a
// non-empty issue string when something needs human attention, and a Go error
// only for an unrecoverable ObjectStore failure.
func (a *Agent) ingestOne(ctx context.Context, noticeID string, idx int, url string) (doc *opportunity.SolicitationDoc, text, issue string, err error) {
	raw, contentType, filename, err := a.fetcher.Fetch(ctx, url)
	if err != nil {
		return nil, "", fmt.Sprintf("[fetch] %s: %v", url, err), nil
	}

	filename = safeFilename(filename, idx)
	sum := sha256Hex(raw)
	rawObject := path.Join(noticeID, "raw", filename)
	textObject := path.Join(noticeID, "text", filename+".txt")

	rec := opportunity.SolicitationDoc{
		Filename:    filename,
		SourceURL:   url,
		ContentType: contentType,
		SHA256:      sum,
		Bytes:       int64(len(raw)),
		IngestedAt:  time.Now().UTC(),
	}

	// Idempotency: if the raw object already exists with the same checksum, the
	// document is unchanged — reuse the stored objects and load the already
	// extracted text back from GCS rather than re-running (metered) extraction.
	existing, ok, statErr := a.store.Stat(ctx, rawObject)
	if statErr != nil {
		return nil, "", "", fmt.Errorf("ingest: stat %s: %w", rawObject, statErr)
	}
	if ok && existing == sum {
		rec.RawObject = a.store.URI(rawObject)
		stored, hasText, rerr := a.readStoredText(ctx, textObject)
		if rerr != nil {
			return nil, "", "", rerr
		}
		if hasText {
			rec.TextObject = a.store.URI(textObject)
			return &rec, stored, "", nil
		}
		// Unchanged, but it had no extractable text before — still flag it so a
		// human sees the gap on every run, not just the first.
		return &rec, "", fmt.Sprintf("[no_text] %s: no extractable text (scanned or empty)", filename), nil
	}

	rawURI, err := a.store.Put(ctx, rawObject, raw, contentType, sum)
	if err != nil {
		return nil, "", "", fmt.Errorf("ingest: put raw %s: %w", rawObject, err)
	}
	rec.RawObject = rawURI

	// Extract text. Empty (not error) means no embedded text — keep the raw file
	// for re-download, leave TextObject empty, and flag for a human. Never invent.
	extracted, err := a.extract.ExtractText(ctx, raw, contentType)
	if err != nil {
		return &rec, "", fmt.Sprintf("[extract] %s: %v", filename, err), nil
	}
	if strings.TrimSpace(extracted) == "" {
		return &rec, "", fmt.Sprintf("[no_text] %s: no extractable text (scanned or empty)", filename), nil
	}

	textURI, err := a.store.Put(ctx, textObject, []byte(extracted), textContentType, sha256Hex([]byte(extracted)))
	if err != nil {
		return nil, "", "", fmt.Errorf("ingest: put text %s: %w", textObject, err)
	}
	rec.TextObject = textURI
	return &rec, extracted, "", nil
}

// readStoredText loads the extracted text previously stored at textObject, if it
// exists. It returns ("", false, nil) when no text object is present (e.g. the
// document was unextractable on the first ingest).
func (a *Agent) readStoredText(ctx context.Context, textObject string) (text string, found bool, err error) {
	_, ok, err := a.store.Stat(ctx, textObject)
	if err != nil {
		return "", false, fmt.Errorf("ingest: stat %s: %w", textObject, err)
	}
	if !ok {
		return "", false, nil
	}
	b, err := a.store.Read(ctx, textObject)
	if err != nil {
		return "", false, fmt.Errorf("ingest: read %s: %w", textObject, err)
	}
	return string(b), true, nil
}

// classify maps the per-run counts onto a terminal status.
func (a *Agent) classify(attachments, docs, issues int) agent.Status {
	switch {
	case attachments > 0 && docs == 0:
		// Attachments existed but none could be fetched.
		return agent.StatusFailed
	case issues > 0:
		// Partial success or unextractable documents — a person should look.
		return agent.StatusNeedsHuman
	default:
		return agent.StatusSuccess
	}
}

// result builds the agent.Result for a finished run.
func (a *Agent) result(noticeID string, docs []opportunity.SolicitationDoc, issues []string, status agent.Status) *agent.Result {
	res := &agent.Result{
		AgentName:   agentName,
		Status:      status,
		NoticeID:    noticeID,
		Flags:       buildFlags(len(docs), issues),
		CompletedAt: time.Now().UTC(),
	}
	switch status {
	case agent.StatusFailed:
		res.Summary = fmt.Sprintf("ingest failed: 0 of %d attachment(s) could be fetched", len(issues))
		res.Error = strings.Join(issues, "; ")
	case agent.StatusNeedsHuman:
		res.Summary = fmt.Sprintf("ingested %d document(s) with %d issue(s) needing review", len(docs), len(issues))
	default:
		res.Summary = fmt.Sprintf("ingested %d document(s)", len(docs))
	}
	return res
}

// buildFlags records the document count and any per-attachment issues, mirroring
// the flag convention used by the Final Review agent.
func buildFlags(docCount int, issues []string) map[string]string {
	flags := map[string]string{
		"documents_ingested": strconv.Itoa(docCount),
		"issues_found":       strconv.Itoa(len(issues)),
	}
	for i, detail := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = detail
	}
	return flags
}

// safeFilename returns a clean, path-separator-free filename, falling back to a
// deterministic name derived from the attachment index when none is available.
func safeFilename(name string, idx int) string {
	name = strings.TrimSpace(name)
	// Drop any directory components a Content-Disposition or URL may have carried.
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || name == "." || name == "/" {
		return fmt.Sprintf("attachment-%d", idx)
	}
	return name
}

// sha256Hex returns the lowercase hex SHA-256 of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
