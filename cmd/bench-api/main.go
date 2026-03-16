package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"samebits.com/evidra-infra-bench/internal/api"
	"samebits.com/evidra-infra-bench/internal/executor"
	"samebits.com/evidra-infra-bench/pkg/store"
)

func main() {
	listenAddr := flag.String("listen", envOr("LISTEN_ADDR", ":8080"), "listen address")
	runsDir := flag.String("runs-dir", envOr("RUNS_DIR", "runs"), "runs directory (shared with CLI)")
	scenariosDir := flag.String("scenarios-dir", envOr("SCENARIOS_DIR", "scenarios"), "scenarios directory")
	flag.Parse()

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

	exec := executor.New(s, absScenariosDir, absRunsDir)
	srv := api.NewServer(s, exec, absScenariosDir)

	fmt.Printf("bench-api listening on %s\n", *listenAddr)
	fmt.Printf("  runs:      %s\n", absRunsDir)
	fmt.Printf("  scenarios: %s\n", absScenariosDir)

	if err := http.ListenAndServe(*listenAddr, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
