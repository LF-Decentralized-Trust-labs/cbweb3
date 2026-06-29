// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
)

func TestLocalizeBundleEndpoints(t *testing.T) {
	b := &bundle.JoinBundle{}
	b.Spec.Validators = []bundle.ValidatorSpec{
		{Address: "0xabc", RPCURL: "http://cbweb3-spoke-brl-besu.central-bank-brazil:8645"},
	}
	b.Spec.Bootnode.Enode = "enode://deadbeef@cbweb3-spoke-brl-besu.central-bank-brazil:31303"
	b.Spec.CBEndpoint = "http://cbweb3-api-gateway.central-bank-brazil:8080/api/v1/onboarding/credential-request"

	ep, err := localizeBundleEndpoints(b)
	if err != nil {
		t.Fatalf("localizeBundleEndpoints: %v", err)
	}

	// Validator RPC: host-run toolkit reaches the CB at localhost on the published port.
	if got, want := ep.validators[0].RPCURL, "http://localhost:8645"; got != want {
		t.Errorf("validator RPCURL = %q; want %q", got, want)
	}
	// Enode: keep the advertised host (resolves on the shared network), internal P2P port.
	if got, want := ep.bootnodeEnode, "enode://deadbeef@cbweb3-spoke-brl-besu.central-bank-brazil:30303"; got != want {
		t.Errorf("bootnode enode = %q; want %q", got, want)
	}
	// CB cert endpoint: localhost on the api-gateway published port (entityPorts(8645)=18645).
	if got, want := ep.cbCertEndpoint, "http://localhost:18645/api/v1/onboarding/credential-request"; got != want {
		t.Errorf("cbCertEndpoint = %q; want %q", got, want)
	}
	// Bank backend (a container) reaches the CB api-gateway via the host gateway.
	if got, want := ep.cbAPIBaseForBank, "http://host.docker.internal:18645"; got != want {
		t.Errorf("cbAPIBaseForBank = %q; want %q", got, want)
	}
}

func TestSetEnodePort(t *testing.T) {
	got, err := setEnodePort("enode://id@host.example:31303", 30303)
	if err != nil {
		t.Fatal(err)
	}
	if want := "enode://id@host.example:30303"; got != want {
		t.Errorf("setEnodePort = %q; want %q", got, want)
	}
}

func TestRewriteURLHost(t *testing.T) {
	// Keep port when newPort == 0.
	if got, _ := rewriteURLHost("http://container:8645/x", "localhost", 0); got != "http://localhost:8645/x" {
		t.Errorf("rewriteURLHost keep-port = %q", got)
	}
	// Replace port when newPort > 0.
	if got, _ := rewriteURLHost("http://container:8080/x", "localhost", 18645); got != "http://localhost:18645/x" {
		t.Errorf("rewriteURLHost replace-port = %q", got)
	}
}
