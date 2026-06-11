package ingest

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// pptxContentType is the MIME type for .pptx (OOXML PresentationML) files.
const pptxContentType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

// isPPTX reports whether contentType denotes a .pptx presentation.
func isPPTX(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "presentationml.presentation")
}

// looksLikePPTX reports whether raw is structurally a .pptx (a ZIP containing
// ppt/presentation.xml), for when the served content type is generic.
func looksLikePPTX(raw []byte) bool {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return false
	}
	for _, f := range zr.File {
		if f.Name == "ppt/presentation.xml" {
			return true
		}
	}
	return false
}

// extractPPTX returns the slide text of a .pptx using only the standard library.
// A .pptx is a ZIP whose ppt/slides/slideN.xml files hold each slide's text as
// DrawingML. The text runs (<a:t>), paragraph breaks (<a:p>) and line breaks
// (<a:br>) share local element names with WordprocessingML, so parseDocumentXML
// (from docx.go) handles them. Slides are emitted in numeric order. No
// third-party dependency is needed.
func extractPPTX(raw []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("pptx: open zip: %w", err)
	}

	type slide struct {
		n int
		f *zip.File
	}
	var slides []slide
	for _, f := range zr.File {
		// Match ppt/slides/slideN.xml only — not slideLayouts, slideMasters, or
		// the _rels sidecars (those don't have the ppt/slides/slide prefix).
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		numStr := strings.TrimSuffix(strings.TrimPrefix(f.Name, "ppt/slides/slide"), ".xml")
		n, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		slides = append(slides, slide{n: n, f: f})
	}
	if len(slides) == 0 {
		return "", nil // no slides with text; a gap, never an error
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].n < slides[j].n })

	var sb strings.Builder
	for _, s := range slides {
		rc, err := s.f.Open()
		if err != nil {
			return "", fmt.Errorf("pptx: open %s: %w", s.f.Name, err)
		}
		text, perr := parseDocumentXML(rc)
		_ = rc.Close()
		if perr != nil {
			return "", fmt.Errorf("pptx: parse %s: %w", s.f.Name, perr)
		}
		if strings.TrimSpace(text) != "" {
			sb.WriteString(text)
			sb.WriteByte('\n')
		}
	}
	return strings.TrimSpace(sb.String()), nil
}
