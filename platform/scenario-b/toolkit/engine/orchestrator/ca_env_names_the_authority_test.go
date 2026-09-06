// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"path"
	"strings"
	"testing"
)

// TestCAEnvNamesTheAuthorityNotTheLeaf pins the one line whose reversal reopens the outage.
//
// CA_CERT_FILE/CA_KEY_FILE tell the compliance service what to ISSUE with. They used to
// name central-bank.crt/.key, which is this entity's own participant leaf and one of the
// files the gen-tls step writes — so a second producer of those filenames was enough to
// leave the two holding different keys, and every credential issuance failed weeks later
// with an x509 error that named neither file.
//
// The authority is {bankCode}-ca.*, which the compliance bootstrap creates as a matched
// pair. Nothing at runtime relates that fact to these two variables, so without this test
// the line can be reverted with every suite still green.
func TestCAEnvNamesTheAuthorityNotTheLeaf(t *testing.T) {
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
		t.Errorf("CA_CERT_FILE is %q, which is the participant LEAF, not the authority.\n"+
			"\tThe issuer must be the {bankCode}-ca pair the compliance bootstrap creates together;\n"+
			"\tthe leaf shares its filenames with the toolkit's gen-tls step and the two have already diverged once.",
			certFile)
	}
	if certBase != keyBase {
		t.Errorf("CA_CERT_FILE (%q) and CA_KEY_FILE (%q) do not name the same pair", certFile, keyFile)
	}
	if path.Dir(certFile) != path.Dir(keyFile) {
		t.Errorf("the CA cert and key are in different directories: %q vs %q", certFile, keyFile)
	}
}
