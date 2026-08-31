// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"bytes"
	"context"
	"testing"
)

// SC-006 / FR-008 / FR-012: neither the issued leaf nor the trust anchor
// contains private key material. The CA private key never appears in any
// serializable output.
func TestNoPrivateKeyInSerializableOutputs(t *testing.T) {
	cs := newLocal()
	ctx := context.Background()
	csr := makeCSR(t, p256Key(t), []string{roleCommercialBank})

	leafPEM, err := cs.IssueLeafCert(ctx, csr, "spoke-a")
	if err != nil {
		t.Fatal(err)
	}
	caPEM, err := cs.GetTrustAnchor(ctx, "spoke-a")
	if err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string][]byte{"leaf": leafPEM, "anchor": caPEM} {
		if bytes.Contains(out, []byte("PRIVATE KEY")) {
			t.Fatalf("private key material found in %s output", name)
		}
	}
}
