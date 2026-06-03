package store_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/store"
)

// Example_jsonStore demonstrates basic usage of the JSONStore.
// This example shows how to create a store, save opportunities, retrieve them,
// list with filters, and delete them.
func Example_jsonStore() {
	// Create a temporary directory for this example
	tmpDir, err := os.MkdirTemp("", "kaimi-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a new JSONStore
	s, err := store.NewJSONStore(tmpDir)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Save an opportunity
	opp := &opportunity.Opportunity{
		ID:               "EXAMPLE-001",
		Title:            "Example Contract Opportunity",
		SolicitationNum:  "SOL-2026-001",
		Agency:           "Department of Example",
		Office:           "Office of Testing",
		PostedDate:       now,
		ResponseDeadline: now.Add(30 * 24 * time.Hour),
		NAICSCode:        "541512",
		NAICSDescription: "Computer Systems Design Services",
		Description:      "Example federal contracting opportunity",
		Type:             "Solicitation",
		URL:              "https://sam.gov/example/001",
		Score:            0.85,
		Selected:         false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.Save(ctx, opp); err != nil {
		log.Fatal(err)
	}

	// Retrieve the opportunity
	retrieved, err := s.Get(ctx, "EXAMPLE-001")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Retrieved: %s\n", retrieved.Title)

	// List all opportunities
	all, err := s.List(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total opportunities: %d\n", len(all))

	// List high-scoring opportunities
	highScore := &store.Filter{MinScore: 0.8}
	highScoring, err := s.List(ctx, highScore)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("High-scoring opportunities: %d\n", len(highScoring))

	// Verify the JSON file was created
	jsonPath := filepath.Join(tmpDir, "queue", "EXAMPLE-001.json")
	if _, err := os.Stat(jsonPath); err == nil {
		fmt.Println("JSON file created: yes")
	}

	// Output:
	// Retrieved: Example Contract Opportunity
	// Total opportunities: 1
	// High-scoring opportunities: 1
	// JSON file created: yes
}

// Example_swappableImplementation demonstrates how the Store interface
// enables swapping implementations (e.g., JSON -> Firestore) without
// changing client code.
func Example_swappableImplementation() {
	tmpDir, err := os.MkdirTemp("", "kaimi-swap-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	// Client code works with the Store interface, not the concrete type
	var s store.Store
	s, err = store.NewJSONStore(tmpDir)
	if err != nil {
		log.Fatal(err)
	}

	// This code will work identically when s is a Firestore implementation
	now := time.Now().UTC().Truncate(time.Second)
	opp := &opportunity.Opportunity{
		ID:        "SWAP-001",
		Title:     "Swappable Store Example",
		Agency:    "Test Agency",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.Save(ctx, opp); err != nil {
		log.Fatal(err)
	}

	retrieved, err := s.Get(ctx, "SWAP-001")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Implementation: JSONStore\n")
	fmt.Printf("Retrieved ID: %s\n", retrieved.ID)

	// Output:
	// Implementation: JSONStore
	// Retrieved ID: SWAP-001
}
