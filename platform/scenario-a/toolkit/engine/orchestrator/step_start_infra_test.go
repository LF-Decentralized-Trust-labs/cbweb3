// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestStartInfraStep_ComposeEnv(t *testing.T) {
	s := newStartInfraStep("start-cb-infra", "cbweb3-central-bank-brazil", "cbweb3-central-bank-brazil-net",
		t.TempDir(), "/nonexistent/infra-compose.yaml", "cbweb3_central_bank_brazil", "default", "", // senha vazia: resolvida do secrets file, como em produção
		22645, 23645, 0).(*startInfraStep)
	env := strings.Join(s.composeEnv(), "\n")
	for _, want := range []string{
		"ENTITY_INFRA_PREFIX=cbweb3-central-bank-brazil",
		"ENTITY_NET_NAME=cbweb3-central-bank-brazil-net",
		"POSTGRES_HOST_PORT=22645",
		"POSTGRES_DB=cbweb3_central_bank_brazil",
		"POSTGRES_USER=default",
		"REDIS_PASSWORD=",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("composeEnv missing %q", want)
		}
	}

	// Redis is no longer published on a host port: passing REDIS_HOST_PORT again would
	// mean someone re-added the mapping, which is the exposure this removed.
	if strings.Contains(env, "REDIS_HOST_PORT=") {
		t.Error("REDIS_HOST_PORT is back; Redis must not be published on the host")
	}
	// The generated credentials must not be the constants they replaced.
	for _, bad := range []string{"POSTGRES_PASSWORD=default", "POSTGRES_PASSWORD=cbweb3", "REDIS_PASSWORD=default"} {
		if strings.Contains(env, bad) {
			t.Errorf("composeEnv carries the shared literal %q", bad)
		}
	}
}

func TestStartInfraStep_Check_FalseWhenDown(t *testing.T) {
	s := newStartInfraStep("start-cb-infra", "p", "n", t.TempDir(), "/x.yaml", "db", "default", "default", 1, 2, 0)
	done, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("Check should be false when nothing listens on the postgres port")
	}
}

func TestTCPReachable(t *testing.T) {
	// A listening socket is reachable; a closed one is not.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port, _ := strconv.Atoi(strings.TrimPrefix(ln.Addr().String(), "127.0.0.1:"))
	if !tcpReachable("127.0.0.1", port) {
		t.Error("expected listening port to be reachable")
	}
	if tcpReachable("127.0.0.1", 1) {
		t.Error("port 1 should not be reachable")
	}
}
