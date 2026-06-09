// Package gdocs provides the Drive/Docs foundation (KAI-M7) for creating
// and modifying Google Docs.
// SECURITY-SENSITIVE: Uses new external Google Drive/Docs scope via
// Application Default Credentials (ADC).
// The docsAPI seam keeps the core logic unit-testable without network calls,
// while NewGoogleClient constructs a live client backed by ADC.
package gdocs

import (
	"context"
	"fmt"

	docs "google.golang.org/api/docs/v1"
)

// docsAPI is the minimal slice of Google Docs operations gdocs needs.
type docsAPI interface {
	Create(ctx context.Context, title string) (docID string, err error)
	InsertText(ctx context.Context, docID, text string) error
}

// Client creates Google Docs through a docsAPI seam.
type Client struct {
	api docsAPI
}

// New returns a Client backed by the given docsAPI (use NewGoogleClient for the
// live Google-backed client, or a mock in tests).
func New(api docsAPI) *Client {
	return &Client{api: api}
}

// CreateDoc creates a Doc titled title, writes content (if non-empty), and
// returns its URL https://docs.google.com/document/d/<docID>/edit.
//
// If the document is created but the content write fails, the returned error
// includes the document ID so the partially-created doc is not lost silently.
func (c *Client) CreateDoc(ctx context.Context, title, content string) (string, error) {
	if title == "" {
		return "", fmt.Errorf("title cannot be empty")
	}

	docID, err := c.api.Create(ctx, title)
	if err != nil {
		return "", fmt.Errorf("create doc: %w", err)
	}

	if content != "" {
		if err := c.api.InsertText(ctx, docID, content); err != nil {
			return "", fmt.Errorf("insert text into doc %s: %w", docID, err)
		}
	}

	return docURL(docID), nil
}

func docURL(id string) string {
	return fmt.Sprintf("https://docs.google.com/document/d/%s/edit", id)
}

// googleDocsAPI implements docsAPI using google.golang.org/api/docs/v1.
type googleDocsAPI struct {
	svc *docs.Service
}

// NewGoogleClient builds a Client backed by the live Google Docs API via ADC.
func NewGoogleClient(ctx context.Context) (*Client, error) {
	svc, err := docs.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create docs service: %w", err)
	}
	return New(&googleDocsAPI{svc: svc}), nil
}

// Create creates an empty document with the given title and returns its ID.
func (g *googleDocsAPI) Create(ctx context.Context, title string) (string, error) {
	doc, err := g.svc.Documents.Create(&docs.Document{Title: title}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("documents.create: %w", err)
	}
	return doc.DocumentId, nil
}

// InsertText inserts text at the start of the document body.
func (g *googleDocsAPI) InsertText(ctx context.Context, docID, text string) error {
	req := &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			InsertText: &docs.InsertTextRequest{
				Text:     text,
				Location: &docs.Location{Index: 1},
			},
		}},
	}
	if _, err := g.svc.Documents.BatchUpdate(docID, req).Context(ctx).Do(); err != nil {
		return fmt.Errorf("documents.batchUpdate: %w", err)
	}
	return nil
}
