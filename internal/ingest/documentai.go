package ingest

import (
	"context"
	"fmt"
	"strings"

	documentai "cloud.google.com/go/documentai/apiv1"
	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"google.golang.org/api/option"
)

// DocumentAIExtractor extracts text from a raw document using a Google Document
// AI processor (an OCR processor in production). It reads PDFs and images —
// including scanned documents — which pure-Go PDF parsers cannot, and is the
// accuracy-first primary extractor for the ingest stage.
type DocumentAIExtractor struct {
	client        *documentai.DocumentProcessorClient
	processorName string
}

// NewDocumentAIExtractor builds an extractor backed by the Document AI processor
// identified by projectID, location, and processorID.
//
// location is the Document AI multi-region ("us" or "eu") — note this is NOT the
// us-east4 region the rest of Kaimi runs in; Document AI is only offered in the
// multi-regions. The caller must call the returned closer when finished to
// release the gRPC client.
func NewDocumentAIExtractor(ctx context.Context, projectID, location, processorID string, opts ...option.ClientOption) (*DocumentAIExtractor, func() error, error) {
	if projectID == "" || location == "" || processorID == "" {
		return nil, nil, fmt.Errorf("documentai: projectID, location, and processorID are required")
	}
	// Document AI is regionalised: the client must target the location endpoint.
	endpoint := fmt.Sprintf("%s-documentai.googleapis.com:443", location)
	opts = append(opts, option.WithEndpoint(endpoint))

	client, err := documentai.NewDocumentProcessorClient(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("documentai: new client: %w", err)
	}
	name := fmt.Sprintf("projects/%s/locations/%s/processors/%s", projectID, location, processorID)
	return &DocumentAIExtractor{client: client, processorName: name}, client.Close, nil
}

// ExtractText sends raw to the Document AI processor and returns the recognized
// text. An empty string with a nil error means the processor found no text (e.g.
// a blank scan) — the caller treats that as a gap, never as failure to fabricate.
func (e *DocumentAIExtractor) ExtractText(ctx context.Context, raw []byte, contentType string) (string, error) {
	req := &documentaipb.ProcessRequest{
		Name: e.processorName,
		Source: &documentaipb.ProcessRequest_RawDocument{
			RawDocument: &documentaipb.RawDocument{
				Content:  raw,
				MimeType: normalizeMIME(contentType),
			},
		},
	}
	resp, err := e.client.ProcessDocument(ctx, req)
	if err != nil {
		return "", fmt.Errorf("documentai: process document: %w", err)
	}
	return resp.GetDocument().GetText(), nil
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
