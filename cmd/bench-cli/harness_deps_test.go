package main

import (
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/localstore"
)

func TestBuildLocalHarnessRuntimeUsesSharedStore(t *testing.T) {
	t.Parallel()

	shared, err := localstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open shared store: %v", err)
	}
	defer func() {
		if err := shared.Close(); err != nil {
			t.Fatalf("close shared store: %v", err)
		}
	}()

	cfg := config.Default()
	cfg.RunsDir = t.TempDir()
	cfg.Adapter = "a2a"

	rt, err := buildLocalHarnessRuntime(cfg, nil, shared)
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	defer rt.Close()

	if rt.Deps.Store != shared {
		t.Fatalf("expected shared store to be reused")
	}
}
