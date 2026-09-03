// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"strings"
	"testing"
)

func TestFrontendHostOrLocal(t *testing.T) {
	if got := frontendHostOrLocal(""); got != "localhost" {
		t.Fatalf("empty host: got %q, want localhost", got)
	}
	if got := frontendHostOrLocal("3.144.127.135"); got != "3.144.127.135" {
		t.Fatalf("routable host: got %q, want 3.144.127.135", got)
	}
}

func TestCorsOriginsCB(t *testing.T) {
	// localhost / empty → only the four local portal origins.
	local := corsOriginsCB(9545, "")
	for _, want := range []string{
		"http://localhost:18545", // governance +9000
		"http://localhost:22545", // treasury +13000
		"http://localhost:23545", // supervisor +14000
		"http://localhost:21545", // noc +12000
	} {
		if !strings.Contains(local, want) {
			t.Errorf("local CORS missing %q in %q", want, local)
		}
	}
	if strings.Contains(local, "3.144.127.135") {
		t.Errorf("local CORS must not contain a remote host: %q", local)
	}

	// Routable host → local origins preserved AND remote origins added.
	remote := corsOriginsCB(9545, "3.144.127.135")
	for _, want := range []string{
		"http://localhost:18545",     // local kept for host-machine browsers
		"http://3.144.127.135:18545", // governance
		"http://3.144.127.135:22545", // treasury
		"http://3.144.127.135:23545", // supervisor
		"http://3.144.127.135:21545", // noc
	} {
		if !strings.Contains(remote, want) {
			t.Errorf("remote CORS missing %q in %q", want, remote)
		}
	}
}

func TestCorsOriginSingle(t *testing.T) {
	if got := corsOriginSingle(9545, ""); got != "http://localhost:18545" {
		t.Fatalf("local single origin: got %q", got)
	}
	remote := corsOriginSingle(9545, "3.144.127.135")
	if !strings.Contains(remote, "http://localhost:18545") || !strings.Contains(remote, "http://3.144.127.135:18545") {
		t.Fatalf("remote single origin must keep local and add remote: %q", remote)
	}
}
