package main

import (
	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
)

func newCertifyCommand() *cobra.Command {
	track := ""
	model := ""
	cfg := config.Default()

	cmd := &cobra.Command{
		Use:   "certify",
		Short: "Run all scenarios in a track and produce a certification grade",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeCertify(cmd, cfg, track, model)
		},
	}
	f := cmd.Flags()
	f.StringVar(&track, "track", "", "certification track (workloads, troubleshooting, networking, storage, pod-security, runtime-security, release-ops, platform-eng)")
	f.StringVar(&model, "model", "", "model name (e.g. sonnet, opus)")
	registerExecutionFlags(f, &cfg, executionFlagOptions{DryRunUsage: "validate without running"})
	registerAgentFlags(f, &cfg, agentFlagOptions{})
	registerResultMetadataFlags(f, &cfg, resultMetadataFlagOptions{})
	_ = cmd.MarkFlagRequired("track")
	_ = cmd.MarkFlagRequired("model")
	return cmd
}
