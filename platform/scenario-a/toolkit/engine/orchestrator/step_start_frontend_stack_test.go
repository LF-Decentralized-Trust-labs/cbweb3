// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"strings"
	"testing"
)

// envValue returns the value of KEY=value in an env slice, or "" if absent.
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}

// TestFrontendComposeEnv_PerEntityImageTag locks in that each entity renders a
// DISTINCT frontend image tag carrying its OWN api-gateway URL. VITE_* are baked at
// build time, so a shared tag would let one bank's bundle (with its gateway URL) be
// reused by another, sending the browser to the wrong gateway and failing CORS.
func TestFrontendComposeEnv_PerEntityImageTag(t *testing.T) {
	itau := newStartFrontendStackStep(StepStartBankFrontend, frontendStackParams{
		EntityPrefix: "cbweb3-bank-itau",
		Services:     []frontendService{{Service: "bank", Port: 25646}},
		APIURL:       "http://localhost:18646/api/v1/",
		ImageTag:     "bank-itau",
	}).(*startFrontendStackStep)

	bradesco := newStartFrontendStackStep(StepStartBankFrontend, frontendStackParams{
		EntityPrefix: "cbweb3-bank-bradesco",
		Services:     []frontendService{{Service: "bank", Port: 25647}},
		APIURL:       "http://localhost:18647/api/v1/",
		ImageTag:     "bank-bradesco",
	}).(*startFrontendStackStep)

	itauEnv := itau.composeEnv()
	bradescoEnv := bradesco.composeEnv()

	if got := envValue(itauEnv, "FRONTEND_IMAGE_TAG"); got != "bank-itau" {
		t.Errorf("itau FRONTEND_IMAGE_TAG = %q; want bank-itau", got)
	}
	if got := envValue(bradescoEnv, "FRONTEND_IMAGE_TAG"); got != "bank-bradesco" {
		t.Errorf("bradesco FRONTEND_IMAGE_TAG = %q; want bank-bradesco", got)
	}

	// The two entities must NOT share an image tag (the bug: both were "local").
	if envValue(itauEnv, "FRONTEND_IMAGE_TAG") == envValue(bradescoEnv, "FRONTEND_IMAGE_TAG") {
		t.Fatal("itau and bradesco share a frontend image tag; bundles will be reused across entities")
	}

	// Each entity's baked api-gateway URL must point at its own gateway.
	if got := envValue(bradescoEnv, "VITE_API_URL"); got != "http://localhost:18647/api/v1/" {
		t.Errorf("bradesco VITE_API_URL = %q; want its own gateway (18647)", got)
	}
}
