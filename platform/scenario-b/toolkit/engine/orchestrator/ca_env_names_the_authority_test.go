// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"path"
	"strings"
	"testing"
)

// TestCAEnvNamesTheBootstrapCAPair pins the one line whose reversal reopens the outage.
//
// CA_CERT_FILE/CA_KEY_FILE tell the compliance service what to ISSUE with, so they must
// name a pair with a single producer. They used to name central-bank.crt/.key, which has
// two, disagreeing about what the names mean: gen-tls (genCBCA) writes a self-signed CA
// there, and the compliance bootstrap's ensureCert writes this entity's participant leaf
// there. In the state that shipped, gen-tls's CA certificate was left beside a participant
// key the bootstrap had written over it, and every credential issuance failed weeks later
// with an x509 error that named neither file.
//
// {bankCode}-ca.* is created by the compliance bootstrap alone, as a matched pair. Nothing
// at runtime relates that fact to these two variables, so without this test the line can be
// reverted with every suite still green.
func TestCAEnvNamesTheBootstrapCAPair(t *testing.T) {
	cfg := SpokeConfig{
		ContainerPrefix: "sc-b-cbweb3-central-bank-chile",
		Entity:          "central-bank",
		NetPrefix:       "central-bank-chile",
		VolumePrefix:    "central-bank-chile",
		RPCPort:         8845,
	}
	env := envMap(cfg.ComposeEnv())

	certFile, ok := env["CA_CERT_FILE"]
	if !ok {
		t.Fatal("CA_CERT_FILE is not exported; the compliance service cannot issue at all")
	}
	keyFile, ok := env["CA_KEY_FILE"]
	if !ok {
		t.Fatal("CA_KEY_FILE is not exported")
	}

	certBase := strings.TrimSuffix(path.Base(certFile), ".crt")
	keyBase := strings.TrimSuffix(path.Base(keyFile), ".key")

	if !strings.HasSuffix(certBase, "-ca") {
		t.Errorf("CA_CERT_FILE is %q, which is not the {bankCode}-ca pair.\n"+
			"\tThe issuer must be the pair the compliance bootstrap creates on its own;\n"+
			"\tcentral-bank.crt/.key is written by BOTH gen-tls and that bootstrap, with\n"+
			"\tconflicting intent, and the two have already diverged once in production.",
			certFile)
	}
	if certBase != keyBase {
		t.Errorf("CA_CERT_FILE (%q) and CA_KEY_FILE (%q) do not name the same pair", certFile, keyFile)
	}
	if path.Dir(certFile) != path.Dir(keyFile) {
		t.Errorf("the CA cert and key are in different directories: %q vs %q", certFile, keyFile)
	}
}
