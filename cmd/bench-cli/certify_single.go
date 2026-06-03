package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
)

func executeCertifySingle(cmd *cobra.Command, cfg config.Config, track, model string) error {
	if _, ok := trackNames[track]; !ok {
		valid := make([]string, 0, len(trackNames))
		for k := range trackNames {
			valid = append(valid, k)
		}
		sort.Strings(valid)
		return fmt.Errorf("certify: unknown track %q (valid: %s)", track, strings.Join(valid, ", "))
	}

	if model == "" {
		return fmt.Errorf("certify: --model is required")
	}

	if !cfg.DryRun && cfg.Provider == "" {
		cfg.Provider = "claude"
	}

	cert, err := runCertifyTrack(cmd.Context(), cfg, track, model)
	if err != nil {
		return err
	}
	printCertification(cmd, *cert)
	if cert.ArtifactPath != "" {
		writef(cmd.OutOrStdout(), "\n  Artifacts: %s\n", cert.ArtifactPath)
	}

	if cert.Passed < cert.Total {
		return fmt.Errorf("certify: %d/%d scenarios passed", cert.Passed, cert.Total)
	}
	return nil
}
