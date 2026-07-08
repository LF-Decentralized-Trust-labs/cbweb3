// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
)

// renderConfigJoinStep renders the commercial bank's Paladin config.yaml from the
// bank template, parametrized by bank id and the spoke contract addresses carried
// in the join bundle. Seeded directly into the named volume
// ${spokeID}_${bankID}_paladin_config — no host filesystem involved (mirrors the
// central-bank found-mode equivalent, renderConfigsStep).
type renderConfigJoinStep struct {
	spokeID           string
	bankID            string
	besuRPCPort       int
	besuWSPort        int
	registryAddress   string
	zetoFactoryAddr   string
	penteFactoryAddr  string
	configTemplateDir string
}

func newRenderConfigJoinStep(spokeID, bankID string, besuRPCPort, besuWSPort int, registryAddress, zetoFactoryAddr, penteFactoryAddr, configTemplateDir string) Step {
	return &renderConfigJoinStep{
		spokeID:           spokeID,
		bankID:            bankID,
		besuRPCPort:       besuRPCPort,
		besuWSPort:        besuWSPort,
		registryAddress:   registryAddress,
		zetoFactoryAddr:   zetoFactoryAddr,
		penteFactoryAddr:  penteFactoryAddr,
		configTemplateDir: configTemplateDir,
	}
}

func (s *renderConfigJoinStep) Name() string { return StepRenderConfigJoin }

// paladinConfigVolume mirrors genTLSJoinStep's — config.yaml and tls.{crt,key}
// share the same volume, mounted at /etc/paladin by commercial-bank/paladin-compose.yaml.
func (s *renderConfigJoinStep) paladinConfigVolume() string {
	return s.spokeID + "_" + s.bankID + "_paladin_config"
}

func (s *renderConfigJoinStep) Check(ctx context.Context) (bool, error) {
	return volumeFileExists(ctx, s.paladinConfigVolume(), "config.yaml")
}

func (s *renderConfigJoinStep) Run(ctx context.Context) error {
	tmplPath := filepath.Join(s.configTemplateDir, "bank", "config.yaml.tmpl")
	data := configTemplateData{
		SpokeID:                 s.spokeID,
		NodeName:                s.bankID,
		BesuRPCPort:             s.besuRPCPort,
		BesuWSPort:              s.besuWSPort,
		RegistryContractAddress: s.registryAddress,
		ZetoFactoryAddress:      s.zetoFactoryAddr,
		PenteFactoryAddress:     s.penteFactoryAddr,
		// Unique per-bank base-ledger submitter key so co-located banks on the spoke's Besu do
		// not collide on nonce (which wedges Pente deploys). See fundedOperatorKey.
		FundedOperatorKey: fundedOperatorKey(s.spokeID, s.bankID),
	}
	rendered, err := renderTemplateToBytes(tmplPath, data)
	if err != nil {
		return fmt.Errorf("render bank config: %w", err)
	}
	if err := writeVolumeFile(ctx, s.paladinConfigVolume(), "config.yaml", rendered, "0644"); err != nil {
		return fmt.Errorf("write bank config to volume %s: %w", s.paladinConfigVolume(), err)
	}
	return nil
}
