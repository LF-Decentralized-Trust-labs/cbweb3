// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// The per-entity launcher is the distributed A/B entry point: one launcher per entity,
// on that entity's own host, listing only that entity's local portals. Each scenario's
// toolkit writes ONLY its own fragment (config.<scenario>.json) into a shared, neutral
// state dir; the launcher SPA merges the fragments client-side. So Scenario A and B
// never read each other's fragment, and the launcher image is generic (built once by
// launcher/build.sh — this step only runs it).
const (
	launcherContainerPrefix = "cbweb3-launcher"
	launcherImage           = "cbweb3/launcher:local"
	launcherDefaultPort     = 5190
	launcherScenarioUpper   = "A"
	launcherFragmentFile    = "config.a.json"
	launcherContainerMount  = "/usr/share/nginx/html/configs"
)

// launcherPort resolves the launcher host port: the manifest's spec.launcherPort (when
// > 0) wins, else the LAUNCHER_PORT env (fallback/override), else the default 5190.
// One launcher per host is the target model; distinct ports let several entities each
// run their own launcher on ONE host (local all-in-one). Both scenarios of the same
// entity declare the same port and thus share one launcher.
func launcherPort(manifestPort int) int {
	if manifestPort > 0 {
		return manifestPort
	}
	if v := os.Getenv("LAUNCHER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			return p
		}
	}
	return launcherDefaultPort
}

// launcherContainerName derives the container name from the port so distinct ports
// yield distinct containers (no name clash on one host), while both scenarios of the
// same entity — same port — share one container.
func launcherContainerName(port int) string {
	return fmt.Sprintf("%s-%d", launcherContainerPrefix, port)
}

// launcherPortalOffsetA is the host-port offset of each role portal over the entity's
// Besu RPC port (Scenario A frontend bands; see ports.go).
var launcherPortalOffsetA = map[string]int{
	"governance": portOffsetFrontendPrimary,
	"bank":       portOffsetFrontendPrimary,
	"treasury":   portOffsetFrontendSecondary,
	"supervisor": portOffsetFrontendSupervisor,
	"noc":        portOffsetFrontendNOC,
}

type launcherRolePortal struct{ role, label string }

// launcherRolesA returns the portals a Scenario A entity exposes, by spec.role.
func launcherRolesA(specRole string) []launcherRolePortal {
	switch specRole {
	case "central-bank":
		return []launcherRolePortal{
			{"governance", "Governance"}, {"treasury", "Treasury"},
			{"supervisor", "Supervisor"}, {"noc", "NOC"},
		}
	case "commercial-bank":
		return []launcherRolePortal{{"bank", "Bank Portal"}}
	default:
		return nil
	}
}

type launcherPortalJSON struct {
	Scenario string `json:"scenario"`
	Role     string `json:"role"`
	Label    string `json:"label"`
	URL      string `json:"url"`
}

type launcherFragment struct {
	Entity  string               `json:"entity"`
	Portals []launcherPortalJSON `json:"portals"`
}

// launcherStateDir is the neutral, per-host dir shared by both scenarios' toolkits on
// the same VPS. Each writes configs/config.<scenario>.json here; the launcher mounts
// configs/ read-only. Must match Scenario B's default for cross-scenario sharing.
func launcherStateDir(port int) string {
	if d := os.Getenv("LAUNCHER_STATE_DIR"); d != "" {
		return d
	}
	base := "/tmp/cbweb3-launcher"
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		base = filepath.Join(home, ".cbweb3-launcher")
	}
	return filepath.Join(base, strconv.Itoa(port))
}

// launcherStep is the SOFT per-entity launcher step. It is soft-by-logging: the A
// orchestrator has no soft flag, so Run never returns an error for a non-fatal launcher
// problem (missing image, docker failure) — it logs and returns nil so the entity's
// operational deploy is never blocked by this accessory.
type launcherStep struct {
	mode     string // enable | disable | "" (== disable)
	specRole string // central-bank | commercial-bank
	entity   string // display label
	host     string // browser-facing host (default localhost)
	rpcPort  int
	port     int  // launcher host port from the manifest (spec.launcherPort); 0 → env/default
	proxy    bool // spec.proxy == enable: portal links are path-based (/<scn>/<role>/) on :80, not host:port
}

