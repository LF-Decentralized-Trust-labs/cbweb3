// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"fmt"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// ResolveJoinDeps constructs orchestrator.JoinDeps from the manifest and a
// LocalProfile for mode:join. Returns an error for an unsupported environment
// or an invalid keyProvider URI.
func ResolveJoinDeps(m *manifest.Manifest, profile LocalProfile) (orchestrator.JoinDeps, error) {
	if m.Spec.Environment != "local" {
		return orchestrator.JoinDeps{}, fmt.Errorf("environment %s is not yet supported — only local is available", m.Spec.Environment)
	}

	kp, err := keyprovider.New(m.Spec.KeyProvider)
	if err != nil {
		return orchestrator.JoinDeps{}, fmt.Errorf("keyProvider URI error: %w", err)
	}

	return orchestrator.JoinDeps{
		KeyProvider:         kp,
		BankCode:            m.BankCode(),
		Institution:        m.Metadata.Name,
		BesuRPCURL:          profile.BesuRPCURL,
		ComposeTemplatePath: profile.CommercialBankComposePath,
		BackendComposePath:  profile.BackendComposePath,
	}, nil
}
