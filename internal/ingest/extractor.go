package ingest

import (
	"context"
	"strings"
)

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

// ExtractText routes by content type: the OOXML office formats (DOCX/PPTX/XLSX)
// go to dependency-free stdlib extractors; everything else (PDFs, images, scans)
// goes to the primary extractor (Document AI). When the content type is generic
// (octet-stream/zip), it sniffs the bytes so an office file served without its
// real MIME type still takes the stdlib path instead of being misrouted to OCR
// (issue #194) — and so the OCR engine is never handed a format it cannot read.
func (r *RoutingExtractor) ExtractText(ctx context.Context, raw []byte, contentType string) (string, error) {
	generic := isGenericType(contentType)
	switch {
	case isDOCX(contentType) || (generic && looksLikeDOCX(raw)):
		return extractDOCX(raw)
	case isPPTX(contentType) || (generic && looksLikePPTX(raw)):
		return extractPPTX(raw)
	case isXLSX(contentType) || (generic && looksLikeXLSX(raw)):
		return extractXLSX(raw)
	default:
		return r.primary.ExtractText(ctx, raw, contentType)
	}
}

// isGenericType reports whether a content type is too generic to trust for
// routing (empty, octet-stream, or a bare zip) — the cases where byte sniffing
// should decide whether a document is really a .docx.
func isGenericType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	return ct == "" || ct == "application/octet-stream" || ct == "application/zip"
}