func newLauncherStep(mode, specRole, entity, host string, rpcPort, port int, proxy bool) Step {
	return &launcherStep{mode: mode, specRole: specRole, entity: entity, host: host, rpcPort: rpcPort, port: port, proxy: proxy}
}

func (s *launcherStep) Name() string { return StepStartLauncher }

// Check always returns false: the step is idempotent and re-runs each apply so the
// fragment stays current (and disable stays effective).
func (s *launcherStep) Check(context.Context) (bool, error) { return false, nil }

func (s *launcherStep) Run(ctx context.Context) error {
	port := launcherPort(s.port)
	configsDir := filepath.Join(launcherStateDir(port), "configs")
	fragPath := filepath.Join(configsDir, launcherFragmentFile)

	// disable (or absent): drop our fragment; tear the launcher down if none remain.
	if s.mode != "enable" {
		_ = os.Remove(fragPath)
		if launcherNoFragments(configsDir) {
			_ = exec.CommandContext(ctx, "docker", "rm", "-f", launcherContainerName(port)).Run()
		}
		return nil
	}

	host := s.host
	if host == "" {
		host = "localhost"
	}
	roles := launcherRolesA(s.specRole)
	if len(roles) == 0 {
		fmt.Fprintf(os.Stderr, "launcher: unknown spec.role %q; skipping\n", s.specRole)
		return nil
	}
	portals := make([]launcherPortalJSON, 0, len(roles))
	for _, rp := range roles {
		// Behind the proxy every portal is on :80 under a path (/<scn>/<role>/); NOC is
		// not proxied yet (hub-owned), so it is dropped from the proxy-mode launcher.
		if s.proxy {
			if rp.role == "noc" {
				continue
			}
			portals = append(portals, launcherPortalJSON{
				Scenario: launcherScenarioUpper,
				Role:     rp.role,
				Label:    rp.label,
				URL:      "http://" + host + proxyPortalBase(rp.role),
			})
			continue
		}
		portals = append(portals, launcherPortalJSON{
			Scenario: launcherScenarioUpper,
			Role:     rp.role,
			Label:    rp.label,
			URL:      fmt.Sprintf("http://%s:%d", host, s.rpcPort+launcherPortalOffsetA[rp.role]),
		})
	}
	if err := os.MkdirAll(configsDir, 0o755); err != nil {
		return fmt.Errorf("launcher: create config dir: %w", err)
	}
	data, err := json.MarshalIndent(launcherFragment{Entity: s.entity, Portals: portals}, "", "  ")
	if err != nil {
		return fmt.Errorf("launcher: marshal fragment: %w", err)
	}
	if err := os.WriteFile(fragPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("launcher: write fragment: %w", err)
	}

	// Ensure the launcher container is up. It reads fragments at runtime, so if it is
	// already running (possibly started by the other scenario) there is nothing to do.
	if launcherContainerExists(ctx, port) {
		return nil
	}
	if !launcherImageExists(ctx) {
		// soft: don't block the deploy — just report how to fix it.
		fmt.Fprintf(os.Stderr, "launcher: image %q not found; build it with launcher/build.sh, then re-apply\n", launcherImage)
		return nil
	}
	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", launcherContainerName(port),
		"--restart", "always",
		"-p", fmt.Sprintf("%d:80", port),
		"-v", configsDir+":"+launcherContainerMount+":ro",
		launcherImage)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "launcher: docker run failed (non-fatal): %v\n%s\n", err, out)
	}
	return nil
}

func launcherContainerExists(ctx context.Context, port int) bool {
	return exec.CommandContext(ctx, "docker", "container", "inspect", launcherContainerName(port)).Run() == nil
}

func launcherImageExists(ctx context.Context) bool {
	return exec.CommandContext(ctx, "docker", "image", "inspect", launcherImage).Run() == nil
}

// launcherNoFragments reports whether configsDir holds no config.*.json fragment.
func launcherNoFragments(configsDir string) bool {
	entries, err := os.ReadDir(configsDir)
	if err != nil {
		return true
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "config.") && strings.HasSuffix(e.Name(), ".json") {
			return false
		}
	}
	return true
}
