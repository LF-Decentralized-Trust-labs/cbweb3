// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/dockervolume"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// Consequences of moving the NOC login off the browser and onto the backend.

// TestContainerReachableURL: spec.noc.keycloakURL was written for a browser, where
// "localhost" is the host. The grant now runs in a container, where it is not.
func TestContainerReachableURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://localhost:40645", "http://host.docker.internal:40645"},
		{"http://127.0.0.1:40645", "http://host.docker.internal:40645"},
		// Already routable: left exactly as the operator wrote it.
		{"https://kc.example/auth", "https://kc.example/auth"},
		{"http://keycloak.internal:8080", "http://keycloak.internal:8080"},
	}
	for _, tc := range cases {
		if got := containerReachableURL(tc.in); got != tc.want {
			t.Errorf("containerReachableURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// fakeSecretStore stands in for the named volume: the resolution decides between the
// operator's value, a persisted one and a fresh one, and that decision must be testable
// without Docker.
type fakeSecretStore struct {
	files   map[string]string
	writes  int
	readErr error
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{files: map[string]string{}}
}

func (f *fakeSecretStore) store() nocCSRFSecretStore {
	return nocCSRFSecretStore{
		read: func(_ context.Context, volume, filePath string) ([]byte, error) {
			if f.readErr != nil {
				return nil, f.readErr
			}
			v, ok := f.files[volume+"/"+filePath]
			if !ok {
				return nil, fmt.Errorf("%w: %s", dockervolume.ErrNotFound, filePath)
			}
			return []byte(v), nil
		},
		write: func(_ context.Context, volume, filePath string, content []byte, _ string) error {
			f.files[volume+"/"+filePath] = string(content)
			f.writes++
			return nil
		},
	}
}

// TestNOCCSRFSecret_IsRandomAndNotDerivedFromThePrefix is the security property the review
// asked for: the entity prefix is public (docker ps, the manifest, the volume names), so a
// secret derived from it is a key anyone can recompute — and this secret is what makes a
// CSRF token unforgeable.
func TestNOCCSRFSecret_IsRandomAndNotDerivedFromThePrefix(t *testing.T) {
	t.Setenv(nocCSRFSecretEnv, "")
	prefix := "cbweb3-noc-brazil"

	// The value the previous, derived implementation would have produced.
	sum := sha256.Sum256([]byte("cbweb3-noc-csrf:" + prefix))
	derived := hex.EncodeToString(sum[:])

	got, err := resolveNOCCSRFSecret(context.Background(), prefix, newFakeSecretStore().store())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got == derived {
		t.Error("the secret is still a pure function of the public prefix; anyone can compute the HMAC key")
	}
	if len(got) != 64 {
		t.Errorf("secret = %q (%d chars), want 32 random bytes hex-encoded", got, len(got))
	}

	// Two stacks provisioned the same way must not share it either.
	other, err := resolveNOCCSRFSecret(context.Background(), "cbweb3-noc-colombia", newFakeSecretStore().store())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got == other {
		t.Error("two stacks minted the same secret")
	}
}

// TestNOCCSRFSecret_SurvivesARestart keeps the property derivation was chosen for: a secret
// that changes between applies refuses every CSRF token issued before it, which presents as
// a browser fault rather than a configuration one.
func TestNOCCSRFSecret_SurvivesARestart(t *testing.T) {
	t.Setenv(nocCSRFSecretEnv, "")
	st := newFakeSecretStore()

	first, err := resolveNOCCSRFSecret(context.Background(), "cbweb3-noc-brazil", st.store())
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	second, err := resolveNOCCSRFSecret(context.Background(), "cbweb3-noc-brazil", st.store())
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if first != second {
		t.Errorf("the secret changed between applies (%q → %q); every live session would start getting 403", first, second)
	}
	if st.writes != 1 {
		t.Errorf("writes = %d, want 1: the persisted secret must be reused, not rewritten", st.writes)
	}
}

// TestNOCCSRFSecret_HonoursTheOperator: the template says a deployment may supply its own
// secret. It has to actually win — and without being appended a second time, which would put
// two entries with the same key into one environment.
func TestNOCCSRFSecret_HonoursTheOperator(t *testing.T) {
	t.Setenv(nocCSRFSecretEnv, "operator-supplied-secret")
	st := newFakeSecretStore()

	got, err := resolveNOCCSRFSecret(context.Background(), "cbweb3-noc-brazil", st.store())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "operator-supplied-secret" {
		t.Errorf("secret = %q, want the operator's", got)
	}
	if st.writes != 0 {
		t.Error("the operator's secret was persisted; the toolkit must not take ownership of it")
	}
}

// TestNOCCSRFSecret_EmptyPersistedValueIsMintedAgain: a blank file must not be handed to the
// backend, which would fall back to a per-process random secret that dies on restart.
func TestNOCCSRFSecret_EmptyPersistedValueIsMintedAgain(t *testing.T) {
	t.Setenv(nocCSRFSecretEnv, "")
	st := newFakeSecretStore()
	st.files[nocCSRFSecretVolume("cbweb3-noc-brazil")+"/"+nocCSRFSecretFile] = "  \n"

	got, err := resolveNOCCSRFSecret(context.Background(), "cbweb3-noc-brazil", st.store())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if strings.TrimSpace(got) == "" {
		t.Error("an empty persisted secret was handed through")
	}
	if st.writes != 1 {
		t.Errorf("writes = %d, want the blank value replaced", st.writes)
	}
}

// TestNOCCSRFSecret_ReadFailureIsReported: a store that cannot be read is a configuration
// problem, and starting the stack anyway would silently drop the whole binding.
func TestNOCCSRFSecret_ReadFailureIsReported(t *testing.T) {
	t.Setenv(nocCSRFSecretEnv, "")
	st := newFakeSecretStore()
	st.readErr = errors.New("docker daemon is not running")

	if _, err := resolveNOCCSRFSecret(context.Background(), "cbweb3-noc-brazil", st.store()); err == nil {
		t.Error("a read failure was swallowed")
	}
}

func manifestWithOrigins(origins ...string) *manifest.Manifest {
	m := &manifest.Manifest{}
	m.Spec.NOC = &manifest.NOC{PortalOrigins: origins}
	return m
}

// TestNOCPortalOrigins_ListsEveryPortalThatMayLogIn is the difference from Scenario B, and
// the reason this is a list at all: ONE NOC backend is shared by every CB portal on the
// host (the samples' own comment says :32645 and :32745 both point at :28645). A browser
// refuses to send credentials to a wildcard, so each origin has to be named.
func TestNOCPortalOrigins_ListsEveryPortalThatMayLogIn(t *testing.T) {
	got := nocPortalOrigins(manifestWithOrigins("http://localhost:32645", "http://localhost:32745"), "localhost", false)

	for _, want := range []string{"http://localhost:32645", "http://localhost:32745"} {
		if !strings.Contains(got, want) {
			t.Errorf("origins %q is missing %q; that portal logs in and is then anonymous", got, want)
		}
	}
	if strings.Contains(got, "*") {
		t.Error("a wildcard origin cannot carry credentials")
	}
}

// TestNOCPortalOrigins_FallsBackToTheSingleCBConvention keeps existing manifests working: a
// manifest written before this field existed must still start, rather than failing on a
// value it could not have known to set.
func TestNOCPortalOrigins_FallsBackToTheSingleCBConvention(t *testing.T) {
	// No spec.noc block at all — the shape every manifest written before this field has.
	got := nocPortalOrigins(&manifest.Manifest{}, "localhost", false)
	if got != "http://localhost:32645" {
		t.Errorf("fallback = %q, want the founding CB's NOC portal", got)
	}
}

// TestNOCPortalOrigins_ProxyCollapsesToOneOrigin: behind the proxy the portal is same-origin
// with the backend, so the manifest's list is irrelevant and must not widen it.
func TestNOCPortalOrigins_ProxyCollapsesToOneOrigin(t *testing.T) {
	got := nocPortalOrigins(manifestWithOrigins("http://should-not-appear:1"), "cb.example", true)
	if strings.Contains(got, "should-not-appear") {
		t.Errorf("proxy mode leaked a manifest origin: %q", got)
	}
}
