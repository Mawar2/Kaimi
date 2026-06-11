package ingest

import (
	"context"
	"fmt"
	"io"
	"strings"

	documentai "cloud.google.com/go/documentai/apiv1"
	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// DocumentAIExtractor extracts text from a raw document using a Google Document
// AI processor (an OCR processor in production). It reads PDFs and images —
// including scanned documents — which pure-Go PDF parsers cannot, and is the
// accuracy-first primary extractor for the ingest stage.
//
// It uses the fast synchronous ProcessDocument path by default, and transparently
// falls back to asynchronous BatchProcessDocuments for PDFs that exceed the sync
// page limit (see issue #194) — the large multi-page solicitation packages that
// carry the Section L/M and SOW text. Batch needs a scratch GCS bucket for its
// in/out documents; when no bucket is configured the extractor is sync-only.
type DocumentAIExtractor struct {
	client        *documentai.DocumentProcessorClient
	storageClient *storage.Client // nil when batch fallback is not configured
	processorName string
	batchBucket   string // scratch bucket for batch in/out; "" disables batch
}

// NewDocumentAIExtractor builds an extractor backed by the Document AI processor
// identified by projectID, location, and processorID.
//
// batchBucket is a GCS bucket (no gs:// prefix) used as scratch space for the
// async batch fallback on large PDFs; pass "" to disable batch (sync only). In
// production it is the solicitations bucket (GCS_SOLICITATIONS_BUCKET).
//
// location is the Document AI multi-region ("us" or "eu") — note this is NOT the
// us-east4 region the rest of Kaimi runs in; Document AI is only offered in the
// multi-regions. The caller must call the returned closer when finished to
// release the gRPC (and, when batch is enabled, the storage) client.
func NewDocumentAIExtractor(ctx context.Context, projectID, location, processorID, batchBucket string, opts ...option.ClientOption) (*DocumentAIExtractor, func() error, error) {
	if projectID == "" || location == "" || processorID == "" {
		return nil, nil, fmt.Errorf("documentai: projectID, location, and processorID are required")
	}
	// Document AI is regionalised: the client must target the location endpoint.
	// User ADC (gcloud application-default login) bills to a "quota project": the
	// resource-scoped sync ProcessDocument call infers it from the processor, but
	// the batch long-running-operation polling goes through the generic operations
	// API, which requires an explicit quota project or fails PermissionDenied
	// (SERVICE_DISABLED on the default project). Set it on the DA client only —
	// the storage client rejects WithQuotaProject (incompatible with its HTTP
	// client) and bills GCS to the bucket's own project anyway.
	endpoint := fmt.Sprintf("%s-documentai.googleapis.com:443", location)
	daOpts := append(append([]option.ClientOption{}, opts...), option.WithEndpoint(endpoint), option.WithQuotaProject(projectID))
	client, err := documentai.NewDocumentProcessorClient(ctx, daOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("documentai: new client: %w", err)
	}

	ex := &DocumentAIExtractor{
		client:        client,
		processorName: fmt.Sprintf("projects/%s/locations/%s/processors/%s", projectID, location, processorID),
		batchBucket:   batchBucket,
	}

	// The batch fallback reads/writes scratch objects in GCS, so it needs a
	// storage client. Storage is global, not regionalised, so it uses default opts.
	if batchBucket != "" {
		sc, err := storage.NewClient(ctx, opts...)
		if err != nil {
			_ = client.Close()
			return nil, nil, fmt.Errorf("documentai: storage client for batch: %w", err)
		}
		ex.storageClient = sc
	}

	closer := func() error {
		cErr := client.Close()
		if ex.storageClient != nil {
			if sErr := ex.storageClient.Close(); sErr != nil && cErr == nil {
				return sErr
			}
		}
		return cErr
	}
	return ex, closer, nil
}

// ExtractText sends raw to the Document AI processor and returns the recognized
// text. An empty string with a nil error means the processor found no text (e.g.
// a blank scan) — the caller treats that as a gap, never as failure to fabricate.
//
// For PDFs that exceed the synchronous page limit, it falls back to async batch
// processing (issue #194) so the largest solicitation packages still yield text.
func (e *DocumentAIExtractor) ExtractText(ctx context.Context, raw []byte, contentType string) (string, error) {
	mime := normalizeMIME(contentType)
	text, err := e.processSync(ctx, raw, mime)
	if err == nil {
		return text, nil
	}
	// Only the synchronous path has a tight page limit. Large PDFs trip it; fall
	// back to batch when it is configured. Non-PDF failures (e.g. an unsupported
	// format) would fail batch too, so don't waste a batch run on them.
	if e.batchBucket != "" && mime == "application/pdf" && isPageLimitErr(err) {
		return e.processBatch(ctx, raw, mime)
	}
	return "", err
}

// processSync runs the inline (online) ProcessDocument request.
func (e *DocumentAIExtractor) processSync(ctx context.Context, raw []byte, mime string) (string, error) {
	req := &documentaipb.ProcessRequest{
		Name: e.processorName,
		Source: &documentaipb.ProcessRequest_RawDocument{
			RawDocument: &documentaipb.RawDocument{Content: raw, MimeType: mime},
		},
	}
	resp, err := e.client.ProcessDocument(ctx, req)
	if err != nil {
		return "", fmt.Errorf("documentai: process document: %w", err)
	}
	return resp.GetDocument().GetText(), nil
}

// processBatch uploads raw to scratch GCS, runs the async BatchProcessDocuments
// operation (no practical page limit), reads the JSON output shards Document AI
// writes back to GCS, concatenates their text, and cleans up the scratch objects.
func (e *DocumentAIExtractor) processBatch(ctx context.Context, raw []byte, mime string) (string, error) {
	if e.storageClient == nil {
		return "", fmt.Errorf("documentai: batch fallback not configured (no storage client)")
	}
	id := sha256Hex(raw)[:16]
	inObject := "_batch/in/" + id
	outPrefix := "_batch/out/" + id + "/"
	bkt := e.storageClient.Bucket(e.batchBucket)
	defer e.cleanupBatch(ctx, bkt, inObject, outPrefix)

	// Upload the input document.
	w := bkt.Object(inObject).NewWriter(ctx)
	w.ContentType = mime
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("documentai batch: upload input: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("documentai batch: close input: %w", err)
	}

	req := &documentaipb.BatchProcessRequest{
		Name: e.processorName,
		InputDocuments: &documentaipb.BatchDocumentsInputConfig{
			Source: &documentaipb.BatchDocumentsInputConfig_GcsDocuments{
				GcsDocuments: &documentaipb.GcsDocuments{
					Documents: []*documentaipb.GcsDocument{
						{GcsUri: fmt.Sprintf("gs://%s/%s", e.batchBucket, inObject), MimeType: mime},
					},
				},
			},
		},
		DocumentOutputConfig: &documentaipb.DocumentOutputConfig{
			Destination: &documentaipb.DocumentOutputConfig_GcsOutputConfig_{
				GcsOutputConfig: &documentaipb.DocumentOutputConfig_GcsOutputConfig{
					GcsUri: fmt.Sprintf("gs://%s/%s", e.batchBucket, outPrefix),
				},
			},
		},
	}

	op, err := e.client.BatchProcessDocuments(ctx, req)
	if err != nil {
		return "", fmt.Errorf("documentai batch: start: %w", err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return "", fmt.Errorf("documentai batch: wait: %w", err)
	}

	// Document AI writes one or more JSON shards under the output prefix; each
	// decodes to a documentaipb.Document. Concatenate their text in listing order.
	var sb strings.Builder
	it := bkt.Objects(ctx, &storage.Query{Prefix: outPrefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", fmt.Errorf("documentai batch: list output: %w", err)
		}
		if !strings.HasSuffix(attrs.Name, ".json") {
			continue
		}
		text, err := e.readShardText(ctx, bkt, attrs.Name)
		if err != nil {
			return "", err
		}
		sb.WriteString(text)
	}
	return sb.String(), nil
}

// readShardText reads one Document AI JSON output shard and returns its text.
func (e *DocumentAIExtractor) readShardText(ctx context.Context, bkt *storage.BucketHandle, object string) (string, error) {
	r, err := bkt.Object(object).NewReader(ctx)
	if err != nil {
		return "", fmt.Errorf("documentai batch: open %s: %w", object, err)
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("documentai batch: read %s: %w", object, err)
	}
	var doc documentaipb.Document
	if err := protojson.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("documentai batch: parse %s: %w", object, err)
	}
	return doc.GetText(), nil
}

// cleanupBatch best-effort deletes the scratch input and output objects so the
// solicitations bucket does not accumulate batch intermediates. Errors are
// ignored: the run already succeeded, and stale scratch is harmless.
func (e *DocumentAIExtractor) cleanupBatch(ctx context.Context, bkt *storage.BucketHandle, inObject, outPrefix string) {
	_ = bkt.Object(inObject).Delete(ctx)
	it := bkt.Objects(ctx, &storage.Query{Prefix: outPrefix})
	for {
		attrs, err := it.Next()
		if err != nil {
			return
		}
		_ = bkt.Object(attrs.Name).Delete(ctx)
	}
}

// isPageLimitErr reports whether a synchronous ProcessDocument error is the
// page/size limit that batch processing can get past. Document AI returns
// InvalidArgument for an oversize document; we also match the explicit page
// message defensively across API versions.
func isPageLimitErr(err error) bool {
	if err == nil {
		return false
	}
	if s, ok := status.FromError(err); ok && s.Code() == codes.InvalidArgument {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "page") && (strings.Contains(msg, "limit") || strings.Contains(msg, "exceed"))
}

// normalizeMIME strips any parameters (e.g. "; charset=…") from a content type
// and defaults a missing one to application/pdf, the dominant attachment format.
// Document AI rejects content types it does not recognise, so this keeps the
// request well-formed.
func normalizeMIME(contentType string) string {
	ct := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	if ct == "" {
		return "application/pdf"
	}
	return strings.ToLower(ct)
}
