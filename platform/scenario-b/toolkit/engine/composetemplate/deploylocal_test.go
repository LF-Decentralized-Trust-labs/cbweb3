package composetemplate

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// SC-004: TK-B4 must not modify scenario-b/deploy/local. This asserts the
// path is clean in git (no template work leaked into the legacy compose path).
func TestDeployLocalUntouched(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available; skipping deploy/local guard")
	}
	deployLocal, _ := filepath.Abs("../../../deploy/local")
	out, err := exec.Command("git", "status", "--porcelain", "--", deployLocal).CombinedOutput()
	if err != nil {
		t.Skipf("git status unavailable: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("deploy/local must remain untouched by TK-B4, but git reports:\n%s", out)
	}
}
