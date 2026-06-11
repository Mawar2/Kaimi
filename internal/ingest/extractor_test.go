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

// makePPTX builds a minimal .pptx: a zip with ppt/presentation.xml (for sniffing)
// and two slides whose DrawingML carries one paragraph each.
func makePPTX(t *testing.T, slide1, slide2 string) []byte {
	t.Helper()
	slide := func(body string) string {
		return `<?xml version="1.0"?><p:sld xmlns:p="urn:p" xmlns:a="urn:a"><p:cSld><p:spTree>` +
			`<a:p><a:r><a:t>` + body + `</a:t></a:r></a:p></p:spTree></p:cSld></p:sld>`
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"ppt/presentation.xml":  `<?xml version="1.0"?><p:presentation xmlns:p="urn:p"/>`,
		"ppt/slides/slide1.xml": slide(slide1),
		"ppt/slides/slide2.xml": slide(slide2),
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// makeXLSX builds a minimal .xlsx: a zip with xl/workbook.xml (for sniffing) and
// xl/sharedStrings.xml holding the given strings as <si> items.
func makeXLSX(t *testing.T, strs ...string) []byte {
	t.Helper()
	var ss strings.Builder
	ss.WriteString(`<?xml version="1.0"?><sst xmlns="urn:x">`)
	for _, s := range strs {
		ss.WriteString(`<si><t>` + s + `</t></si>`)
	}
	ss.WriteString(`</sst>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"xl/workbook.xml":      `<?xml version="1.0"?><workbook xmlns="urn:w"/>`,
		"xl/sharedStrings.xml": ss.String(),
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestRoutingExtractor_PPTX covers the new stdlib PPTX path (#194 follow-up):
// typed and octet-stream .pptx both extract slide text via stdlib, never OCR, in
// slide order.
func TestRoutingExtractor_PPTX(t *testing.T) {
	raw := makePPTX(t, "Industry day overview", "Technical requirements")
	for _, ct := range []string{pptxContentType, "application/octet-stream"} {
		spy := &spyExtractor{}
		got, err := NewRoutingExtractor(spy).ExtractText(context.Background(), raw, ct)
		if err != nil {
			t.Fatalf("ct=%s: unexpected error: %v", ct, err)
		}
		if spy.called {
			t.Errorf("ct=%s: OCR primary must not run for a .pptx", ct)
		}
		if !strings.Contains(got, "Industry day overview") || !strings.Contains(got, "Technical requirements") {
			t.Errorf("ct=%s: got %q, want both slides' text", ct, got)
		}
	}
}

// TestRoutingExtractor_XLSX covers the new stdlib XLSX path: typed and
// octet-stream .xlsx both extract shared-string text via stdlib, never OCR.
func TestRoutingExtractor_XLSX(t *testing.T) {
	raw := makeXLSX(t, "Question: page limit?", "Answer: 30 pages")
	for _, ct := range []string{xlsxContentType, "application/octet-stream"} {
		spy := &spyExtractor{}
		got, err := NewRoutingExtractor(spy).ExtractText(context.Background(), raw, ct)
		if err != nil {
			t.Fatalf("ct=%s: unexpected error: %v", ct, err)
		}
		if spy.called {
			t.Errorf("ct=%s: OCR primary must not run for a .xlsx", ct)
		}
		if !strings.Contains(got, "page limit?") || !strings.Contains(got, "30 pages") {
			t.Errorf("ct=%s: got %q, want shared-string text", ct, got)
		}
	}
}
