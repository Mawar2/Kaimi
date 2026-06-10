package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
)

// buildDOCX constructs a minimal but valid .docx (a ZIP holding word/document.xml)
// from the given WordprocessingML body, so the DOCX extractor can be unit-tested
// without a binary fixture or any network.
func buildDOCX(t *testing.T, bodyXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` + bodyXML + `</w:body></w:document>`

	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create document.xml: %v", err)
	}
	if _, err := w.Write([]byte(doc)); err != nil {
		t.Fatalf("write document.xml: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestExtractDOCX_ParagraphsAndRuns(t *testing.T) {
	body := `<w:p><w:r><w:t>Hello</w:t></w:r><w:r><w:t> world</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Second paragraph</w:t></w:r></w:p>`
	raw := buildDOCX(t, body)

	got, err := extractDOCX(raw)
	if err != nil {
		t.Fatalf("extractDOCX: %v", err)
	}
	want := "Hello world\nSecond paragraph"
	if got != want {
		t.Errorf("extractDOCX = %q, want %q", got, want)
	}
}

func TestExtractDOCX_TabsAndBreaks(t *testing.T) {
	body := `<w:p><w:r><w:t>A</w:t><w:tab/><w:t>B</w:t><w:br/><w:t>C</w:t></w:r></w:p>`
	raw := buildDOCX(t, body)

	got, err := extractDOCX(raw)
	if err != nil {
		t.Fatalf("extractDOCX: %v", err)
	}
	if want := "A\tB\nC"; got != want {
		t.Errorf("extractDOCX = %q, want %q", got, want)
	}
}

func TestExtractDOCX_NotAWordDoc(t *testing.T) {
	// A valid zip but without word/document.xml must error, not panic.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("other.txt")
	_, _ = w.Write([]byte("nope"))
	_ = zw.Close()

	if _, err := extractDOCX(buf.Bytes()); err == nil {
		t.Fatal("expected error for a zip without word/document.xml")
	}
}

func TestExtractDOCX_NotAZip(t *testing.T) {
	if _, err := extractDOCX([]byte("this is not a zip")); err == nil {
		t.Fatal("expected error for non-zip input")
	}
}

func TestRoutingExtractor_DOCXUsesStdlib_NotPrimary(t *testing.T) {
	body := `<w:p><w:r><w:t>routed locally</w:t></w:r></w:p>`
	raw := buildDOCX(t, body)

	// A primary that would fail the test if it were ever called for DOCX.
	primary := &fakeExtractor{text: "PRIMARY SHOULD NOT RUN"}
	r := NewRoutingExtractor(primary)

	got, err := r.ExtractText(context.TODO(), raw, docxContentType)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(got, "routed locally") {
		t.Errorf("DOCX not routed to stdlib extractor: got %q", got)
	}
}

func TestRoutingExtractor_NonDOCXUsesPrimary(t *testing.T) {
	primary := &fakeExtractor{text: "from document ai"}
	r := NewRoutingExtractor(primary)

	got, err := r.ExtractText(context.TODO(), []byte("%PDF"), "application/pdf")
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if got != "from document ai" {
		t.Errorf("non-DOCX should use primary extractor, got %q", got)
	}
}
