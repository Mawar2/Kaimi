package ingest

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// docxContentType is the MIME type for .docx (OOXML WordprocessingML) files.
const docxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// isDOCX reports whether contentType denotes a .docx document.
func isDOCX(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "wordprocessingml.document")
}

// extractDOCX returns the plain text of a .docx file using only the standard
// library: a .docx is a ZIP archive whose word/document.xml holds the body as
// WordprocessingML. We stream that XML and emit the text runs (<w:t>), inserting
// a newline at each paragraph (</w:p>) and tabs/line-breaks where the markup asks
// for them. No third-party dependency is needed for this format.
func extractDOCX(raw []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("docx: open zip: %w", err)
	}

	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", fmt.Errorf("docx: word/document.xml not found (not a Word document?)")
	}

	rc, err := doc.Open()
	if err != nil {
		return "", fmt.Errorf("docx: open document.xml: %w", err)
	}
	defer func() { _ = rc.Close() }() // read-only close; nothing to surface

	return parseDocumentXML(rc)
}

// parseDocumentXML walks the WordprocessingML token stream and assembles text.
func parseDocumentXML(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var sb strings.Builder
	inText := false // currently inside a <w:t> element

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("docx: parse xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t": // text run
				inText = true
			case "tab":
				sb.WriteByte('\t')
			case "br", "cr": // explicit line break
				sb.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p": // end of paragraph
				sb.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}

	return strings.TrimSpace(sb.String()), nil
}
