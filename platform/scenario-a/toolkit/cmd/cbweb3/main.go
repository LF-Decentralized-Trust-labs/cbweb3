// SPDX-License-Identifier: Apache-2.0

// cbweb3 is the CLI for the Scenario A provisioning toolkit.
// Usage: cbweb3 apply -f <manifest.yaml> [--dry-run] [--output json|yaml]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/apply"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: cbweb3 <subcommand> [flags]")
		fmt.Fprintln(os.Stderr, "  Subcommands: apply")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "apply":
		os.Exit(runApply(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "  Subcommands: apply")
		os.Exit(1)
	}
}

func runApply(args []string) int {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		manifestFile string
		dryRun       bool
		outputFmt    string
	)
	fs.StringVar(&manifestFile, "f", "", "path to the ParticipantDeployment YAML manifest (required)")
	fs.StringVar(&manifestFile, "file", "", "path to the ParticipantDeployment YAML manifest (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "show plan without executing")
	fs.BoolVar(&dryRun, "n", false, "alias for --dry-run")
	fs.StringVar(&outputFmt, "output", "yaml", "output format: json | yaml")
	fs.StringVar(&outputFmt, "o", "yaml", "alias for --output")

	if err := fs.Parse(args); err != nil {
		// flag.ContinueOnError: error already written to stderr by flag package.
		return 1
	}

	// Validate --output before touching the manifest.
	if outputFmt != "json" && outputFmt != "yaml" {
		fmt.Fprintf(os.Stderr, "unknown output format: %s\n", outputFmt)
		return 1
	}

	// -f is mandatory.
	if manifestFile == "" {
		fmt.Fprintln(os.Stderr, "required flag -f (--file) is missing")
		return 1
	}

	// Load and validate manifest.
	m, err := manifest.Load(manifestFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "manifest file not found: %v\n", err)
		return 1
	}
	if err := manifest.Validate(m); err != nil {
		fmt.Fprintf(os.Stderr, "validation error: %v\n", err)
		return 1
	}

	// Guard unsupported modes and environments.
	if m.Spec.Mode != "found" {
		fmt.Fprintf(os.Stderr, "mode: %s not yet supported — will be implemented in TK-9\n", m.Spec.Mode)
		return 1
	}
	if m.Spec.Environment != "local" {
		fmt.Fprintf(os.Stderr, "environment %s is not yet supported in TK-7 — only local is available\n", m.Spec.Environment)
		return 1
	}

	// Resolve binary-relative paths.
	exPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot determine executable path: %v\n", err)
		return 1
	}
	exDir := filepath.Dir(exPath)

	rpcPort := 0
	if m.Spec.Node.RPC != nil {
		rpcPort = m.Spec.Node.RPC.Port
	}
	profile := apply.LocalProfileFromExDir(exDir, m.Spec.Node.DataDir, rpcPort)

	in := apply.ApplyInput{
		Manifest:   m,
		DryRun:     dryRun,
		OutputFmt:  outputFmt,
		OutputDir:  profile.OutputDir,
		BesuRPCURL: profile.BesuRPCURL,
	}

	// Set up signal handling so Ctrl-C produces a partial report.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var result apply.ApplyResult
	if dryRun {
		result, err = apply.DryRun(ctx, in)
	} else {
		result, err = apply.Run(ctx, in)
	}

	// Emit structured report to stdout (always, even on failure).
	if emitErr := apply.EmitReport(os.Stdout, result, outputFmt); emitErr != nil {
		fmt.Fprintf(os.Stderr, "failed to emit report: %v\n", emitErr)
	}

	if err != nil || result.Status == "failed" || result.Status == "interrupted" {
		return 1
	}
	return 0
}
