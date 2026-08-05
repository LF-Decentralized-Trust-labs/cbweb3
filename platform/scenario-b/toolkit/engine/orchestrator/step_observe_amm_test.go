// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// The NOC backend reads AMM pool status from an api-gateway that must be reachable from
// the NOC's own docker network, so the URL is manifest-provided (spec.noc.ammGatewayURL)
// and reaches the container through the compose env.
func TestObserveComposeEnvCarriesAMMGateway(t *testing.T) {
	c := ObserveConfig{
		Runner:        &exec.FakeRunner{},
		Bundle:        observeBundle(),
		AMMGatewayURL: "http://host.docker.internal:41645",
	}
	c.WithDefaults()

	if joined := strings.Join(c.ComposeEnv(), "\n"); !strings.Contains(joined, "AMM_GATEWAY_URL=http://host.docker.internal:41645") {
		t.Errorf("AMM_GATEWAY_URL missing from compose env:\n%s", joined)
	}
}

// Unset means "leave the backend default": exporting an empty value would still be
// harmless (the backend treats empty as unset), but not exporting keeps the compose
// env free of noise.
func TestObserveComposeEnvOmitsUnsetAMMGateway(t *testing.T) {
	c := ObserveConfig{Runner: &exec.FakeRunner{}, Bundle: observeBundle()}
	c.WithDefaults()

	if joined := strings.Join(c.ComposeEnv(), "\n"); strings.Contains(joined, "AMM_GATEWAY_URL") {
		t.Errorf("AMM_GATEWAY_URL present with no manifest value:\n%s", joined)
	}
}

// The AMM gateway is backend-side configuration; it must not leak into the portal image
// tag, or every gateway change would force a portal rebuild.
func TestObserveAMMGatewayDoesNotChangeThePortalImage(t *testing.T) {
	base := ObserveConfig{Runner: &exec.FakeRunner{}, Bundle: observeBundle()}
	base.WithDefaults()
	withGateway := base
	withGateway.AMMGatewayURL = "http://host.docker.internal:41645"

	if base.portalImage() != withGateway.portalImage() {
		t.Errorf("portal image changed with the AMM gateway: %q vs %q", base.portalImage(), withGateway.portalImage())
	}
}
