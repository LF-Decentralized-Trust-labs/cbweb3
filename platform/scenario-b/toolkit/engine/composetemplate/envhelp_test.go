// SPDX-License-Identifier: Apache-2.0

package composetemplate

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// templatesDir is the provisioning templates root, relative to this package.
const templatesDir = "../../../provisioning/templates"

// loadEnv parses a simple KEY=VALUE .env file (ignores blank lines and #).
func loadEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open env %s: %v", path, err)
	}
	defer f.Close()
	env := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		env[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return env
}

func loadTemplateAndEnv(t *testing.T, name string) (*Template, map[string]string) {
	t.Helper()
	tpl, err := Load(filepath.Join(templatesDir, name+".compose.yaml"))
	if err != nil {
		t.Fatalf("load template %s: %v", name, err)
	}
	env := loadEnv(t, filepath.Join(templatesDir, "vars", name+".env.example"))
	return tpl, env
}
