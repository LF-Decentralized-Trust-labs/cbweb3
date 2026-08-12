// SPDX-License-Identifier: Apache-2.0

// Package relayauth provides per-CB asymmetric authentication for internal
// service-to-service relay endpoints (R2-CR-6 / TASK-17, requirement C).
//
// Instead of a single shared symmetric secret (INTERNAL_RELAY_AUTH_SECRET, which
// shipped identically in every .env.infra.*.example), each entity signs its relay
// requests with the EC private key the project's PKI already issues
// (PKI_DIR/<entity>.key) and the receiver verifies against the sender's pinned
// certificate (PKI_DIR/<entity>.crt). A forged or replayed message can no longer be
// authenticated without the originating CB's private key.
//
// The signature covers a canonical request string bound to a timestamp, so it
// authenticates this specific request and bounds replay to a short window. It is
// transport-agnostic (an application-layer signature in HTTP headers), so it works
// across the Cacti relay hop without requiring mTLS termination changes.
package relayauth

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// randReader is the entropy source for ECDSA signing; indirected for testability.
var randReader io.Reader = rand.Reader

// Header names carried on a signed relay request.
const (
	HeaderKeyID     = "X-Relay-Key-Id"    // sender entity id, e.g. "central-bank-a"
	HeaderTimestamp = "X-Relay-Timestamp" // unix seconds, as a decimal string
	HeaderSignature = "X-Relay-Signature" // base64(ASN.1 ECDSA signature)
	scheme          = "cbweb3-relay-v1"   // canonical-string version tag
)

// DefaultMaxSkew bounds how far a request timestamp may be from the verifier's
// clock (in either direction). It caps the replay window for a captured signature.
const DefaultMaxSkew = 5 * time.Minute

// CanonicalString builds the exact bytes that are signed and verified. Binding the
// method, path, body hash, and timestamp makes a signature non-transferable to a
// different request and non-replayable outside the skew window.
func CanonicalString(timestamp int64, method, path string, body []byte) string {
	sum := sha256.Sum256(body)
	return strings.Join([]string{
		scheme,
		strconv.FormatInt(timestamp, 10),
		strings.ToUpper(method),
		path,
		fmt.Sprintf("%x", sum[:]),
	}, "\n")
}

// Sign produces a base64 ASN.1 ECDSA signature over the canonical request string.
func Sign(key *ecdsa.PrivateKey, timestamp int64, method, path string, body []byte) (string, error) {
	if key == nil {
		return "", fmt.Errorf("relayauth: nil signing key")
	}
	digest := sha256.Sum256([]byte(CanonicalString(timestamp, method, path, body)))
	sig, err := ecdsa.SignASN1(randReader, key, digest[:])
	if err != nil {
		return "", fmt.Errorf("relayauth: sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify checks a base64 ASN.1 ECDSA signature against the canonical request string.
func Verify(pub *ecdsa.PublicKey, timestamp int64, method, path string, body []byte, signatureB64 string) error {
	if pub == nil {
		return fmt.Errorf("relayauth: nil verifying key")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureB64))
	if err != nil {
		return fmt.Errorf("relayauth: malformed signature: %w", err)
	}
	digest := sha256.Sum256([]byte(CanonicalString(timestamp, method, path, body)))
	if !ecdsa.VerifyASN1(pub, digest[:], sig) {
		return fmt.Errorf("relayauth: signature does not verify")
	}
	return nil
}

// LoadECPrivateKey reads an EC private key in PEM form (the format produced by the
// project PKI as PKI_DIR/<entity>.key). It accepts both "EC PRIVATE KEY" (SEC1) and
// "PRIVATE KEY" (PKCS#8) blocks.
func LoadECPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("relayauth: read private key %s: %w", path, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("relayauth: no PEM block in %s", path)
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("relayauth: parse PKCS8 %s: %w", path, err)
		}
		ec, ok := k.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("relayauth: %s is not an EC private key (%T)", path, k)
		}
		return ec, nil
	default:
		return nil, fmt.Errorf("relayauth: unexpected PEM type %q in %s", block.Type, path)
	}
}

