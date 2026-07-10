package composetemplate

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// Optional canonical validation: `docker compose config` renders each template
// with its .env.example. Skipped when Docker is not available.
func TestDockerComposeConfig(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available; skipping semantic compose validation")
	}
	names := []string{
		"hub", "entity-besu", "entity-infra", "entity-keycloak",
		"entity-backend", "entity-frontend", "relay", "noc",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			tpl, _ := filepath.Abs(filepath.Join(templatesDir, name+".compose.yaml"))
			env, _ := filepath.Abs(filepath.Join(templatesDir, "vars", name+".env.example"))
			cmd := exec.Command("docker", "compose", "-f", tpl, "--env-file", env, "config", "-q")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("docker compose config failed for %s: %v\n%s", name, err, out)
			}
		})
	}
}
