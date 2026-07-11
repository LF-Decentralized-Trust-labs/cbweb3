// SPDX-License-Identifier: Apache-2.0

// Command cbweb3b is the Scenario B toolkit CLI.
//
// Usage:
//
//	cbweb3b validate -f <manifest.yaml> [-f <manifest.yaml> ...] [-o json|yaml]
//	cbweb3b apply    -f <manifest.yaml> [--dry-run] [-o json|yaml]
//	                 [--data-dir <dir>] [--out-dir <dir>] [--repo-root <dir>]
//	                 [--hub-rpc <url>] [--hub-ws <url>]
//
// `validate` parses/validates manifests and reports (no effects). `apply` runs
// the orchestrator for the manifest's mode (found-hub or found-spoke): with
// --dry-run it plans without effects; without it, it executes. join is not
// supported yet (TK-B8).
//
// Exit codes: 0 = success (valid / all steps done|skipped|planned);
// 1 = validation/config error or a failed step; 2 = usage/parse error.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/yaml.v3"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/apply"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/manifest"
)

const (
	exitValid   = 0
	exitInvalid = 1
	exitUsage   = 2
)

type fileList []string

func (f *fileList) String() string { return fmt.Sprintf("%v", []string(*f)) }
func (f *fileList) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "apply":
		return runApply(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return exitValid
	default:
		fmt.Fprintf(stderr, "cbweb3b: unknown command %q\n", args[0])
		usage(stderr)
		return exitUsage
	}
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	var files fileList
	var output string
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Var(&files, "f", "manifest file (repeatable)")
	fs.Var(&files, "file", "manifest file (repeatable)")
	fs.StringVar(&output, "o", "yaml", "output format: json|yaml")
	fs.StringVar(&output, "output", "yaml", "output format: json|yaml")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if output != "json" && output != "yaml" {
		fmt.Fprintf(stderr, "cbweb3b: invalid -o value %q; must be json or yaml\n", output)
		return exitUsage
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "cbweb3b: at least one -f/--file is required")
		usage(stderr)
		return exitUsage
	}
	manifests, err := manifest.LoadSet(files)
	if err != nil {
		fmt.Fprintf(stderr, "cbweb3b: %v\n", err)
		return exitUsage
	}
	report := buildReport(files, manifests)
	if err := emit(stdout, output, report); err != nil {
		fmt.Fprintf(stderr, "cbweb3b: %v\n", err)
		return exitUsage
	}
	if report.Valid {
		return exitValid
	}
	return exitInvalid
}

func runApply(args []string, stdout, stderr io.Writer) int {
	var files fileList
	var output, dataDir, outDir, repoRoot, hubRPC, hubWS, spokeRPC, spokeWS, gatewayURL, cbAddr, relay string
	var dryRun bool
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Var(&files, "f", "manifest file")
	fs.Var(&files, "file", "manifest file")
	fs.StringVar(&output, "o", "yaml", "output format: json|yaml")
	fs.StringVar(&output, "output", "yaml", "output format: json|yaml")
	fs.BoolVar(&dryRun, "dry-run", false, "plan the steps without executing effects")
	fs.StringVar(&dataDir, "data-dir", "", "state/lock directory (default: manifest node.dataDir)")
	fs.StringVar(&outDir, "out-dir", "", "bundle output directory (default: data-dir)")
	fs.StringVar(&repoRoot, "repo-root", ".", "repository root (for contracts/templates)")
	fs.StringVar(&hubRPC, "hub-rpc", "", "hub RPC URL (readiness gate)")
	fs.StringVar(&hubWS, "hub-ws", "", "hub WS URL")
	fs.StringVar(&spokeRPC, "spoke-rpc", "", "spoke RPC URL (found-spoke)")
	fs.StringVar(&spokeWS, "spoke-ws", "", "spoke WS URL (found-spoke → relay registration + spoke bundle)")
	fs.StringVar(&gatewayURL, "gateway-url", "", "spoke gateway URL (found-spoke → relay)")
	fs.StringVar(&cbAddr, "cb-address", "", "central bank address (found-spoke → register-cb)")
	fs.StringVar(&relay, "relay", "", "RelayRegistrar URI (default local; e.g. relay://host:4000)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if output != "json" && output != "yaml" {
		fmt.Fprintf(stderr, "cbweb3b: invalid -o value %q; must be json or yaml\n", output)
		return exitUsage
	}
	if len(files) != 1 {
		fmt.Fprintln(stderr, "cbweb3b: apply requires exactly one -f/--file")
		return exitUsage
	}

	// Partial report on interruption.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rep, err := apply.Apply(ctx, apply.Options{
		ManifestPath: files[0],
		RepoRoot:     repoRoot,
		DataDir:      dataDir,
		OutDir:       outDir,
		HubRPC:       hubRPC,
		HubWS:        hubWS,
		SpokeRPC:     spokeRPC,
		SpokeWS:      spokeWS,
		GatewayURL:   gatewayURL,
		CBAddress:    cbAddr,
		Relay:        relay,
		Format:       output,
		DryRun:       dryRun,
	})

	// Emit whatever report we have (partial on failure/interruption).
	if len(rep.Steps) > 0 || rep.Mode != "" {
		if out, rerr := apply.Render(rep, output); rerr == nil {
			fmt.Fprintln(stdout, string(out))
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "cbweb3b: %v\n", err)
		return exitInvalid
	}
	return exitValid
}

// Report is the `validate` output document.
type Report struct {
	Manifests []ManifestReport   `yaml:"manifests" json:"manifests"`
	SetErrors []manifest.Finding `yaml:"setErrors,omitempty" json:"setErrors,omitempty"`
	Valid     bool               `yaml:"valid" json:"valid"`
}

type ManifestReport struct {
	File     string             `yaml:"file" json:"file"`
	Name     string             `yaml:"name" json:"name"`
	Mode     string             `yaml:"mode" json:"mode"`
	Valid    bool               `yaml:"valid" json:"valid"`
	Errors   []manifest.Finding `yaml:"errors" json:"errors"`
	Warnings []manifest.Finding `yaml:"warnings" json:"warnings"`
}

func buildReport(files []string, manifests []*manifest.ParticipantDeployment) Report {
	rep := Report{Valid: true}
	for i, pd := range manifests {
		res := manifest.Validate(pd)
		mr := ManifestReport{
			File:     files[i],
			Name:     pd.Metadata.Name,
			Mode:     pd.Spec.Mode,
			Valid:    res.Valid(),
			Errors:   res.Errors,
			Warnings: res.Warnings,
		}
		if !mr.Valid {
			rep.Valid = false
		}
		rep.Manifests = append(rep.Manifests, mr)
	}
	if len(manifests) > 1 {
		set := manifest.ValidateSet(manifests)
		if !set.Valid() {
			rep.SetErrors = set.Errors
			rep.Valid = false
		}
	}
	return rep
}

func emit(w io.Writer, format string, rep Report) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	default:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(rep)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `cbweb3b — Scenario B toolkit

Usage:
  cbweb3b validate -f <manifest.yaml> [-f ...] [-o json|yaml]
  cbweb3b apply    -f <manifest.yaml> [--dry-run] [-o json|yaml]
                   [--data-dir <dir>] [--out-dir <dir>] [--repo-root <dir>]
                   [--hub-rpc <url>] [--hub-ws <url>]

validate: parse/validate manifests and report (no effects).
apply:    run the orchestrator for the manifest mode (found-hub | found-spoke);
          --dry-run plans without effects. join is not supported yet (TK-B8).

Exit codes: 0 success, 1 validation/config error or failed step, 2 usage/parse error
`)
}
