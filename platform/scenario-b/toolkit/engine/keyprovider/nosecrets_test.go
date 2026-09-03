// SPDX-License-Identifier: Apache-2.0

package keyprovider

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
)

// SC-006 / FR-012: no public output of the KeyProvider contains private key
// material. The ONLY path that yields a private key is the local-only
// LocalKeyExporter; the production stub does not implement it.
func TestNoSecretsInPublicOutputs(t *testing.T) {
	l := newLocal("")
	ctx := context.Background()

	pub, err := l.GenerateKey(ctx, "bank-a")
	if err != nil {
		t.Fatal(err)
	}
	privHex, err := l.ExportPrivateKeyHex("bank-a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hex.EncodeToString(pub), privHex) {
		t.Fatal("private key material leaked into the public key output")
	}
}

func TestProdStubExposesNoExporter(t *testing.T) {
	kp, err := New("kms://prod")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := kp.(LocalKeyExporter); ok {
		t.Fatal("production provider must not expose private keys")
	}
}
