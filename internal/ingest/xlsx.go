package ingest

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// xlsxContentType is the MIME type for .xlsx (OOXML SpreadsheetML) files.
const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// isXLSX reports whether contentType denotes a .xlsx spreadsheet.
func isXLSX(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "spreadsheetml.sheet")
}

// looksLikeXLSX reports whether raw is structurally a .xlsx (a ZIP containing
// xl/workbook.xml), for when the served content type is generic.
func looksLikeXLSX(raw []byte) bool {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return false
	}
	for _, f := range zr.File {
		if f.Name == "xl/workbook.xml" {
			return true
		}
	}
	return false
}

// extractXLSX returns the cell text of a .xlsx using only the standard library.
// A .xlsx is a ZIP; nearly all human-readable text is pooled in
// xl/sharedStrings.xml as a list of strings (<si>) that cells reference by index.
// We emit each shared string on its own line — enough to ground a draft on a Q&A
// or pricing sheet — without resolving cell coordinates (numbers and formulas
// carry little prose). No third-party dependency is needed.
func extractXLSX(raw []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("xlsx: open zip: %w", err)
	}

	var ss *zip.File
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			ss = f
			break
		}
	}
	if ss == nil {
		return "", nil // a numeric-only workbook has no shared strings; a gap, not an error
	}

	rc, err := ss.Open()
	if err != nil {
		return "", fmt.Errorf("xlsx: open sharedStrings: %w", err)
	}
	defer func() { _ = rc.Close() }()

	return parseSharedStrings(rc)
}

// parseSharedStrings emits the text of each <si> shared-string item on its own
// line. Text lives in <t> elements (a plain string, or one per <r> rich-text run
// within an <si>), so runs inside one item are concatenated and the item is
// flushed as a line at </si>.
func parseSharedStrings(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var out strings.Builder
	var item strings.Builder
	inText := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("xlsx: parse sharedStrings: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "si":
				if s := strings.TrimSpace(item.String()); s != "" {
					out.WriteString(s)
					out.WriteByte('\n')
				}
				item.Reset()
			}
		case xml.CharData:
			if inText {
				item.Write(t)
			}
		}
	}
	return strings.TrimSpace(out.String()), nil
}
