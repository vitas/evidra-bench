package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	evidrastand "samebits.com/evidra-infra-bench"
	"samebits.com/evidra-infra-bench/internal/api"
	"samebits.com/evidra-infra-bench/internal/executor"
	"samebits.com/evidra-infra-bench/pkg/store"
)

var (
	version = "dev"
	commit  = "dev"
)

func main() {
	listenAddr := flag.String("listen", envOr("LISTEN_ADDR", ":8080"), "listen address")
	runsDir := flag.String("runs-dir", envOr("RUNS_DIR", "runs"), "runs directory (shared with CLI)")
	scenariosDir := flag.String("scenarios-dir", envOr("SCENARIOS_DIR", "scenarios"), "scenarios directory")
	readonly := flag.Bool("readonly", envOr("READONLY", "") != "", "disable execute endpoints (demo mode)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("bench-api %s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	absRunsDir, err := filepath.Abs(*runsDir)
	if err != nil {
		log.Fatalf("resolve runs dir: %v", err)
	}
	absScenariosDir, err := filepath.Abs(*scenariosDir)
	if err != nil {
		log.Fatalf("resolve scenarios dir: %v", err)
	}

	s, err := store.Open(absRunsDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	var exec *executor.Executor
	if !*readonly {
		exec = executor.New(s, absScenariosDir, absRunsDir)
	}

	srv := api.NewServer(s, exec, absScenariosDir, version)
	handler := srv.Handler()

	// Serve embedded UI if available (built with -tags embed_ui).
	if evidrastand.UIDistFS != nil {
		handler = withUI(evidrastand.UIDistFS, handler)
		fmt.Println("  ui:        embedded")
	}

	mode := "read-write"
	if *readonly {
		mode = "read-only (demo)"
	}

	fmt.Printf("bench-api listening on %s\n", *listenAddr)
	fmt.Printf("  runs:      %s\n", absRunsDir)
	fmt.Printf("  scenarios: %s\n", absScenariosDir)
	fmt.Printf("  mode:      %s\n", mode)

	if err := http.ListenAndServe(*listenAddr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// withUI wraps the API handler to serve the embedded UI for non-API paths.
// API paths (/v1/*, /healthz) pass through to the API handler.
// Everything else serves the SPA (index.html for client-side routing).
func withUI(uiFS fs.FS, apiHandler http.Handler) http.Handler {
	fileServer := http.FileServerFS(uiFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API routes pass through.
		if len(path) >= 3 && path[:3] == "/v1" || path == "/healthz" {
			apiHandler.ServeHTTP(w, r)
			return
		}

		// Try to serve static file.
		if f, err := uiFS.Open(path[1:]); err == nil {
			f.Close()
			setCacheHeaders(w, path)
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for client-side routing.
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// setCacheHeaders sets Cache-Control based on whether the asset has a
// content-hash in the filename (Vite's /assets/* output).
func setCacheHeaders(w http.ResponseWriter, path string) {
	if strings.HasPrefix(path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