// LoadCertPublicKey reads an X.509 certificate (PKI_DIR/<entity>.crt) and returns its
// ECDSA public key.
func LoadCertPublicKey(path string) (*ecdsa.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("relayauth: read cert %s: %w", path, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("relayauth: no CERTIFICATE PEM block in %s", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("relayauth: parse cert %s: %w", path, err)
	}
	// A certificate authority is not a service identity. The toolkit writes the CB's CA as
	// central-bank.crt, which the name-based skip below does not catch, so it was pinned under the
	// key-id "central-bank" — a trust anchor asserting something it does not mean.
	if cert.IsCA {
		return nil, fmt.Errorf("relayauth: %s is a CA certificate, not a peer identity", path)
	}
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("relayauth: %s public key is not ECDSA (%T)", path, cert.PublicKey)
	}
	return pub, nil
}

// ParticipantPin is one onboarded peer's identity as the central bank recorded it: the certificate
// the CB itself issued when signing that peer's CSR, plus whether the peer is currently active.
//
// The certificate certifies the very key the peer signs with — a bank's CSR is generated over the
// same PKI_DIR/<bankCode>.key its relay signer uses — which is what makes the participants table a
// valid pin source rather than merely a record.
type ParticipantPin struct {
	ID      string
	CertPEM string
	// Active is the compliance status. An inactive peer is not pinned, and it also suppresses any
	// file pin for the same id: otherwise deactivating a bank would revoke nothing, because a stale
	// file would keep authenticating it.
	Active bool
}

// LoadCertPublicKeyPEM extracts the ECDSA public key from a PEM certificate in memory.
func LoadCertPublicKeyPEM(raw []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("relayauth: no CERTIFICATE PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("relayauth: parse certificate: %w", err)
	}
	if cert.IsCA {
		return nil, errors.New("relayauth: certificate is a CA, not a peer identity")
	}
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("relayauth: certificate public key is not ECDSA (%T)", cert.PublicKey)
	}
	return pub, nil
}

// BuildRegistry merges file-sourced pins with the central bank's own participant records.
//
// Precedence, and the reason for it:
//
//	active participant    pinned from the issued certificate, overriding any file pin — the table is
//	                      authoritative, so a stale file cannot keep a superseded key alive
//	inactive participant  NOT pinned, and any file pin for that id is REMOVED. This is the whole
//	                      revocation story: without the removal, deactivating a bank in compliance
//	                      would leave it authenticating from a leftover file
//	not a participant     the file pin stands — that is how a peer the CB never onboards is trusted,
//	                      the Cacti relay being the case that matters
//
// A malformed certificate is skipped rather than fatal: one bad row must not stop every other peer
// from authenticating.
func BuildRegistry(files *Registry, participants []ParticipantPin) *Registry {
	out := NewRegistry()
	if files != nil {
		out.maxSkew = files.maxSkew
		for id, pub := range files.keys {
			out.keys[id] = pub
		}
	}
	for _, p := range participants {
		if p.ID == "" {
			continue
		}
		if !p.Active {
			// Known to compliance and not active: nothing may pin it, files included.
			delete(out.keys, p.ID)
			continue
		}
		pub, err := LoadCertPublicKeyPEM([]byte(p.CertPEM))
		if err != nil {
			log.Printf("[relay-auth] participant %q has an unusable certificate, not pinned: %v", p.ID, err)
			delete(out.keys, p.ID)
			continue
		}
		out.keys[p.ID] = pub
	}
	return out
}

// Store holds the registry currently in force and allows it to be replaced without locking the
// verification path.
//
// Registries are treated as IMMUTABLE once built: a reload constructs a new one and swaps the
// pointer atomically. That is what makes hot reload safe here — Registry.keys is a plain map, so
// mutating a live registry while requests verify against it would be a data race, and taking a lock
// on every verification to avoid that would put a contended mutex in front of the settlement path.
//
// Reload matters because a central bank's peers are its onboarded banks: one onboarded after the
// gateway booted signs its calls immediately, and with a boot-time-only registry its id would not be
// pinned — so its requests would be rejected with 401 until someone restarted the gateway.
type Store struct {
	p atomic.Pointer[Registry]

	// refresh reloads the registry from its sources; set at wiring time because loading needs
	// database access the relayauth package deliberately does not have.
	mu          sync.Mutex
	refresh     func()
	minInterval time.Duration
	lastRefresh time.Time
}

// NewStore returns a store holding reg (which may be nil).
func NewStore(reg *Registry) *Store {
	s := &Store{}
	s.Set(reg)
	return s
}

// SetRefresher installs the reload function used when a request presents an unknown key-id, and the
// minimum interval between such reloads.
func (s *Store) SetRefresher(refresh func(), minInterval time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refresh = refresh
	s.minInterval = minInterval
}

