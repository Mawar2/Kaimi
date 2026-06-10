// Package main implements the dashboard server for Kaimi.
package main

import (
	"context"
	"encoding/json"
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
	"github.com/Mawar2/Kaimi/internal/document"
	"github.com/Mawar2/Kaimi/internal/finalreview"
	"github.com/Mawar2/Kaimi/internal/googledocs"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/proposal"
	"github.com/Mawar2/Kaimi/internal/scorer"
	"github.com/Mawar2/Kaimi/internal/store"
	"github.com/Mawar2/Kaimi/internal/writer"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// newMux wires all dashboard routes to the shared branded handler (issue
// #147): the Triage overview at "/", the opportunity detail, and the Zone 2
// surfaces (proposals, workspace, gate actions — issue #156).
// "/opportunities" stays mounted as an alias because the overview's filter
// form submits there.
func newMux(h *dashboard.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/", h)
	mux.Handle("/opportunities", h)
	mux.Handle("/opportunities/", h)
	return mux
}

// newProposalService wires the REAL Zone 2 agents behind the shared gated
// lifecycle (epic #153): the Outline agent (cached docs client unless live
// Google credentials are configured elsewhere), the Technical Writer (stub
// by default; -live-writer drafts with Gemini over Vertex AI ADC), and the
// Final Review agent (deterministic checks by default; -live-review adds the
// Gemini compliance pass over Vertex AI ADC).
//
// Note: the live compliance pass only fires once the orchestrator threads
// solicitation document text into finalreview.Input.Documents (see #172); until
// then it is constructed but skips for lack of documents.
func newProposalService(s store.Store, basePath string, liveWriter, liveReview bool, profilePath string) (*proposal.Service, error) {
	docs, err := document.NewStore(basePath)
	if err != nil {
		return nil, fmt.Errorf("document store: %w", err)
	}
	docsClient, err := googledocs.NewClient(context.Background(), googledocs.Config{UseCached: true})
	if err != nil {
		return nil, fmt.Errorf("docs client: %w", err)
	}

	profile := &scorer.CapabilityProfile{}
	if profilePath != "" {
		data, err := os.ReadFile(profilePath)
		if err != nil {
			return nil, fmt.Errorf("read profile: %w", err)
		}
		if err := json.Unmarshal(data, profile); err != nil {
			return nil, fmt.Errorf("parse profile: %w", err)
		}
	}

	w := writer.New()
	if liveWriter {
		projectID := envOr("GCP_PROJECT_ID", "")
		if projectID == "" {
			return nil, fmt.Errorf("-live-writer requires GCP_PROJECT_ID")
		}
		gen, err := writer.NewGeminiGenerator(context.Background(),
			projectID, envOr("GCP_REGION", "us-east4"), envOr("GEMINI_MODEL", "gemini-2.5-pro"))
		if err != nil {
			return nil, fmt.Errorf("gemini generator: %w", err)
		}
		w = writer.NewWithGenerator(gen)
		log.Printf("Technical Writer: LIVE Gemini drafting enabled (project %s)", projectID)
	} else {
		log.Printf("Technical Writer: stub mode (pass -live-writer for Gemini drafting)")
	}

	review := finalreview.New()
	if liveReview {
		projectID := envOr("GCP_PROJECT_ID", "")
		if projectID == "" {
			return nil, fmt.Errorf("-live-review requires GCP_PROJECT_ID")
		}
		checker, err := finalreview.NewGeminiComplianceChecker(context.Background(),
			projectID, envOr("GCP_REGION", "us-east4"), envOr("GEMINI_MODEL", "gemini-2.5-pro"))
		if err != nil {
			return nil, fmt.Errorf("gemini compliance checker: %w", err)
		}
		review = finalreview.NewWithComplianceChecker(checker)
		log.Printf("Final Review: LIVE Gemini compliance pass enabled (project %s)", projectID)
	} else {
		log.Printf("Final Review: deterministic checks only (pass -live-review for Gemini compliance)")
	}

	return proposal.NewService(&proposal.Deps{
		Opportunities: s,
		Documents:     docs,
		Outline:       outline.New(docsClient),
		Writer:        w,
		Review:        review,
		Profile:       profile,
	}), nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run() error {
	port := flag.Int("port", 8900, "Port to serve on")
	storePath := flag.String("store", "", "Path to the JSON store directory")
	liveWriter := flag.Bool("live-writer", false, "Draft with the real Gemini writer (Vertex AI ADC; needs GCP_PROJECT_ID)")
	liveReview := flag.Bool("live-review", false, "Run the Gemini compliance pass in Final Review (Vertex AI ADC; needs GCP_PROJECT_ID)")
	profilePath := flag.String("profile", "", "Capability profile JSON for grounding the writer (optional)")
	flag.Parse()

	if *storePath == "" {
		return fmt.Errorf("--store path is required")
	}

	s, err := store.NewJSONStore(*storePath)
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}

	proposals, err := newProposalService(s, *storePath, *liveWriter, *liveReview, *profilePath)
	if err != nil {
		return fmt.Errorf("failed to wire proposal service: %w", err)
	}

	mux := newMux(dashboard.NewHandler(dashboard.NewService(s), dashboard.WithProposals(proposals)))

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
	// Let in-flight agent stages persist their final status.
	proposals.Wait()

	log.Println("Server exiting")
	return nil
}
