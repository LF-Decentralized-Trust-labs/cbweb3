package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
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
	launcherScenario        = "b"
	launcherScenarioUpper   = "B"
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

// launcherStateDir is the neutral, per-host dir shared by both scenarios' toolkits.
// When LAUNCHER_STATE_DIR is unset the default is namespaced by port, so a distinct
// port per entity is enough to keep several launchers on one host from sharing a
// config dir (and mixing entities' fragments).
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

// launcherPortalOffsetB is the host-port offset of each role portal over the entity's
// Besu RPC port (Scenario B frontend bands; see step_found_hub/spoke/join).
var launcherPortalOffsetB = map[string]int{
	"governance": 9000, "bank": 9000, "treasury": 13000, "supervisor": 14000, "noc": 12000,
}

type launcherRolePortal struct{ role, label string }

// launcherRolesB returns the portals a Scenario B entity exposes, by topology role.
// Only commercial banks and central banks get a launcher (the network operator's hub
// is not an A/B entity). A spoke central bank brings up governance + treasury +
// supervisor + a local NOC portal (found-spoke also deploys the NOC stack).
func launcherRolesB(topoRole string) []launcherRolePortal {
	switch topoRole {
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

// LauncherParams configures the per-entity launcher step (Scenario B).
type LauncherParams struct {
	Runner   exec.CommandRunner
	Mode     string // "enable" | "disable" | "" (== disable)
	TopoRole string // hub | central-bank | commercial-bank
	Entity   string // display label shown in the launcher header
	Host     string // browser-facing host baked into portal URLs (default localhost)
	RPCPort  int
	Port     int // launcher host port from the manifest (spec.launcherPort); 0 → env/default
}

// NewLauncherStep builds the SOFT per-entity launcher step. It never blocks the entity's
// operational deploy: a missing launcher image or docker error is non-fatal. Appended by
// apply after the found-spoke / join step list (commercial banks and central banks —
// not the hub).
func NewLauncherStep(p LauncherParams) Step {
	return Step{
		Name: "start-launcher",
		Soft: true,
		Run:  func(ctx context.Context) error { return runLauncherB(ctx, p) },
	}
}

func runLauncherB(ctx context.Context, p LauncherParams) error {
	port := launcherPort(p.Port)
	configsDir := filepath.Join(launcherStateDir(port), "configs")
	fragPath := filepath.Join(configsDir, "config."+launcherScenario+".json")

	// disable (or absent): drop our fragment; tear the launcher down if none remain.
	if p.Mode != "enable" {
		_ = os.Remove(fragPath)
		if launcherNoFragments(configsDir) {
			_, _ = p.Runner.Run(ctx, "docker", "rm", "-f", launcherContainerName(port))
		}
		return nil
	}

	// enable: write this scenario's fragment.
	host := p.Host
	if host == "" {
		host = "localhost"
	}
	portals := make([]launcherPortalJSON, 0)
	for _, rp := range launcherRolesB(p.TopoRole) {
		portals = append(portals, launcherPortalJSON{
			Scenario: launcherScenarioUpper,
			Role:     rp.role,
			Label:    rp.label,
			URL:      fmt.Sprintf("http://%s:%d", host, p.RPCPort+launcherPortalOffsetB[rp.role]),
		})
	}
	if len(portals) == 0 {
		return fmt.Errorf("launcher: unknown topology role %q", p.TopoRole)
	}
	if err := os.MkdirAll(configsDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(launcherFragment{Entity: p.Entity, Portals: portals}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(fragPath, append(data, '\n'), 0o644); err != nil {
		return err
	}

	// Ensure the launcher container is up. It reads fragments at runtime, so if it is
	// already running (possibly started by the other scenario) there is nothing to do —
	// the new fragment is picked up on the next reload.
	if launcherContainerExists(ctx, p.Runner, port) {
		return nil
	}
	if !imageExists(ctx, p.Runner, launcherImage) {
		return fmt.Errorf("launcher image %q not found; build it first with launcher/build.sh", launcherImage)
	}
	_, err = p.Runner.Run(ctx, "docker", "run", "-d",
		"--name", launcherContainerName(port),
		"--restart", "always",
		"-p", fmt.Sprintf("%d:80", port),
		"-v", configsDir+":"+launcherContainerMount+":ro",
		launcherImage)
	return err
}

func launcherContainerExists(ctx context.Context, r exec.CommandRunner, port int) bool {
	_, err := r.Run(ctx, "docker", "container", "inspect", launcherContainerName(port))
	return err == nil
}

// launcherNoFragments reports whether configsDir holds no config.*.json fragment
// (so the launcher has nothing left to show and can be torn down).
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
