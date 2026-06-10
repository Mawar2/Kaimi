package ingest

import "context"

// RoutingExtractor dispatches each document to the right text extractor: DOCX is
// handled locally with the standard library (no dependency, no network), while
// everything else — PDFs, images, scanned documents — goes to the primary
// extractor (Document AI in production).
//
// This keeps the cheap, deterministic DOCX path off the metered Document AI API
// while still routing the formats that genuinely need OCR/layout analysis to it.
type RoutingExtractor struct {
	primary Extractor
}

// NewRoutingExtractor builds a RoutingExtractor whose non-DOCX documents are
// handled by primary (the Document AI extractor in production).
func NewRoutingExtractor(primary Extractor) *RoutingExtractor {
	return &RoutingExtractor{primary: primary}
}

// ExtractText routes by content type: DOCX to the stdlib extractor, all else to
// the primary extractor.
func (r *RoutingExtractor) ExtractText(ctx context.Context, raw []byte, contentType string) (string, error) {
	if isDOCX(contentType) {
		return extractDOCX(raw)
	}
	return r.primary.ExtractText(ctx, raw, contentType)
}
