// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"fmt"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/certsource"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// ResolveDeps constructs orchestrator.Deps from the manifest and a LocalProfile.
// Returns an error for unsupported environment, invalid keyProvider/certSource URIs,
// or a missing relay endpoint when required.
func ResolveDeps(m *manifest.Manifest, profile LocalProfile) (orchestrator.Deps, error) {
	if m.Spec.Environment != "local" {
		return orchestrator.Deps{}, fmt.Errorf("environment %s is not yet supported in TK-7 — only local is available", m.Spec.Environment)
	}

	kp, err := keyprovider.New(m.Spec.KeyProvider)
	if err != nil {
		return orchestrator.Deps{}, fmt.Errorf("keyProvider URI error: %w", err)
	}

	cs, err := certsource.New(m.Spec.CertSource)
	if err != nil {
		return orchestrator.Deps{}, fmt.Errorf("certSource URI error: %w", err)
	}

	var rr orchestrator.RelayRegistrar
	if m.Spec.Relay != nil && m.Spec.Relay.Endpoint != "" {
		rr = orchestrator.NewHTTPRelayRegistrar(m.Spec.Relay.Endpoint)
	} else {
		rr = orchestrator.NoOpRelayRegistrar{}
	}

	return orchestrator.Deps{
		KeyProvider:              kp,
		CertSource:               cs,
		RelayRegistrar:           rr,
		ScriptsDir:               profile.ScriptsDir,
		ComposeTemplatePath:      profile.ComposeTemplatePath,
		CentralBankComposePath:   profile.CentralBankComposePath,
		BesuImage:                profile.BesuImage,
		PaladinConfigTemplateDir: profile.PaladinConfigDir,
		BesuRPCURL:               profile.BesuRPCURL,
		PaladinCBURL:             profile.PaladinCBURL,
	}, nil
}
