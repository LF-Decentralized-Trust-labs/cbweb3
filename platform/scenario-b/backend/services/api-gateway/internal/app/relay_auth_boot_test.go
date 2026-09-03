// SPDX-License-Identifier: Apache-2.0

package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
)

// The boot guard exists to turn "enforcement on, nothing to verify against" into one refusal instead
// of a wave of 401s. Judging the FILE registry made it refuse the deployment it was written for: a
// central bank's peers are the banks it onboarded, pinned from the participants table, and its PKI
// dir holds no peer certificate at all.

func leafCertPEM(t *testing.T, cn string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func enforcingConfig() middleware.RelayAuthConfig {
	return middleware.RelayAuthConfig{
		Registry:         relayauth.NewStore(relayauth.NewRegistry()),
		LegacySecret:     "legacy",
		RequireSignature: true,
	}
}

// A CB whose only peers are onboarded banks must start. This is the case the previous guard refused.
func TestValidateRelayAuthWithPins_AcceptsParticipantOnlyRegistry(t *testing.T) {
	pins := []relayauth.ParticipantPin{{ID: "bank-itau", CertPEM: leafCertPEM(t, "bank-itau"), Active: true}}

	// Empty PKI dir on purpose: this is what a central bank's really looks like.
	if err := validateRelayAuthWithPins(enforcingConfig(), t.TempDir(), pins); err != nil {
		t.Fatalf("a CB with peers pinned from the participants table must start: %v", err)
	}
}

// Nothing pinned anywhere still refuses: every internal request would 401, including a correct one.
func TestValidateRelayAuthWithPins_RefusesWhenNeitherSourceHasAPeer(t *testing.T) {
	err := validateRelayAuthWithPins(enforcingConfig(), t.TempDir(), nil)
	if err == nil {
		t.Fatal("enforcement with no pinned key must refuse to start")
	}
	// The message has to name both sources, or an operator fixes the wrong one.
	for _, want := range []string{"RELAY_REQUIRE_SIGNATURE", "PKI_DIR", "onboard"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must mention %q to be actionable, got: %v", want, err)
		}
	}
}

// An INACTIVE participant is not a peer. Deactivating a bank is what revokes its ability to
// authenticate, so a registry holding only deactivated banks is empty for this purpose.
func TestValidateRelayAuthWithPins_RefusesWhenEveryParticipantIsInactive(t *testing.T) {
	pins := []relayauth.ParticipantPin{{ID: "bank-itau", CertPEM: leafCertPEM(t, "bank-itau"), Active: false}}

	if err := validateRelayAuthWithPins(enforcingConfig(), t.TempDir(), pins); err == nil {
		t.Fatal("a deactivated participant must not count as something to verify against")
	}
}

// Without enforcement there is nothing to judge, and a gateway with no peers is a normal state.
func TestValidateRelayAuthWithPins_MigratingStateStarts(t *testing.T) {
	cfg := middleware.RelayAuthConfig{
		Registry:     relayauth.NewStore(relayauth.NewRegistry()),
		LegacySecret: "legacy",
	}
	if err := validateRelayAuthWithPins(cfg, t.TempDir(), nil); err != nil {
		t.Fatalf("the pre-cutover state must keep starting: %v", err)
	}
}