// EnsureFresh returns the registry in force, reloading it first if keyID is not pinned.
//
// Why reload on a miss. A central bank's peers are its onboarded banks, and a bank signs its calls as
// soon as it has a key — so between onboarding and the next periodic refresh there is a window where a
// legitimate peer is not pinned and every call it makes is rejected with 401, not downgraded. Treating
// the miss as a cache miss closes that window.
//
// It is not a weakening: a reload can only load certificates this central bank itself issued, so
// presenting an unknown id gains an attacker nothing. It is rate-limited so unknown ids cannot become
// a free way to make the gateway hammer its own database.
func (s *Store) EnsureFresh(keyID string) *Registry {
	if s == nil {
		return nil
	}
	current := s.Get()
	// An empty key-id is an unsigned request: nothing to look up, nothing to refresh for.
	if keyID == "" {
		return current
	}
	if current != nil {
		if _, pinned := current.keys[keyID]; pinned {
			return current
		}
	}
	s.mu.Lock()
	refresh := s.refresh
	if refresh != nil && (s.minInterval <= 0 || time.Since(s.lastRefresh) >= s.minInterval) {
		s.lastRefresh = time.Now()
	} else {
		refresh = nil
	}
	s.mu.Unlock()
	if refresh == nil {
		return current
	}
	refresh()
	return s.Get()
}

// Get returns the registry in force; nil-safe.
func (s *Store) Get() *Registry {
	if s == nil {
		return nil
	}
	return s.p.Load()
}

// Set replaces the registry in force.
func (s *Store) Set(reg *Registry) {
	if s == nil {
		return
	}
	s.p.Store(reg)
}

// Registry maps an entity key-id to its verifying public key. It is the trust
// anchor: only entities whose certificate is pinned in the registry can authenticate.
type Registry struct {
	keys    map[string]*ecdsa.PublicKey
	maxSkew time.Duration
}

// NewRegistry builds an empty registry with the default skew tolerance.
func NewRegistry() *Registry {
	return &Registry{keys: map[string]*ecdsa.PublicKey{}, maxSkew: DefaultMaxSkew}
}

// Add pins a public key under a key-id (typically the entity's BANK_CODE).
func (r *Registry) Add(keyID string, pub *ecdsa.PublicKey) {
	r.keys[keyID] = pub
}

// WithMaxSkew overrides the replay window tolerance.
func (r *Registry) WithMaxSkew(d time.Duration) *Registry {
	if d > 0 {
		r.maxSkew = d
	}
	return r
}

// Len reports how many peer keys are pinned.
func (r *Registry) Len() int { return len(r.keys) }

// IDs returns the pinned key-ids, sorted.
//
// For visibility at boot. A registry that is non-empty but missing the callers' ids is worse than an
// empty one: empty means "no signatures here" and the middleware falls back to the shared secret,
// while non-empty means "verify strictly", so a SIGNED request from an unpinned id is rejected with
// 401 and never downgraded. That refusal is correct — downgrading would make the signature
// worthless — but it stops the settlement path, so which ids are pinned has to be visible before
// traffic arrives rather than inferred from a wave of 401s.
func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.keys))
	for id := range r.keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// LoadRegistryFromPKIDir pins one verifying key per entity by reading
// PKI_DIR/<entity>.crt for each entity id in entityIDs. Missing or non-ECDSA certs
// are skipped with the error collected, so a partial PKI still yields a usable
// registry for the entities that are present.
func LoadRegistryFromPKIDir(pkiDir string, entityIDs []string) (*Registry, error) {
	reg := NewRegistry()
	var problems []string
	for _, id := range entityIDs {
		pub, err := LoadCertPublicKey(filepath.Join(pkiDir, id+".crt"))
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		reg.Add(id, pub)
	}
	if reg.Len() == 0 {
		return reg, fmt.Errorf("relayauth: no peer certs loaded from %s: %s", pkiDir, strings.Join(problems, "; "))
	}
	return reg, nil
}

