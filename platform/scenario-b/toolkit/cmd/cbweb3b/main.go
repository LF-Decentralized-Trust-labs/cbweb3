// SPDX-License-Identifier: Apache-2.0

// Command cbweb3b is the Scenario B toolkit CLI. In this phase (TK-B1) it only
// parses, validates, and reports on ParticipantDeployment manifests — it does
// not execute any provisioning steps.
//
// Usage:
//
//	cbweb3b validate -f <manifest.yaml> [-f <manifest.yaml> ...] [-o json|yaml]
//	cbweb3b apply    -f <manifest.yaml> [-f <manifest.yaml> ...] --dry-run [-o json|yaml]
//
// validate and apply --dry-run are equivalent in this phase. apply without
// --dry-run is not implemented yet and exits non-zero.
//
// Exit codes: 0 = all manifests valid (warnings allowed); 1 = one or more
// validation errors; 2 = usage/parse error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/manifest"
)

const (
	exitValid   = 0
	exitInvalid = 1
	exitUsage   = 2
)

// fileList is a repeatable -f/--file flag.
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

	cmd := args[0]
	switch cmd {
	case "validate", "apply":
	case "-h", "--help", "help":
		usage(stdout)
		return exitValid
	default:
		fmt.Fprintf(stderr, "cbweb3b: unknown command %q\n", cmd)
		usage(stderr)
		return exitUsage
	}

	var files fileList
	var output string
	var dryRun bool

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Var(&files, "f", "manifest file (repeatable)")
	fs.Var(&files, "file", "manifest file (repeatable)")
	fs.StringVar(&output, "o", "yaml", "output format: json|yaml")
	fs.StringVar(&output, "output", "yaml", "output format: json|yaml")
	fs.BoolVar(&dryRun, "dry-run", false, "validate only (apply): no steps are executed")

	if err := fs.Parse(args[1:]); err != nil {
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
	if cmd == "apply" && !dryRun {
		fmt.Fprintln(stderr, "cbweb3b: apply without --dry-run is not implemented in TK-B1")
		return exitInvalid
	}

	// Parse (usage/parse errors → exit 2).
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

// Report is the CLI output document.
type Report struct {
	Manifests []ManifestReport   `yaml:"manifests" json:"manifests"`
	SetErrors []manifest.Finding `yaml:"setErrors,omitempty" json:"setErrors,omitempty"`
	Valid     bool               `yaml:"valid" json:"valid"`
}

// ManifestReport is the per-manifest validation report.
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

	// Set-level collision checks only make sense for 2+ manifests.
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
	fmt.Fprint(w, `cbweb3b — Scenario B toolkit (TK-B1: validate only)

Usage:
  cbweb3b validate -f <manifest.yaml> [-f <manifest.yaml> ...] [-o json|yaml]
  cbweb3b apply    -f <manifest.yaml> [-f <manifest.yaml> ...] --dry-run [-o json|yaml]

Flags:
  -f, --file     manifest file (repeatable; multiple files enable collision checks)
  -o, --output   output format: json|yaml (default yaml)
      --dry-run  apply: validate only, no steps executed (required in this phase)

Exit codes: 0 valid, 1 validation errors, 2 usage/parse error
`)
}
