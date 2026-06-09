// Package main is the entry point for the Kaimi pipeline dashboard server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Mawar2/Kaimi/internal/dashboard"
	"github.com/Mawar2/Kaimi/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Dashboard error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", getEnv("ADDR", ":8080"), "HTTP server address")
	storePath := flag.String("store-path", getEnv("STORE_PATH", "./queue"), "Store directory path")
	flag.Parse()

	fmt.Printf("Kaimi Dashboard starting on %s...\n", *addr)
	fmt.Printf("Store path: %s\n", *storePath)

	opportunityStore, err := store.NewJSONStore(*storePath)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	svc := dashboard.NewService(opportunityStore)
	handler := dashboard.NewHandler(svc)

	server := &http.Server{
		Addr:    *addr,
		Handler: handler,
	}

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down dashboard...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