// LoadRegistryGlob pins a verifying key for every entity certificate in pkiDir
// (files matching "*.crt", excluding CA and participant certs). The key-id is the
// cert filename without extension, which matches each entity's BANK_CODE
// (e.g. "central-bank-a.crt" → key-id "central-bank-a"). Returns an empty registry
// (and nil error) when the directory has no usable entity certs, so a gateway
// without PKI simply falls back to legacy auth rather than failing to start.
func LoadRegistryGlob(pkiDir string) (*Registry, error) {
	reg := NewRegistry()
	if pkiDir == "" {
		return reg, nil
	}
	matches, err := filepath.Glob(filepath.Join(pkiDir, "*.crt"))
	if err != nil {
		return reg, fmt.Errorf("relayauth: glob %s: %w", pkiDir, err)
	}
	for _, path := range matches {
		base := strings.TrimSuffix(filepath.Base(path), ".crt")
		if strings.HasSuffix(base, "-ca") || strings.HasSuffix(base, "-participant") {
			continue
		}
		pub, err := LoadCertPublicKey(path)
		if err != nil {
			// Skip non-ECDSA / unreadable certs rather than failing the whole load.
			continue
		}
		reg.Add(base, pub)
	}
	return reg, nil
}

// VerifyRequest authenticates a request from its signed headers. keyID/timestamp/
// signatureB64 come from the X-Relay-* headers. It enforces the skew window, that
// the key-id is pinned, and that the signature verifies over the canonical string.
func (r *Registry) VerifyRequest(keyID, timestampStr, signatureB64, method, path string, body []byte, now time.Time) error {
	if keyID == "" || timestampStr == "" || signatureB64 == "" {
		return fmt.Errorf("relayauth: missing signature headers")
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(timestampStr), 10, 64)
	if err != nil {
		return fmt.Errorf("relayauth: invalid timestamp %q", timestampStr)
	}
	skew := now.Sub(time.Unix(ts, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > r.maxSkew {
		return fmt.Errorf("relayauth: timestamp outside %s window (skew %s)", r.maxSkew, skew.Round(time.Second))
	}
	pub, ok := r.keys[keyID]
	if !ok {
		return fmt.Errorf("relayauth: unknown key-id %q", keyID)
	}
	return Verify(pub, ts, method, path, body, signatureB64)
}

// Signer holds an entity's identity and private key and stamps outbound requests
// with the X-Relay-* signature headers.
type Signer struct {
	keyID string
	key   *ecdsa.PrivateKey
}

// NewSigner builds a Signer from a key-id and an already-loaded EC private key.
func NewSigner(keyID string, key *ecdsa.PrivateKey) *Signer {
	return &Signer{keyID: keyID, key: key}
}

// LoadSigner builds a Signer for entity keyID by loading PKI_DIR/<keyID>.key.
func LoadSigner(pkiDir, keyID string) (*Signer, error) {
	key, err := LoadECPrivateKey(filepath.Join(pkiDir, keyID+".key"))
	if err != nil {
		return nil, err
	}
	return &Signer{keyID: keyID, key: key}, nil
}

// PublicKey returns the verifying key that a peer must pin to authenticate this signer. Exposed so a
// caller can register its own identity (and so tests can verify end to end rather than trusting that
// the headers "look signed").
func (s *Signer) PublicKey() *ecdsa.PublicKey {
	if s == nil || s.key == nil {
		return nil
	}
	return &s.key.PublicKey
}

// KeyID returns the signer's key id (the entity/bank code).
func (s *Signer) KeyID() string {
	if s == nil {
		return ""
	}
	return s.keyID
}

// SignAttestation signs an arbitrary canonical message with the entity's key and returns
// the raw ASN.1 ECDSA signature bytes. Used for OFF-CHAIN governance attestations (e.g.
// circuit-breaker pause/resume) — the on-chain authorization is the signer's address
// (msg.sender), so this blob is an auditable institutional attestation, generated
// server-side so operators never have to supply a signature by hand.
func (s *Signer) SignAttestation(message string) ([]byte, error) {
	if s == nil || s.key == nil {
		return nil, fmt.Errorf("relayauth: nil signing key")
	}
	digest := sha256.Sum256([]byte(message))
	sig, err := ecdsa.SignASN1(randReader, s.key, digest[:])
	if err != nil {
		return nil, fmt.Errorf("relayauth: sign attestation: %w", err)
	}
	return sig, nil
}

// HeadersFor returns the X-Relay-* headers authenticating (method, path, body) at
// the given time. Callers set these on the outbound HTTP request.
func (s *Signer) HeadersFor(method, path string, body []byte, now time.Time) (map[string]string, error) {
	ts := now.Unix()
	sig, err := Sign(s.key, ts, method, path, body)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		HeaderKeyID:     s.keyID,
		HeaderTimestamp: strconv.FormatInt(ts, 10),
		HeaderSignature: sig,
	}, nil
}
