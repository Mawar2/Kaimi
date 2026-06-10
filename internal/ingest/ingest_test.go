package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// --- Test doubles -----------------------------------------------------------

// fakeFetcher returns canned responses keyed by URL, or an error if missing.
type fakeFetcher struct {
	byURL map[string]fakeDoc
	calls []string
}

type fakeDoc struct {
	data        []byte
	contentType string
	filename    string
	err         error
}

func (f *fakeFetcher) Fetch(_ context.Context, url string) (data []byte, contentType, filename string, err error) {
	f.calls = append(f.calls, url)
	d, ok := f.byURL[url]
	if !ok {
		return nil, "", "", fmt.Errorf("no canned response for %s", url)
	}
	return d.data, d.contentType, d.filename, d.err
}

// memStore is an in-memory ObjectStore recording puts, with optional pre-seeded
// SHA values to exercise the dedup path.
type memStore struct {
	bucket  string
	objects map[string][]byte
	shas    map[string]string // object -> stored sha
	puts    []string          // object names written, in order
}

func newMemStore() *memStore {
	return &memStore{bucket: "test-bucket", objects: map[string][]byte{}, shas: map[string]string{}}
}

func (m *memStore) Stat(_ context.Context, object string) (sha string, exists bool, err error) {
	sha, ok := m.shas[object]
	return sha, ok, nil
}

func (m *memStore) Put(_ context.Context, object string, data []byte, _, sha string) (string, error) {
	m.objects[object] = data
	m.shas[object] = sha
	m.puts = append(m.puts, object)
	return m.URI(object), nil
}

func (m *memStore) URI(object string) string {
	return "gs://" + m.bucket + "/" + object
}

// fakeExtractor returns canned text per content type, or an error/empty string.
type fakeExtractor struct {
	text string
	err  error
}

