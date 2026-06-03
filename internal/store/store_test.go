package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// TestFilter_ZeroValues verifies that a zero-value Filter behaves correctly.
//
// This test ensures the Filter struct's semantics are clear: zero values mean
// "don't filter on this field."
func TestFilter_ZeroValues(t *testing.T) {
	var f Filter

	// Verify zero values
	if f.Selected != nil {
		t.Error("Expected Selected to be nil")
	}
	if f.MinScore != 0.0 {
		t.Errorf("Expected MinScore to be 0.0, got %f", f.MinScore)
	}
	if f.MaxScore != 0.0 {
		t.Errorf("Expected MaxScore to be 0.0, got %f", f.MaxScore)
	}
}

// TestFilter_SelectedPointer verifies that the Selected field works with pointer semantics.
func TestFilter_SelectedPointer(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		selected *bool
	}{
		{"nil", nil},
		{"true", &trueVal},
		{"false", &falseVal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := Filter{Selected: tt.selected}
			if f.Selected != tt.selected {
				t.Errorf("Expected Selected to be %v, got %v", tt.selected, f.Selected)
			}
		})
	}
}

// Contract tests for Store implementations
// These tests verify that any Store implementation (JSON, Firestore, etc.) conforms
// to the interface contract.

func TestJSONStore_Save(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	opp := &opportunity.Opportunity{
		ID:               "test-123",
		Title:            "Test Opportunity",
		SolicitationNum:  "SOL-001",
		Agency:           "Test Agency",
		PostedDate:       now,
		ResponseDeadline: now.Add(30 * 24 * time.Hour),
		NAICSCode:        "541512",
		Description:      "Test description",
		URL:              "https://sam.gov/test",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Save opportunity
	if err := store.Save(ctx, opp); err != nil {
		t.Fatalf("Failed to save opportunity: %v", err)
	}

	// Verify file was created
	filePath := filepath.Join(tempDir, "queue", "test-123.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected file to be created, but it doesn't exist")
	}
}

func TestJSONStore_SaveNilOpportunity(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	err = store.Save(ctx, nil)
	if err == nil {
		t.Error("Expected error when saving nil opportunity")
	}
}

func TestJSONStore_SaveEmptyID(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	opp := &opportunity.Opportunity{
		ID:    "",
		Title: "Test",
	}

	err = store.Save(ctx, opp)
	if err == nil {
		t.Error("Expected error when saving opportunity with empty ID")
	}
}

func TestJSONStore_Get(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	original := &opportunity.Opportunity{
		ID:               "test-456",
		Title:            "Test Opportunity Get",
		SolicitationNum:  "SOL-002",
		Agency:           "Test Agency",
		PostedDate:       now,
		ResponseDeadline: now.Add(30 * 24 * time.Hour),
		NAICSCode:        "541512",
		Description:      "Test description",
		URL:              "https://sam.gov/test/456",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Save opportunity
	if err := store.Save(ctx, original); err != nil {
		t.Fatalf("Failed to save opportunity: %v", err)
	}

	// Retrieve opportunity
	retrieved, err := store.Get(ctx, "test-456")
	if err != nil {
		t.Fatalf("Failed to get opportunity: %v", err)
	}

	// Verify fields match
	if retrieved.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, original.ID)
	}
	if retrieved.Title != original.Title {
		t.Errorf("Title mismatch: got %q, want %q", retrieved.Title, original.Title)
	}
	if !retrieved.PostedDate.Equal(original.PostedDate) {
		t.Errorf("PostedDate mismatch: got %v, want %v", retrieved.PostedDate, original.PostedDate)
	}
}

func TestJSONStore_GetNotFound(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	_, err = store.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error when getting nonexistent opportunity")
	}
}

func TestJSONStore_List(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	// Save multiple opportunities
	opportunities := []*opportunity.Opportunity{
		{
			ID:               "opp-1",
			Title:            "Opportunity 1",
			Agency:           "Agency A",
			PostedDate:       now,
			ResponseDeadline: now.Add(30 * 24 * time.Hour),
			NAICSCode:        "541512",
			Score:            0.8,
			Selected:         false,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "opp-2",
			Title:            "Opportunity 2",
			Agency:           "Agency B",
			PostedDate:       now,
			ResponseDeadline: now.Add(45 * 24 * time.Hour),
			NAICSCode:        "541519",
			Score:            0.6,
			Selected:         true,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "opp-3",
			Title:            "Opportunity 3",
			Agency:           "Agency C",
			PostedDate:       now,
			ResponseDeadline: now.Add(60 * 24 * time.Hour),
			NAICSCode:        "541512",
			Score:            0.9,
			Selected:         false,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}

	for _, opp := range opportunities {
		if err := store.Save(ctx, opp); err != nil {
			t.Fatalf("Failed to save opportunity %s: %v", opp.ID, err)
		}
	}

	// List all opportunities (no filter)
	all, err := store.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list opportunities: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 opportunities, got %d", len(all))
	}
}

