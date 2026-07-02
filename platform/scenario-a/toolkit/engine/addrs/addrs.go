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
	// ParticipantRegistryAddress is the IdentityRegistry.sol (participant whitelist)
	// deployed by onboard-registry — distinct from RegistryContractAddress, which is
	// the Paladin node registry. PARTICIPANT_REGISTRY_ADDRESS.
	ParticipantRegistryAddress string
	// FiatTokenAddress is the FiatCentralBankMoney (fCeBM) ERC-20 deployed on the
	// spoke's Besu chain by deploy-fiat-token. FIAT_TOKEN_ADDRESS.
	FiatTokenAddress string
	// HTLCAddress is the HashTimeLockedContract deployed on the spoke's Besu chain by
	// deploy-htlc (Scenario A domestic settlement leg). HTLC_ADDRESS.
	HTLCAddress string
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
		RegistryContractAddress:    kv["REGISTRY_CONTRACT_ADDRESS"],
		ZetoFactoryAddress:         kv["ZETO_FACTORY_ADDRESS"],
		PenteFactoryAddress:        kv["PENTE_FACTORY_ADDRESS"],
		ZetoTokenAddress:           kv["ZETO_TOKEN_ADDRESS"],
		PenteContextGroupID:        kv["PENTE_CONTEXT_GROUP_ID"],
		PenteContextAddress:        kv["PENTE_CONTEXT_ADDRESS"],
		FXAgreementDeployedAt:      kv["FX_AGREEMENT_DEPLOYED_AT"],
		ParticipantRegistryAddress: kv["PARTICIPANT_REGISTRY_ADDRESS"],
		FiatTokenAddress:           kv["FIAT_TOKEN_ADDRESS"],
		HTLCAddress:                kv["HTLC_ADDRESS"],
	}, nil
}

// AppendAddr appends a single KEY=value line to the deployed-addrs env file at
// path, creating it if necessary. Used to record addresses produced natively by
// the engine (e.g. PARTICIPANT_REGISTRY_ADDRESS) without disturbing existing lines.
func AppendAddr(path, key, value string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open deployed-addrs for append: %w", err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s=%s\n", key, value); err != nil {
		return fmt.Errorf("append %s: %w", key, err)
	}
	return nil
}
