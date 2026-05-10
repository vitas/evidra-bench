package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"samebits.com/evidra-infra-bench/pkg/config"
)

func newServeCommand(cfg *config.Config) *cobra.Command {
	opts := serveOptions{}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the bench service REST API",
		RunE: func(cmd *cobra.Command, args []string) error {
			runOpts := opts
			addr := os.Getenv("BENCH_SERVICE_ADDR")
			if addr == "" {
				addr = ":8090"
			}
			runOpts = applyServeEnvOptions(runOpts)
			// Override config from environment for containerized deployment.
			if v := os.Getenv("INFRA_BENCH_SCENARIOS_DIR"); v != "" {
				cfg.ScenariosDir = v
			}
			if v := os.Getenv("INFRA_BENCH_PROVIDER"); v != "" {
				cfg.Provider = v
			}
			if v := os.Getenv("INFRA_BENCH_MODEL"); v != "" {
				cfg.Model = v
			}
			if v := os.Getenv("INFRA_BENCH_MCP_SERVER"); v != "" {
				cfg.MCPServer = v
			}
			if v := os.Getenv("INFRA_BENCH_TOOL_SERVER_ID"); v != "" {
				cfg.ToolServerID = v
			}
			if v := os.Getenv("INFRA_BENCH_TOOL_SERVER_VERSION"); v != "" {
				cfg.ToolServerVersion = v
			}
			if v := os.Getenv("INFRA_BENCH_CLUSTER_NAME"); v != "" {
				cfg.ClusterName = v
			}
			if os.Getenv("INFRA_BENCH_REUSE_CLUSTER") == "true" {
				cfg.ReuseCluster = true
			}
			if v := os.Getenv("KUBECONFIG"); v != "" {
				cfg.KubeconfigPath = v
			}
			if v := os.Getenv("INFRA_BENCH_PARALLEL"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.Parallel = n
				}
			}
			return serveAPI(*cfg, addr, runOpts)
		},
	}
	cmd.Flags().BoolVar(&opts.ControlPlaneOnly, "control-plane-only", false, "start API/control plane without provisioning a local executor cluster (env: BENCH_CONTROL_PLANE_ONLY)")
	return cmd
}

func isTruthyEnv(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func applyServeEnvOptions(opts serveOptions) serveOptions {
	if isTruthyEnv("BENCH_CONTROL_PLANE_ONLY") {
		opts.ControlPlaneOnly = true
	}
	return opts
}