func TestJSONStore_ListWithFilter(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	// Save multiple opportunities with different scores and selection status
	opportunities := []*opportunity.Opportunity{
		{ID: "opp-1", Score: 0.5, Selected: false, CreatedAt: now, UpdatedAt: now, Title: "A", Agency: "A", PostedDate: now, ResponseDeadline: now, NAICSCode: "541512"},
		{ID: "opp-2", Score: 0.7, Selected: true, CreatedAt: now, UpdatedAt: now, Title: "B", Agency: "B", PostedDate: now, ResponseDeadline: now, NAICSCode: "541512"},
		{ID: "opp-3", Score: 0.9, Selected: false, CreatedAt: now, UpdatedAt: now, Title: "C", Agency: "C", PostedDate: now, ResponseDeadline: now, NAICSCode: "541512"},
	}

	for _, opp := range opportunities {
		if err := store.Save(ctx, opp); err != nil {
			t.Fatalf("Failed to save opportunity: %v", err)
		}
	}

	tests := []struct {
		name     string
		filter   *Filter
		expected int
	}{
		{
			name:     "no filter",
			filter:   nil,
			expected: 3,
		},
		{
			name:     "filter by selected=true",
			filter:   &Filter{Selected: boolPtr(true)},
			expected: 1,
		},
		{
			name:     "filter by selected=false",
			filter:   &Filter{Selected: boolPtr(false)},
			expected: 2,
		},
		{
			name:     "filter by MinScore",
			filter:   &Filter{MinScore: 0.7},
			expected: 2,
		},
		{
			name:     "filter by MaxScore",
			filter:   &Filter{MaxScore: 0.7},
			expected: 2,
		},
		{
			name:     "filter by score range",
			filter:   &Filter{MinScore: 0.6, MaxScore: 0.8},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Failed to list opportunities: %v", err)
			}
			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
			}
		})
	}
}

func TestJSONStore_Delete(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	opp := &opportunity.Opportunity{
		ID:               "delete-test",
		Title:            "Delete Test",
		Agency:           "Test Agency",
		PostedDate:       now,
		ResponseDeadline: now.Add(30 * 24 * time.Hour),
		NAICSCode:        "541512",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Save opportunity
	if err := store.Save(ctx, opp); err != nil {
		t.Fatalf("Failed to save opportunity: %v", err)
	}

	// Verify it exists
	if _, err := store.Get(ctx, "delete-test"); err != nil {
		t.Fatalf("Opportunity should exist before deletion: %v", err)
	}

	// Delete opportunity
	if err := store.Delete(ctx, "delete-test"); err != nil {
		t.Fatalf("Failed to delete opportunity: %v", err)
	}

	// Verify it no longer exists
	if _, err := store.Get(ctx, "delete-test"); err == nil {
		t.Error("Expected error when getting deleted opportunity")
	}
}

func TestJSONStore_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	err = store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error when deleting nonexistent opportunity")
	}
}

func TestJSONStore_NewJSONStore_NonexistentBasePath(t *testing.T) {
	// Test creating store with non-existent base path - should be created automatically
	tempDir := t.TempDir()
	nonexistentPath := filepath.Join(tempDir, "nonexistent", "path")

	store, err := NewJSONStore(nonexistentPath)
	if err != nil {
		t.Fatalf("NewJSONStore() should create nonexistent directories, got error: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nonexistentPath); os.IsNotExist(err) {
		t.Error("Expected base path to be created")
	}

	// Verify queue subdirectory was created
	queuePath := filepath.Join(nonexistentPath, "queue")
	if _, err := os.Stat(queuePath); os.IsNotExist(err) {
		t.Error("Expected queue subdirectory to be created")
	}

	// Verify store is usable
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	opp := &opportunity.Opportunity{
		ID:        "test-new-path",
		Title:     "Test",
		Agency:    "Test Agency",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.Save(ctx, opp); err != nil {
		t.Errorf("Store should be usable after creating paths: %v", err)
	}
}

func TestJSONStore_GetEmptyID(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	_, err = store.Get(ctx, "")
	if err == nil {
		t.Error("Expected error when getting opportunity with empty ID")
	}
}

func TestJSONStore_DeleteEmptyID(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	err = store.Delete(ctx, "")
	if err == nil {
		t.Error("Expected error when deleting opportunity with empty ID")
	}
}

func TestJSONStore_ListWithCorruptedFiles(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	// Save a valid opportunity
	validOpp := &opportunity.Opportunity{
		ID:        "valid-opp",
		Title:     "Valid Opportunity",
		Agency:    "Test Agency",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Save(ctx, validOpp); err != nil {
		t.Fatalf("Failed to save valid opportunity: %v", err)
	}

	// Create a corrupted JSON file
	queuePath := filepath.Join(tempDir, "queue")
	corruptedPath := filepath.Join(queuePath, "corrupted.json")
	if err := os.WriteFile(corruptedPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to create corrupted file: %v", err)
	}

	// List should skip corrupted files and return valid ones
	results, err := store.List(ctx, nil)
	if err != nil {
		t.Fatalf("List() should not error on corrupted files: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 valid opportunity, got %d", len(results))
	}

	if len(results) > 0 && results[0].ID != "valid-opp" {
		t.Errorf("Expected to get valid-opp, got %s", results[0].ID)
	}
}

func TestJSONStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSONStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	// Test concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			opp := &opportunity.Opportunity{
				ID:               fmt.Sprintf("concurrent-%d", id),
				Title:            fmt.Sprintf("Concurrent Test %d", id),
				Agency:           "Test Agency",
				PostedDate:       now,
				ResponseDeadline: now.Add(30 * 24 * time.Hour),
				NAICSCode:        "541512",
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			if err := store.Save(ctx, opp); err != nil {
				t.Errorf("Failed to save opportunity %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	// Verify all opportunities were saved
	all, err := store.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list opportunities: %v", err)
	}
	if len(all) != 10 {
		t.Errorf("Expected 10 opportunities, got %d", len(all))
	}
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}
