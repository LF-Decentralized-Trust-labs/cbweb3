// SPDX-License-Identifier: Apache-2.0

// Package apply implements the cbweb3 apply command logic, separated from the CLI
// layer so it can be unit-tested without invoking the binary via os/exec.
package apply

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"gopkg.in/yaml.v3"
)

// ApplyInput carries everything resolved from CLI flags and the parsed manifest.
// It is the boundary object between cmd/cbweb3/main.go and engine/apply.
type ApplyInput struct {
	Manifest   *manifest.Manifest
	DryRun     bool
	OutputFmt  string // "yaml" | "json"; default "yaml"
	OutputDir  string // where to write bundles/<spoke-id>.bundle.yaml
	BesuRPCURL string // http://localhost:<rpc.port> (local profile)
	// JoinBundlePath is the resolved absolute path to the join bundle (mode:join).
	JoinBundlePath string
	// CommercialBankComposePath / BackendComposePath are resolved (binary-relative)
	// paths for mode:join, passed through from the CLI profile.
	CommercialBankComposePath string
	BackendComposePath        string
	// CentralBankComposePath is the resolved (binary-relative) path to the TK-4
	// central-bank Besu compose template, used by the start-besu step (mode:found).
	CentralBankComposePath string
	// BesuImage is the pinned Besu image for the bootnode (mode:found).
	BesuImage string
	// PaladinImage is the pinned Paladin image for the spoke's Paladin nodes (mode:found).
	PaladinImage string
	// ContractsOutDir is the Foundry build output dir for the participant whitelist deploy.
	ContractsOutDir string
	// CommercialBankPaladinComposePath is the bank Paladin compose template (mode:join US2).
	CommercialBankPaladinComposePath string
	// ScriptsDir, ComposeTemplatePath (Paladin), and PaladinConfigDir are the
	// resolved (binary-relative) paths the mode:found engine needs. They are
	// threaded through here so the binary-relative resolution survives into the
	// engine instead of being dropped and re-resolved to empty.
	ScriptsDir          string
	ComposeTemplatePath string
	PaladinConfigDir    string
}

// ApplyResult is the structured execution report written to stdout.
// It is always built (possibly partially) before any stdout write.
type ApplyResult struct {
	Spoke  string       `json:"spoke"  yaml:"spoke"`
	Mode   string       `json:"mode"   yaml:"mode"`
	DryRun bool         `json:"dryRun" yaml:"dryRun"`
	Status string       `json:"status" yaml:"status"` // "success" | "failed" | "dry-run" | "interrupted"
	Steps  []StepResult `json:"steps"  yaml:"steps"`
	Bundle *BundleRef   `json:"bundle,omitempty" yaml:"bundle,omitempty"`
	Error  string       `json:"error,omitempty"  yaml:"error,omitempty"`
}

// StepResult records the outcome (live) or planned action (dry-run) for one step.
type StepResult struct {
	Name        string `json:"name"                  yaml:"name"`
	Status      string `json:"status"                yaml:"status"`
	CompletedAt string `json:"completedAt,omitempty" yaml:"completedAt,omitempty"`
	Error       string `json:"error,omitempty"       yaml:"error,omitempty"`
}

// BundleRef points to the emitted join bundle artifact.
type BundleRef struct {
	Path string `json:"path" yaml:"path"`
}

// EmitReport serializes result to w in the format specified by outputFmt ("yaml" or "json").
func EmitReport(w io.Writer, result ApplyResult, outputFmt string) error {
	switch outputFmt {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "yaml", "":
		data, err := yaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal report: %w", err)
		}
		_, err = w.Write(data)
		return err
	default:
		return fmt.Errorf("unknown output format: %s", outputFmt)
	}
}