func (e *fakeExtractor) ExtractText(_ context.Context, _ []byte, _ string) (string, error) {
	return e.text, e.err
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// --- Tests ------------------------------------------------------------------

func TestIngest_HappyPath_FetchesUploadsExtracts(t *testing.T) {
	raw := []byte("%PDF-1.7 solicitation body")
	fetch := &fakeFetcher{byURL: map[string]fakeDoc{
		"https://sam.gov/n/1/rfp.pdf": {data: raw, contentType: "application/pdf", filename: "rfp.pdf"},
	}}
	store := newMemStore()
	extract := &fakeExtractor{text: "Section L instructions..."}
	a := New(fetch, store, extract)

	opp := &opportunity.Opportunity{ID: "N-1", Attachments: []string{"https://sam.gov/n/1/rfp.pdf"}}

	docs, res, err := a.Ingest(context.Background(), opp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != agent.StatusSuccess {
		t.Fatalf("status = %q, want success; flags=%v", res.Status, res.Flags)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	d := docs[0]
	if d.Filename != "rfp.pdf" || d.SourceURL != "https://sam.gov/n/1/rfp.pdf" {
		t.Errorf("doc identity wrong: %+v", d)
	}
	if d.RawObject != "gs://test-bucket/N-1/raw/rfp.pdf" {
		t.Errorf("RawObject = %q", d.RawObject)
	}
	if d.TextObject != "gs://test-bucket/N-1/text/rfp.pdf.txt" {
		t.Errorf("TextObject = %q", d.TextObject)
	}
	if d.SHA256 != sha256hex(raw) {
		t.Errorf("SHA256 = %q, want %q", d.SHA256, sha256hex(raw))
	}
	if d.Bytes != int64(len(raw)) {
		t.Errorf("Bytes = %d, want %d", d.Bytes, len(raw))
	}
	if d.IngestedAt.IsZero() {
		t.Error("IngestedAt is zero")
	}
	// Both raw and text objects were written.
	if got := strings.Join(store.puts, ","); !strings.Contains(got, "N-1/raw/rfp.pdf") || !strings.Contains(got, "N-1/text/rfp.pdf.txt") {
		t.Errorf("puts = %v, want raw+text", store.puts)
	}
	if string(store.objects["N-1/text/rfp.pdf.txt"]) != "Section L instructions..." {
		t.Errorf("stored text = %q", store.objects["N-1/text/rfp.pdf.txt"])
	}
}

func TestIngest_NoAttachments_SucceedsWithNoDocs(t *testing.T) {
	a := New(&fakeFetcher{}, newMemStore(), &fakeExtractor{})
	docs, res, err := a.Ingest(context.Background(), &opportunity.Opportunity{ID: "N-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != agent.StatusSuccess {
		t.Errorf("status = %q, want success", res.Status)
	}
	if len(docs) != 0 {
		t.Errorf("got %d docs, want 0", len(docs))
	}
}

func TestIngest_Idempotent_SkipsReuploadWhenShaMatches(t *testing.T) {
	raw := []byte("unchanged bytes")
	fetch := &fakeFetcher{byURL: map[string]fakeDoc{
		"u": {data: raw, contentType: "application/pdf", filename: "a.pdf"},
	}}
	store := newMemStore()
	// Pre-seed the raw object with the SAME sha => dedup should skip re-upload.
	store.shas["N-3/raw/a.pdf"] = sha256hex(raw)
	a := New(fetch, store, &fakeExtractor{text: "x"})

	docs, res, err := a.Ingest(context.Background(), &opportunity.Opportunity{ID: "N-3", Attachments: []string{"u"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != agent.StatusSuccess {
		t.Fatalf("status = %q, want success", res.Status)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	// Nothing should have been written because the raw sha already matched.
	if len(store.puts) != 0 {
		t.Errorf("expected no puts on dedup, got %v", store.puts)
	}
	if docs[0].RawObject != "gs://test-bucket/N-3/raw/a.pdf" {
		t.Errorf("RawObject = %q", docs[0].RawObject)
	}
}

func TestIngest_FetchFailure_RoutesToNeedsHuman(t *testing.T) {
	// One good, one missing (fetch error) => partial => needs_human.
	good := []byte("good")
	fetch := &fakeFetcher{byURL: map[string]fakeDoc{
		"ok": {data: good, contentType: "application/pdf", filename: "ok.pdf"},
	}}
	a := New(fetch, newMemStore(), &fakeExtractor{text: "t"})
	docs, res, err := a.Ingest(context.Background(), &opportunity.Opportunity{
		ID: "N-4", Attachments: []string{"ok", "missing"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != agent.StatusNeedsHuman {
		t.Fatalf("status = %q, want needs_human; flags=%v", res.Status, res.Flags)
	}
	if len(docs) != 1 {
		t.Errorf("got %d docs, want 1 (the good one)", len(docs))
	}
	if res.Flags["issues_found"] != "1" {
		t.Errorf("issues_found = %q, want 1", res.Flags["issues_found"])
	}
}

func TestIngest_AllFetchesFail_RoutesToFailed(t *testing.T) {
	a := New(&fakeFetcher{}, newMemStore(), &fakeExtractor{})
	docs, res, err := a.Ingest(context.Background(), &opportunity.Opportunity{
		ID: "N-5", Attachments: []string{"x", "y"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != agent.StatusFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if len(docs) != 0 {
		t.Errorf("got %d docs, want 0", len(docs))
	}
}

func TestIngest_EmptyExtraction_RecordsDocButFlags(t *testing.T) {
	// A scanned PDF the extractor returns no text for: raw is still saved (for
	// re-download), but the doc is flagged and the run needs a human.
	raw := []byte("scanned image bytes")
	fetch := &fakeFetcher{byURL: map[string]fakeDoc{
		"s": {data: raw, contentType: "application/pdf", filename: "scan.pdf"},
	}}
	store := newMemStore()
	a := New(fetch, store, &fakeExtractor{text: "   "}) // whitespace only
	docs, res, err := a.Ingest(context.Background(), &opportunity.Opportunity{ID: "N-6", Attachments: []string{"s"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != agent.StatusNeedsHuman {
		t.Fatalf("status = %q, want needs_human; flags=%v", res.Status, res.Flags)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	// Raw saved for re-download; no text object written.
	if _, ok := store.objects["N-6/raw/scan.pdf"]; !ok {
		t.Error("raw object should be saved even when text extraction is empty")
	}
	if docs[0].TextObject != "" {
		t.Errorf("TextObject should be empty for an unextractable doc, got %q", docs[0].TextObject)
	}
}

func TestIngest_NilOpportunity_Errors(t *testing.T) {
	a := New(&fakeFetcher{}, newMemStore(), &fakeExtractor{})
	if _, _, err := a.Ingest(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil opportunity")
	}
}
