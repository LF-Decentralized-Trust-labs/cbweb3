// SPDX-License-Identifier: Apache-2.0

// Package addrs provides shared parsing of deployed contract addresses
// from the .deployed-addrs.env file written by the provisioning engine.
package addrs

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// DeployedAddrs holds the contract addresses written by the deploy Go test scripts.
// Read from <SPOKE_DATA_DIR>/.deployed-addrs.env (key=value format, one per line).
type DeployedAddrs struct {
	RegistryContractAddress string // REGISTRY_CONTRACT_ADDRESS
	ZetoFactoryAddress      string // ZETO_FACTORY_ADDRESS
	PenteFactoryAddress     string // PENTE_FACTORY_ADDRESS
	ZetoTokenAddress        string // ZETO_TOKEN_ADDRESS (written after step 6)
	PenteContextGroupID     string // PENTE_CONTEXT_GROUP_ID (written after step 7)
	PenteContextAddress     string // PENTE_CONTEXT_ADDRESS (written after step 7)
	FXAgreementDeployedAt   string // FX_AGREEMENT_DEPLOYED_AT (written after step 8)
}

// ParseDeployedAddrs reads a KEY=VALUE env file from path and populates DeployedAddrs.
// Returns an empty DeployedAddrs (not an error) when the file does not exist.
func ParseDeployedAddrs(path string) (DeployedAddrs, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DeployedAddrs{}, nil
		}
		return DeployedAddrs{}, fmt.Errorf("open deployed-addrs: %w", err)
	}
	defer f.Close()

	kv := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		kv[key] = val
	}
	if err := scanner.Err(); err != nil {
		return DeployedAddrs{}, fmt.Errorf("read deployed-addrs: %w", err)
	}

	return DeployedAddrs{
		RegistryContractAddress: kv["REGISTRY_CONTRACT_ADDRESS"],
		ZetoFactoryAddress:      kv["ZETO_FACTORY_ADDRESS"],
		PenteFactoryAddress:     kv["PENTE_FACTORY_ADDRESS"],
		ZetoTokenAddress:        kv["ZETO_TOKEN_ADDRESS"],
		PenteContextGroupID:     kv["PENTE_CONTEXT_GROUP_ID"],
		PenteContextAddress:     kv["PENTE_CONTEXT_ADDRESS"],
		FXAgreementDeployedAt:   kv["FX_AGREEMENT_DEPLOYED_AT"],
	}, nil
}
