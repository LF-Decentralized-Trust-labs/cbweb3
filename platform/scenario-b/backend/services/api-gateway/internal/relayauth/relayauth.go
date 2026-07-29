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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("relayauth: %s public key is not ECDSA (%T)", path, cert.PublicKey)
	}
	return pub, nil
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
