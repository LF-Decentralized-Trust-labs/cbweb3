// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// provisionKeycloakStep brings up the entity's Keycloak (feature 034), backed by
// the entity's dedicated Postgres, importing the realm JSONs rendered from the
// per-entity plan. The CB instance hosts the central-bank + cbweb3 (NOC) realms.
type provisionKeycloakStep struct {
	name          string
	entityPrefix  string
	netName       string
	dataDir       string
	composePath   string
	kcDBURL       string
	kcUser        string
	kcPassword    string
	keycloakImage string
	hostPort      int
	realms        []KeycloakRealmPlan
	timeout       time.Duration
}

func newProvisionKeycloakStep(name string, p keycloakStepParams) Step {
	return &provisionKeycloakStep{
		name:          name,
		entityPrefix:  p.EntityPrefix,
		netName:       p.NetName,
		dataDir:       p.DataDir,
		composePath:   p.ComposePath,
		kcDBURL:       p.KCDBURL,
		kcUser:        p.KCUser,
		kcPassword:    p.KCPassword,
		keycloakImage: p.KeycloakImage,
		hostPort:      p.HostPort,
		realms:        p.Realms,
		timeout:       p.Timeout,
	}
}

// keycloakStepParams groups the inputs for provisionKeycloakStep.
type keycloakStepParams struct {
	EntityPrefix  string
	NetName       string
	DataDir       string
	ComposePath   string
	KCDBURL       string
	KCUser        string
	KCPassword    string
	KeycloakImage string
	HostPort      int
	Realms        []KeycloakRealmPlan
	Timeout       time.Duration
}

func (s *provisionKeycloakStep) Name() string { return s.name }

func (s *provisionKeycloakStep) Check(ctx context.Context) (bool, error) {
	return tcpReachable("localhost", s.hostPort), nil
}

func (s *provisionKeycloakStep) Run(ctx context.Context) error {
	// Render the realm import JSONs the Keycloak container imports on startup.
	importDir := s.importDir()
	if err := os.MkdirAll(importDir, 0o755); err != nil {
		return fmt.Errorf("mkdir keycloak import: %w", err)
	}
	for _, plan := range s.realms {
		data, err := renderRealmJSON(plan)
		if err != nil {
			return fmt.Errorf("render realm %s: %w", plan.Realm, err)
		}
		out := filepath.Join(importDir, plan.Realm+"-realm.json")
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return fmt.Errorf("write realm %s: %w", plan.Realm, err)
		}
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.composePath, "up", "-d", "--wait")
	cmd.Env = s.composeEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose up keycloak: %w\noutput:\n%s", err, out)
	}
	return nil
}

func (s *provisionKeycloakStep) importDir() string {
	return filepath.Join(s.dataDir, "keycloak-import")
}

func (s *provisionKeycloakStep) composeEnv() []string {
	user := s.kcUser
	if user == "" {
		user = "default"
	}
	pass := s.kcPassword
	if pass == "" {
		pass = "default"
	}
	img := s.keycloakImage
	if img == "" {
		img = "quay.io/keycloak/keycloak:26.0"
	}
	return append(os.Environ(),
		"ENTITY_INFRA_PREFIX="+s.entityPrefix,
		"ENTITY_NET_NAME="+s.netName,
		"KEYCLOAK_HOST_PORT="+strconv.Itoa(s.hostPort),
		"KC_DB_URL="+s.kcDBURL,
		"KC_DB_USERNAME="+user,
		"KC_DB_PASSWORD="+pass,
		"KEYCLOAK_IMAGE="+img,
		"KEYCLOAK_IMPORT_DIR="+s.importDir(),
	)
}
