// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"
)

// deployFXAJoinStep stands up the on-chain FX context inside the bilateral Pente group
// (mode:join US3): it deploys an IdentityRegistry INTO the group, registers the CB
// (CENTRAL_BANK) and the joining bank (COMMERCIAL_BANK) as Verified, and deploys FXAgreement
// wired to that in-group registry. The bank's Paladin identity is the deployer/admin.
//
// The in-group registry is required: an FXAgreement pointing at a base-ledger registry address
// reverts canTransact with empty 0x, because the base-ledger contract has no code in the Pente
// private EVM (see PLAN.md Phase 1a).
type deployFXAJoinStep struct {
	spokeID          string
	bankID           string
	dataDir          string
	paladinURL       string
	artifactPath     string // FXAgreement Foundry artifact (…/out/FXAgreement.sol/FXAgreement.json)
	identityRegistry string // deprecated: base-ledger whitelist from the bundle; superseded by the in-group registry
	timeout          time.Duration
	logw             io.Writer // progress logging (nil-safe)
}

func newDeployFXAJoinStep(spokeID, bankID, dataDir, paladinURL, artifactPath, identityRegistry string, timeout time.Duration, logw io.Writer) Step {
	return &deployFXAJoinStep{
		spokeID:          spokeID,
		bankID:           bankID,
		dataDir:          dataDir,
		paladinURL:       paladinURL,
		artifactPath:     artifactPath,
		identityRegistry: identityRegistry,
		timeout:          timeout,
		logw:             logw,
	}
}

func (s *deployFXAJoinStep) Name() string { return StepDeployFXAJoin }

func (s *deployFXAJoinStep) Check(_ context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	return addrs.FXAgreementDeployedAt != "", nil
}

func (s *deployFXAJoinStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	envPath := filepath.Join(s.dataDir, ".deployed-addrs.env")
	addrs, err := parseDeployedAddrs(envPath)
	if err != nil {
		return fmt.Errorf("read deployed-addrs: %w", err)
	}
	if addrs.PenteContextGroupID == "" {
		return fmt.Errorf("PENTE_CONTEXT_GROUP_ID missing — create-pente-context must run first")
	}

	// Wait for the group to be ready to accept transactions before deploying.
	if err := waitPenteGroupReady(ctx, s.paladinURL, addrs.PenteContextGroupID, s.timeout); err != nil {
		return err
	}

	deployer := paladinIdentity(bankNodeName(s.spokeID, s.bankID))
	members := []penteMember{
		{Identity: paladinIdentity(cbNodeName(s.spokeID)), Name: cbNodeName(s.spokeID), Role: roleCentralBank},
		{Identity: deployer, Name: bankNodeName(s.spokeID, s.bankID), Role: roleCommercialBank},
	}
	progress := func(msg string) { logDetail(s.logw, s.spokeID, StepDeployFXAJoin, msg) }
	registryAddr, fxaAddr, err := setupBilateralFXAContext(
		ctx, s.paladinURL, addrs.PenteContextGroupID, deployer,
		identityRegistryArtifact(s.artifactPath), s.artifactPath, members, progress)
	if err != nil {
		return fmt.Errorf("set up in-group FX context: %w", err)
	}

	if registryAddr != "" {
		if err := addrsAppend(envPath, "FX_PENTE_REGISTRY_ADDRESS", registryAddr); err != nil {
			return fmt.Errorf("persist FX_PENTE_REGISTRY_ADDRESS: %w", err)
		}
	}
	if fxaAddr != "" {
		if err := addrsAppend(envPath, "FX_AGREEMENT_ADDRESS", fxaAddr); err != nil {
			return fmt.Errorf("persist FX_AGREEMENT_ADDRESS: %w", err)
		}
	}
	// Mark completion (used by Check) even if the address resolves later.
	marker := fxaAddr
	if marker == "" {
		marker = "deployed"
	}
	if err := addrsAppend(envPath, "FX_AGREEMENT_DEPLOYED_AT", marker); err != nil {
		return fmt.Errorf("persist FX_AGREEMENT_DEPLOYED_AT: %w", err)
	}

	// A6: write the bilateral FX context (group + in-group FXAgreement address) into the
	// named volume mounted into the bank's payment-orchestrator at
	// /workspace/backend/config/fx-contexts (no host filesystem involved — deviation
	// from the original SPOKE_DATA_DIR bind-mount design; see the addendum in
	// specs/032-commercial-bank-join/research.md). The backend reads it at propose
	// time to resolve the on-chain target — the address is unknown at env-render time
	// (deploy-fxa runs after the backend starts). See PLAN.md "A6".
	if fxaAddr != "" {
		if err := writeBankFXContext(ctx, s.fxContextsVolume(), fxContextEntry{
			SpokeID:         s.spokeID,
			GroupID:         addrs.PenteContextGroupID,
			ContractAddress: fxaAddr,
			BankIdentity:    deployer,
			CBIdentity:      paladinIdentity(cbNodeName(s.spokeID)),
		}); err != nil {
			return fmt.Errorf("write fx context file: %w", err)
		}
	}
	return nil
}

// fxContextsVolume is mounted at /workspace/backend/config/fx-contexts by
// entity-backend/backend-compose.yaml, read-only-in-practice by payment-orchestrator
// (FX_CONTEXTS_FILE). Naming mirrors the Paladin volumes: ${SPOKE_ID}_${BANK_ID}_<artifact>.
func (s *deployFXAJoinStep) fxContextsVolume() string {
	return s.spokeID + "_" + s.bankID + "_fx_contexts"
}

// fxContextEntry mirrors the backend's fxContext JSON (payment-orchestrator reads it).
type fxContextEntry struct {
	SpokeID         string `json:"spoke_id"`
	GroupID         string `json:"group_id"`
	ContractAddress string `json:"contract_address"`
	BankIdentity    string `json:"bank_identity"`
	CBIdentity      string `json:"cb_identity"`
}

// writeBankFXContext writes the bank's single FX context as a one-element JSON
// array into fx-contexts.json in the named volume — no host filesystem involved.
func writeBankFXContext(ctx context.Context, volume string, entry fxContextEntry) error {
	data, err := json.MarshalIndent([]fxContextEntry{entry}, "", "  ")
	if err != nil {
		return err
	}
	return writeVolumeFile(ctx, volume, "fx-contexts.json", data, "0644")
}

// identityRegistryArtifact derives the IdentityRegistry Foundry artifact path from the
// FXAgreement artifact path (both live under the same contracts/out directory).
func identityRegistryArtifact(fxaArtifactPath string) string {
	outDir := filepath.Dir(filepath.Dir(fxaArtifactPath)) // …/out/FXAgreement.sol/x.json -> …/out
	return filepath.Join(outDir, "IdentityRegistry.sol", "IdentityRegistry.json")
}
