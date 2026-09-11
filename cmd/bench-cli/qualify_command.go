package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/qualification"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// newQualifyCommand exposes the ADR 0001 §8 qualification ledger:
//
//	qualify verify --scenario-dir DIR --provider kind --kubeconfig KC
//	qualify record --scenario-dir DIR ... --matrix-file matrix.json
//
// verify recomputes this environment's input digests and reports whether
// the stored ledger entry authorizes qualified=true. record writes a new
// entry pinned to the current digests — a deliberate, human-driven act
// after the qualification matrix passed; nothing records automatically.
// errNotAuthorized marks the demote-to-preview exit path (nonzero status,
// no usage spam).
var errNotAuthorized = errors.New("qualification not authorized: case stays preview")

func newQualifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "qualify",
		Short: "Verify or record per-scenario qualification ledger entries",
	}

	cmd.AddCommand(newQualifyVerifyCommand(), newQualifyRecordCommand())
	return cmd
}

type qualifyOpts struct {
	scenarioDir, provider, kubeconfig, providerVersion string
}

func (o *qualifyOpts) bind(c *cobra.Command) {
	c.Flags().StringVar(&o.scenarioDir, "scenario-dir", "", "scenario directory containing scenario.yaml (required)")
	c.Flags().StringVar(&o.provider, "provider", "kind", "environment provider name (kind|k3d)")
	c.Flags().StringVar(&o.kubeconfig, "kubeconfig", "", "kubeconfig for the live server-version probe")
	c.Flags().StringVar(&o.providerVersion, "provider-version", "", "explicit provider version (skips the probe; use \"<provider>@<gitVersion>\" forms to match a run)")
	_ = c.MarkFlagRequired("scenario-dir")
}

// inputs computes the run-equivalent digest set for the flags given.
func (o *qualifyOpts) inputs(ctx context.Context) (qualification.Inputs, *qualification.Verdict, error) {
	s, err := scenario.Load(o.scenarioDir)
	if err != nil {
		return qualification.Inputs{}, nil, err
	}
	if s.AuthorityProfile == nil {
		return qualification.Inputs{}, nil, fmt.Errorf("%s has no authority_profile: this case can never qualify", s.ID)
	}
	v, in := harness.ComputeQualification(ctx, s, o.kubeconfig, o.provider, o.providerVersion)
	return in, v, nil
}

func newQualifyVerifyCommand() *cobra.Command {
	var o qualifyOpts
	c := &cobra.Command{
		Use:   "verify",
		Short: "Recompute digests and report whether the ledger authorizes qualification",
		RunE: func(c *cobra.Command, _ []string) error {
			in, v, err := o.inputs(c.Context())
			if err != nil {
				return err
			}
			out := map[string]any{
				"authorized": v.Authorized, "missing": v.Missing,
				"reasons": v.Reasons, "inputs": in,
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(data))
			if !v.Authorized {
				c.SilenceErrors = true
				return errNotAuthorized
			}
			return nil
		},
	}
	o.bind(c)
	return c
}

func newQualifyRecordCommand() *cobra.Command {
	var (
		o                qualifyOpts
		matrixFile       string
		notes            string
		knownGoodPasses  int
		knownGoodRequire int
	)
	c := &cobra.Command{
		Use:   "record",
		Short: "Write a ledger entry pinned to current inputs (post-matrix act)",
		RunE: func(c *cobra.Command, _ []string) error {
			in, v, err := o.inputs(c.Context())
			if err != nil {
				return err
			}
			matrix := qualification.MatrixEvidence{
				KnownGoodPasses: knownGoodPasses, KnownGoodRequired: knownGoodRequire,
			}
			if matrixFile != "" {
				data, err := os.ReadFile(matrixFile)
				if err != nil {
					return err
				}
				if err := json.Unmarshal(data, &matrix); err != nil {
					return fmt.Errorf("matrix file: %w", err)
				}
			}
			if v.Authorized {
				return fmt.Errorf("ledger already authorizes %s for %s; nothing to do",
					o.scenarioDir, strings.Join(in.Providers, "+"))
			}
			s, err := scenario.Load(o.scenarioDir)
			if err != nil {
				return err
			}
			entry := &qualification.Entry{
				Schema: qualification.SchemaID, ScenarioID: s.ID, Qualified: true,
				GrantedAt: time.Now().UTC(), Inputs: in, Matrix: matrix, Notes: notes,
			}
			// A cross-provider matrix grants one case per leg; the second
			// (or nth) record UNIONs its pin into the existing entry instead
			// of clobbering the first provider's qualification.
			prev, perr := qualification.Load(o.scenarioDir)
			if perr != nil {
				return perr
			}
			if prev != nil && prev.Qualified {
				if !prev.Inputs.EqualModuloProviders(in) {
					return fmt.Errorf("inputs drifted since %s was granted (%s); delete the ledger file to re-grant from scratch", qualification.FileName, o.scenarioDir)
				}
				if !prev.Matrix.Complete() {
					return fmt.Errorf("existing entry has an incomplete matrix; refusing to extend it")
				}
				if prev.Matrix.KnownGoodRequired > matrix.KnownGoodRequired {
					matrix.KnownGoodRequired = prev.Matrix.KnownGoodRequired
				}
				entry.Inputs.Providers = qualification.UnionProviders(prev.Inputs.Providers, in.Providers)
				entry.Matrix = qualification.MergeMatrices(prev.Matrix, matrix)
				entry.GrantedAt = prev.GrantedAt
				if notes == "" {
					entry.Notes = prev.Notes
				}
			}
			if !matrix.Complete() {
				fmt.Fprintln(os.Stderr, "warning: matrix evidence incomplete; the entry will NOT authorize runs")
			}
			if err := qualification.Write(o.scenarioDir, entry); err != nil {
				return err
			}
			fmt.Printf("qualification entry written to %s/%s (component %s, provider %s)\n",
				o.scenarioDir, qualification.FileName, in.ComponentRevis, strings.Join(in.Providers, ","))
			return nil
		},
	}
	o.bind(c)
	c.Flags().StringVar(&matrixFile, "matrix-file", "", "JSON file with the MatrixEvidence attestation")
	c.Flags().StringVar(&notes, "notes", "", "free-text provenance note")
	c.Flags().IntVar(&knownGoodPasses, "known-good-passes", 0, "known-good consecutive passes achieved")
	c.Flags().IntVar(&knownGoodRequire, "known-good-required", 5, "known-good passes required by the flake budget")
	return c
}
