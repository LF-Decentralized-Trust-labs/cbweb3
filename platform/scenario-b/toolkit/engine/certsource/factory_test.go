// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"context"
	"testing"
)

// SC-007: self-signed[://...] -> local; ca:// -> prod stub; other -> error.
func TestFactorySelfSigned(t *testing.T) {
	cs, err := New("self-signed")
	if err != nil || cs == nil {
		t.Fatalf("self-signed: %v", err)
	}
	if _, err := New("self-signed://spoke-a"); err != nil {
		t.Fatalf("self-signed://: %v", err)
	}
}

func TestFactoryProdStub(t *testing.T) {
	cs, err := New("ca://prod-ca.example")
	if err != nil {
		t.Fatalf("ca://: %v", err)
	}
	if _, err := cs.GetTrustAnchor(context.Background(), "spoke-a"); err != ErrNotImplemented {
		t.Fatalf("got %v, want ErrNotImplemented", err)
	}
	if _, err := cs.IssueLeafCert(context.Background(), nil, "spoke-a"); err != ErrNotImplemented {
		t.Fatalf("got %v, want ErrNotImplemented", err)
	}
}

func TestFactoryUnsupportedURI(t *testing.T) {
	if _, err := New("http://foo"); err != ErrUnsupportedURI {
		t.Fatalf("got %v, want ErrUnsupportedURI", err)
	}
}
