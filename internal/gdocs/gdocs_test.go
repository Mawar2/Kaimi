package gdocs

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// mockDocsAPI is a test double for docsAPI that records calls and returns
// configured results, so CreateDoc can be tested without hitting Google.
type mockDocsAPI struct {
	createID     string
	createErr    error
	insertErr    error
	createCalled bool
	insertCalled bool
	gotTitle     string
	gotText      string
}

func (m *mockDocsAPI) Create(_ context.Context, title string) (string, error) {
	m.createCalled = true
	m.gotTitle = title
	return m.createID, m.createErr
}

func (m *mockDocsAPI) InsertText(_ context.Context, _, text string) error {
	m.insertCalled = true
	m.gotText = text
	return m.insertErr
}

func TestCreateDoc_Success(t *testing.T) {
	mock := &mockDocsAPI{createID: "doc123"}
	c := New(mock)
	url, err := c.CreateDoc(context.Background(), "Title", "body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://docs.google.com/document/d/doc123/edit" {
		t.Errorf("url = %q, want the doc123 edit URL", url)
	}
	if mock.gotText != "body" {
		t.Errorf("inserted text = %q, want body", mock.gotText)
	}
	if !mock.insertCalled {
		t.Error("expected insert to be called")
	}
}

func TestCreateDoc_EmptyContent_NoInsert(t *testing.T) {
	mock := &mockDocsAPI{createID: "doc123"}
	c := New(mock)
	url, err := c.CreateDoc(context.Background(), "T", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected valid URL, got empty")
	}
	if mock.insertCalled {
		t.Error("expected insert to NOT be called")
	}
}

func TestCreateDoc_EmptyTitle_Error(t *testing.T) {
	mock := &mockDocsAPI{}
	c := New(mock)
	_, err := c.CreateDoc(context.Background(), "", "body")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
	if mock.createCalled {
		t.Error("expected create to NOT be called")
	}
}

func TestCreateDoc_CreateError(t *testing.T) {
	mock := &mockDocsAPI{createErr: errors.New("API error")}
	c := New(mock)
	url, err := c.CreateDoc(context.Background(), "T", "body")
	if err == nil {
		t.Fatal("expected error")
	}
	if url != "" {
		t.Errorf("expected empty url, got %q", url)
	}
}

func TestCreateDoc_InsertError_IncludesDocID(t *testing.T) {
	mock := &mockDocsAPI{createID: "docXYZ", insertErr: errors.New("API error")}
	c := New(mock)
	url, err := c.CreateDoc(context.Background(), "T", "body")
	if err == nil {
		t.Fatal("expected error")
	}
	if url != "" {
		t.Errorf("expected empty url, got %q", url)
	}
	if !strings.Contains(err.Error(), "docXYZ") {
		t.Errorf("error message should contain docID 'docXYZ', got: %v", err)
	}
}

func TestCreateDoc_E2E(t *testing.T) {
	if os.Getenv("KAI_M7_E2E") == "" {
		t.Skip("skipping live Google Docs E2E; set KAI_M7_E2E (and ADC) to enable")
	}
	ctx := context.Background()
	c, err := NewGoogleClient(ctx)
	if err != nil {
		t.Fatalf("NewGoogleClient failed: %v", err)
	}
	url, err := c.CreateDoc(ctx, "Kaimi E2E Test Doc", "This is an automated E2E test.")
	if err != nil {
		t.Fatalf("CreateDoc failed: %v", err)
	}
	if !strings.Contains(url, "docs.google.com/document/d/") {
		t.Errorf("unexpected URL format: %q", url)
	}
	t.Logf("E2E doc created: %s", url)
}
