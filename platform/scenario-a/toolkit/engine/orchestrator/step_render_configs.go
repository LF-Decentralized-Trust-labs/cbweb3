// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

type renderConfigsStep struct {
	spokeID           string
	dataDir           string
	besuRPCPort       int
	besuWSPort        int
	configTemplateDir string
}

// configTemplateData is the data injected into every Paladin config template.
type configTemplateData struct {
	SpokeID                 string
	NodeName                string
	BesuRPCPort             int
	BesuWSPort              int
	RegistryContractAddress string
	ZetoFactoryAddress      string
	PenteFactoryAddress     string
	// FundedOperatorKey is the hex (no 0x) secp256k1 private key for this node's
	// `funded_operator` base-ledger submitter. It MUST be unique per node — see fundedOperatorKey.
	FundedOperatorKey string
}

// fundedOperatorKey derives a deterministic, per-node secp256k1 private key (hex, no 0x) for a
// Paladin node's `funded_operator` base-ledger submitter.
//
// Every Paladin node on a spoke shares one Besu chain, so two nodes submitting public transactions
// from the SAME account collide on nonce ("Nonce too low"), which silently wedges the Pente
// transaction pipeline (a large deploy such as FXAgreement never receives a receipt and everything
// queues behind it). Previously all commercial banks used the same hardcoded dev key
// (0xf17f5215…), which manifested on the second spoke once two banks submitted concurrently.
//
// The derived account needs no genesis prefunding: the spoke genesis sets zeroBaseFee, so gas is
// free. Derivation is deterministic (idempotent across redeploys) and effectively always a valid
// secp256k1 scalar (a sha256 digest is < the curve order with overwhelming probability).
func fundedOperatorKey(spokeID, nodeName string) string {
	h := sha256.Sum256([]byte("cbweb3/funded_operator/" + spokeID + "/" + nodeName))
	return hex.EncodeToString(h[:])
}

func newRenderConfigsStep(spokeID, dataDir string, besuRPCPort, besuWSPort int, configTemplateDir string) Step {
	return &renderConfigsStep{
		spokeID:           spokeID,
		dataDir:           dataDir,
		besuRPCPort:       besuRPCPort,
		besuWSPort:        besuWSPort,
		configTemplateDir: configTemplateDir,
	}
}

func (s *renderConfigsStep) Name() string { return StepRenderConfigs }

// paladinConfigVolume mirrors genTLSStep's — config.yaml and tls.{crt,key} share
// the same volume, mounted at /etc/paladin by paladin-compose.yaml.
func (s *renderConfigsStep) paladinConfigVolume() string { return s.spokeID + "_cb_paladin_config" }

func (s *renderConfigsStep) Check(ctx context.Context) (bool, error) {
	return volumeFileExists(ctx, s.paladinConfigVolume(), "config.yaml")
}

func (s *renderConfigsStep) Run(ctx context.Context) error {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return fmt.Errorf("read deployed-addrs: %w", err)
	}

	// found is CB-only: render only the central-bank node config. Commercial-bank
	// Paladin configs are rendered dynamically at join time (feature 033 US2).
	nodes := []struct {
		name        string
		tmplSubPath string // relative to configTemplateDir
	}{
		{"central-bank", "central-bank/config.yaml.tmpl"},
	}

	for _, node := range nodes {
		tmplPath := filepath.Join(s.configTemplateDir, node.tmplSubPath)
		data := configTemplateData{
			SpokeID:                 s.spokeID,
			NodeName:                node.name,
			BesuRPCPort:             s.besuRPCPort,
			BesuWSPort:              s.besuWSPort,
			RegistryContractAddress: addrs.RegistryContractAddress,
			ZetoFactoryAddress:      addrs.ZetoFactoryAddress,
			PenteFactoryAddress:     addrs.PenteFactoryAddress,
		}

		rendered, err := renderTemplateToBytes(tmplPath, data)
		if err != nil {
			return fmt.Errorf("render config for %s: %w", node.name, err)
		}
		if err := writeVolumeFile(ctx, s.paladinConfigVolume(), "config.yaml", rendered, "0644"); err != nil {
			return fmt.Errorf("write config for %s to volume %s: %w", node.name, s.paladinConfigVolume(), err)
		}
	}
	return nil
}

func renderTemplate(tmplPath, outPath string, data any) error {
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return fmt.Errorf("parse template %s: %w", tmplPath, err)
	}

	out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create output file %s: %w", outPath, err)
	}
	defer out.Close()

	if err := tmpl.Execute(out, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	return nil
}

// renderTemplateToBytes is renderTemplate's volume-backed counterpart: it
// executes the template into memory instead of a host file, for callers that
// then seed the result into a named Docker volume (writeVolumeFile).
func renderTemplateToBytes(tmplPath string, data any) ([]byte, error) {
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", tmplPath, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}
