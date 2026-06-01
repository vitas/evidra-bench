package main

import (
	"log"
	"path/filepath"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/localstore"
	"github.com/vitas/evidra-bench/pkg/report"
)

type localHarnessRuntime struct {
	Deps  harness.Deps
	Close func()
}

func buildLocalHarnessRuntime(cfg config.Config, envProvider environment.ClusterLifecycle, sharedStore *localstore.Store) (*localHarnessRuntime, error) {
	var agentAdapter adapter.Adapter
	var err error
	if cfg.Adapter != "a2a" {
		agentAdapter, err = resolveLocalAdapter(cfg.Adapter)
		if err != nil {
			return nil, err
		}
	}

	runner := &environment.ExecRunner{}
	bootstrapper := environment.NewBootstrapper(runner)
	writer := artifact.NewWriter(cfg.RunsDir)
	reporter := report.NewReporter(report.Config{
		EvidencePath: filepath.Join(cfg.RunsDir, "evidence"),
	})

	resultsStore := sharedStore
	closeFn := func() {}
	if resultsStore == nil {
		resultsStore, err = localstore.Open(cfg.RunsDir)
		if err != nil {
			log.Printf("[harness] warning: could not open results store: %v", err)
		}
		if resultsStore != nil {
			closeFn = func() {
				if closeErr := resultsStore.Close(); closeErr != nil {
					log.Printf("[harness] warning: could not close results store: %v", closeErr)
				}
			}
		}
	}

	return &localHarnessRuntime{
		Deps: harness.Deps{
			EnvProvider:  envProvider,
			Bootstrapper: bootstrapper,
			Adapter:      agentAdapter,
			Writer:       writer,
			Reporter:     reporter,
			Store:        resultsStore,
		},
		Close: closeFn,
	}, nil
}
