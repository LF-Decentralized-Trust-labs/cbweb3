// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// renderConfigJoinStep renders the commercial bank's Paladin config.yaml from the
// bank template, parametrized by bank id and the spoke contract addresses carried
// in the join bundle. Writes to <dataDir>/paladin/<bankId>/config.yaml.
type renderConfigJoinStep struct {
	spokeID           string
	bankID            string
	dataDir           string
	besuRPCPort       int
	besuWSPort        int
	registryAddress   string
	zetoFactoryAddr   string
	penteFactoryAddr  string
	configTemplateDir string
}

func newRenderConfigJoinStep(spokeID, bankID, dataDir string, besuRPCPort, besuWSPort int, registryAddress, zetoFactoryAddr, penteFactoryAddr, configTemplateDir string) Step {
	return &renderConfigJoinStep{
		spokeID:           spokeID,
		bankID:            bankID,
		dataDir:           dataDir,
		besuRPCPort:       besuRPCPort,
		besuWSPort:        besuWSPort,
		registryAddress:   registryAddress,
		zetoFactoryAddr:   zetoFactoryAddr,
		penteFactoryAddr:  penteFactoryAddr,
		configTemplateDir: configTemplateDir,
	}
}

func (s *renderConfigJoinStep) Name() string { return StepRenderConfigJoin }

func (s *renderConfigJoinStep) Check(_ context.Context) (bool, error) {
	_, err := os.Stat(filepath.Join(s.dataDir, "paladin", s.bankID, "config.yaml"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *renderConfigJoinStep) Run(_ context.Context) error {
	tmplPath := filepath.Join(s.configTemplateDir, "bank", "config.yaml.tmpl")
	data := configTemplateData{
		SpokeID:                 s.spokeID,
		NodeName:                s.bankID,
		BesuRPCPort:             s.besuRPCPort,
		BesuWSPort:              s.besuWSPort,
		RegistryContractAddress: s.registryAddress,
		ZetoFactoryAddress:      s.zetoFactoryAddr,
		PenteFactoryAddress:     s.penteFactoryAddr,
	}
	outDir := filepath.Join(s.dataDir, "paladin", s.bankID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	outPath := filepath.Join(outDir, "config.yaml")
	if err := renderTemplate(tmplPath, outPath, data); err != nil {
		return fmt.Errorf("render bank config: %w", err)
	}
	return nil
}
