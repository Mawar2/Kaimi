// Package main is the entry point for the Kaimi dashboard HTTP server.
// It serves a read-only web UI for monitoring the federal BD pipeline.
//
// Usage:
//
//	dashboard -addr :8080 -store-dir ./data
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/Mawar2/Kaimi/internal/dashboard"
	"github.com/Mawar2/Kaimi/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	storeDir := flag.String("store-dir", "./data", "base directory for the JSON opportunity store")
	flag.Parse()

	s, err := store.NewJSONStore(*storeDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	svc := dashboard.NewService(s)
	h := dashboard.NewHandler(svc)

	fmt.Printf("Kaimi dashboard at http://localhost%s\n", *addr)
	if err := http.ListenAndServe(*addr, h); err != nil {
		log.Fatal(err)
	}
}
