package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
)

// makeDOCX builds a minimal valid .docx (a zip whose word/document.xml carries
// one paragraph) for routing tests.
func makeDOCX(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	doc := `<?xml version="1.0"?><w:document xmlns:w="urn:x"><w:body><w:p><w:r><w:t>` +
		body + `</w:t></w:r></w:p></w:body></w:document>`
	if _, err := w.Write([]byte(doc)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// spyExtractor is the OCR primary; it records whether it was called so the test
// can assert a DOCX never reaches it.
type spyExtractor struct{ called bool }

func (s *spyExtractor) ExtractText(context.Context, []byte, string) (string, error) {
	s.called = true
	return "OCR-TEXT", nil
}

// TestRoutingExtractor_OctetStreamDOCX_UsesStdlib covers issue #194 bug 2: a
// .docx served as application/octet-stream must be sniffed and handled by the
// stdlib DOCX extractor, never misrouted to the (metered, format-limited) OCR.
func TestRoutingExtractor_OctetStreamDOCX_UsesStdlib(t *testing.T) {
	raw := makeDOCX(t, "Section L instructions to offerors")
	spy := &spyExtractor{}

	got, err := NewRoutingExtractor(spy).ExtractText(context.Background(), raw, "application/octet-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spy.called {
		t.Error("OCR primary must NOT run for an octet-stream .docx (#194)")
	}
	if !strings.Contains(got, "Section L instructions to offerors") {
		t.Errorf("got %q, want the docx body text", got)
	}
}

// TestRoutingExtractor_NonDOCXOctetStream_UsesPrimary verifies the sniff does not
// over-trigger: non-docx octet-stream bytes still fall through to the OCR primary.
func TestRoutingExtractor_NonDOCXOctetStream_UsesPrimary(t *testing.T) {
	spy := &spyExtractor{}

	got, err := NewRoutingExtractor(spy).ExtractText(context.Background(), []byte("%PDF-1.7 not a zip"), "application/octet-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spy.called {
		t.Error("non-docx octet-stream must fall through to the primary extractor")
	}
	if got != "OCR-TEXT" {
		t.Errorf("got %q, want primary extractor output", got)
	}
}

// TestRoutingExtractor_TypedDOCX_UsesStdlib confirms the original typed path still
// routes DOCX locally (and the spy primary stays untouched).
func TestRoutingExtractor_TypedDOCX_UsesStdlib(t *testing.T) {
	raw := makeDOCX(t, "typed docx body")
	spy := &spyExtractor{}

	got, err := NewRoutingExtractor(spy).ExtractText(context.Background(), raw, docxContentType)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spy.called {
		t.Error("OCR primary must not run for a typed .docx")
	}
	if !strings.Contains(got, "typed docx body") {
		t.Errorf("got %q, want docx text", got)
	}
}
