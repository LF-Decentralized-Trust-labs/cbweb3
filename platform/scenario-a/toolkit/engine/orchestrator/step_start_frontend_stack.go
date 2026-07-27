// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// frontendService is one portal to bring up: its compose service name and the
// host port it is published on.
type frontendService struct {
	Service string
	Port    int
}

// startFrontendStackStep builds and brings up an entity's frontend portals (feature
// 034) from the per-entity frontend compose template, then waits for each portal's
// nginx to serve. VITE_* are build-time, baked into the static bundle, so they point
// at the entity's host-published api-gateway/Keycloak ports. Reused by the CB (found
// → governance/treasury/supervisor/noc) and a commercial bank (join → bank).
type startFrontendStackStep struct {
	name           string
	entityPrefix   string
	netName        string
	context        string
	composePath    string
	services       []frontendService
	apiURL         string // with /api/v1/ suffix (bank/governance/treasury)
	apiBase        string // no suffix (supervisor, noc)
	portalOwner    string
	fiatSymbol     string
	institution    string
	keycloakURL    string
	keycloakRealm  string
	keycloakClient string
	launcherURL    string
	nocBackendURL  string // VITE_NOC_BACKEND_URL for the co-located NOC portal (observe backend)
	imageTag       string
	healthTimeout  time.Duration
	healthInterval time.Duration
}

func newStartFrontendStackStep(name string, p frontendStackParams) Step {
	return &startFrontendStackStep{
		name:           name,
		entityPrefix:   p.EntityPrefix,
		netName:        p.NetName,
		context:        p.Context,
		composePath:    p.ComposePath,
		services:       p.Services,
		apiURL:         p.APIURL,
		apiBase:        p.APIBase,
		portalOwner:    p.PortalOwner,
		fiatSymbol:     p.FiatSymbol,
		institution:    p.Institution,
		keycloakURL:    p.KeycloakURL,
		keycloakRealm:  p.KeycloakRealm,
		keycloakClient: p.KeycloakClient,
		launcherURL:    p.LauncherURL,
		nocBackendURL:  p.NOCBackendURL,
		imageTag:       p.ImageTag,
		healthTimeout:  p.HealthTimeout,
		healthInterval: p.HealthInterval,
	}
}

// frontendStackParams groups the inputs for startFrontendStackStep.
type frontendStackParams struct {
	EntityPrefix   string
	NetName        string
	Context        string
	ComposePath    string
	Services       []frontendService
	APIURL         string
	APIBase        string
	PortalOwner    string
	FiatSymbol     string
	Institution    string
	KeycloakURL    string
	KeycloakRealm  string
	KeycloakClient string
	LauncherURL    string
	NOCBackendURL  string
	ImageTag       string
	HealthTimeout  time.Duration
	HealthInterval time.Duration
}

func (s *startFrontendStackStep) Name() string { return s.name }

// Check returns true if every selected portal's nginx already serves.
func (s *startFrontendStackStep) Check(ctx context.Context) (bool, error) {
	for _, svc := range s.services {
		if !httpHealthy(ctx, fmt.Sprintf("http://localhost:%d/", svc.Port)) {
			return false, nil
		}
	}
	return true, nil
}

func (s *startFrontendStackStep) Run(ctx context.Context) error {
	// Unique compose project per entity (shared template would otherwise reconcile
	// and remove another entity's containers — see step_start_infra.go).
	//
	// --build: VITE_* are baked into the static bundle at build time, so the image
	// must be (re)built per entity with this entity's args. Combined with a per-entity
	// image tag (ImageTag), this prevents one entity's bundle — carrying its own
	// api-gateway URL — from being reused by another and failing CORS in the browser.
	args := []string{"compose", "-p", s.entityPrefix + "-frontend", "-f", s.composePath, "up", "-d", "--build"}
	for _, svc := range s.services {
		args = append(args, svc.Service)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = s.composeEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose up frontend: %w\noutput:\n%s", err, out)
	}

	deadline := time.Now().Add(s.healthTimeout)
	for time.Now().Before(deadline) {
		if done, _ := s.Check(ctx); done {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.healthInterval):
		}
	}
	return fmt.Errorf("frontend portals health check timed out after %s", s.healthTimeout)
}

func (s *startFrontendStackStep) composeEnv() []string {
	tag := s.imageTag
	if tag == "" {
		tag = "local"
	}
	env := append(os.Environ(),
		"ENTITY_PREFIX="+s.entityPrefix,
		"ENTITY_NET_NAME="+s.netName,
		"FRONTEND_CONTEXT="+s.context,
		"VITE_API_URL="+s.apiURL,
		"VITE_API_BASE="+s.apiBase,
		"VITE_PORTAL_OWNER="+s.portalOwner,
		"VITE_FIAT_SYMBOL="+s.fiatSymbol,
		"VITE_INSTITUTION_NAME="+s.institution,
		"VITE_KEYCLOAK_URL="+s.keycloakURL,
		"VITE_KEYCLOAK_REALM="+s.keycloakRealm,
		"VITE_KEYCLOAK_CLIENT_ID="+s.keycloakClient,
		"VITE_LAUNCHER_URL="+s.launcherURL,
		"VITE_NOC_BACKEND_URL="+s.nocBackendURL,
		"FRONTEND_IMAGE_TAG="+tag,
	)
	// Publish only the selected portals' ports; others default to 0 (unpublished).
	for _, svc := range s.services {
		env = append(env, frontendPortEnv(svc.Service)+"="+strconv.Itoa(svc.Port))
	}
	return env
}

// frontendPortEnv maps a compose service name to its host-port env var.
func frontendPortEnv(service string) string {
	switch service {
	case "governance":
		return "GOVERNANCE_PORT"
	case "treasury":
		return "TREASURY_PORT"
	case "supervisor":
		return "SUPERVISOR_PORT"
	case "noc":
		return "NOC_PORT"
	case "bank":
		return "BANK_PORT"
	default:
		return "UNKNOWN_PORT"
	}
}
