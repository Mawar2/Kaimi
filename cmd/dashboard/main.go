// Package main implements the dashboard server for Kaimi.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mawar2/Kaimi/internal/dashboard"
	"github.com/Mawar2/Kaimi/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// newMux wires all dashboard routes to the shared branded handler (issue
// #147): it serves the pipeline overview — stage cards plus the opportunity
// table — at "/" (ux-spec View 1) and the opportunity detail at
// "/opportunity/{id}" (View 2). "/opportunities" stays mounted as an alias
// because the overview's filter form submits there.
func newMux(h *dashboard.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/", h)
	mux.Handle("/opportunities", h)
	mux.Handle("/opportunities/", h)
	return mux
}

func run() error {
	port := flag.Int("port", 8900, "Port to serve on")
	storePath := flag.String("store", "", "Path to the JSON store directory")
	flag.Parse()

	if *storePath == "" {
		return fmt.Errorf("--store path is required")
	}

	s, err := store.NewJSONStore(*storePath)
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}

	mux := newMux(dashboard.NewHandler(dashboard.NewService(s)))

	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", *port))
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Starting dashboard on http://%s", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("listen: %s\n", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server exiting")
	return nil
}
