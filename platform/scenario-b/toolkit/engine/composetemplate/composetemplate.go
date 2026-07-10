// Package composetemplate validates the parametrized Docker Compose templates
// under scenario-b/provisioning/templates/. It does NOT render, run, or compute
// port/name offsets (that is the engine's job, TK-B6). Given a template and an
// example env, it checks: full interpolation, no embedded secrets, deterministic
// named volumes (no host bind mounts except the bank pki/), and — across two
// entity envs — the absence of port/name/network/volume collisions.
package composetemplate

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// varRe matches ${VAR}, ${VAR:-def}, ${VAR-def}, ${VAR:?err}, ${VAR?err},
// ${VAR:+alt}, ${VAR+alt}.
var varRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-|:\?|:\+|-|\?|\+)?([^}]*)\}`)

// residualVarRe detects an un-substituted variable placeholder (`${` followed
// by a name char). It deliberately ignores `${...}` free text in comments.
var residualVarRe = regexp.MustCompile(`\$\{[A-Za-z_]`)

// Template is a loaded compose template (raw text preserved for interpolation).
type Template struct {
	Path string
	Raw  string
}

// Load reads a compose template from disk.
func Load(path string) (*Template, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Template{Path: path, Raw: string(b)}, nil
}

// RequiredVars returns the names of variables that have no default (mandatory).
func (t *Template) RequiredVars() []string {
	set := map[string]bool{}
	for _, m := range varRe.FindAllStringSubmatch(t.Raw, -1) {
		name, op := m[1], m[2]
		if op == "" || op == ":?" || op == "?" {
			set[name] = true
		}
	}
	return sortedKeys(set)
}

// interpolate substitutes ${...} using env and returns the result plus the
// sorted list of mandatory variables that had no value (missing).
func interpolate(raw string, env map[string]string) (string, []string) {
	missing := map[string]bool{}
	out := varRe.ReplaceAllStringFunc(raw, func(s string) string {
		m := varRe.FindStringSubmatch(s)
		name, op, arg := m[1], m[2], m[3]
		val, ok := env[name]
		if ok && val != "" {
			if op == ":+" || op == "+" {
				return arg
			}
			return val
		}
		switch op {
		case ":-", "-":
			return arg
		case ":+", "+":
			return ""
		default: // "", ":?", "?" → mandatory
			missing[name] = true
			return "<<MISSING:" + name + ">>"
		}
	})
	return out, sortedKeys(missing)
}

// RuleError is a single validation failure tied to a named rule.
type RuleError struct{ Rule, Detail string }

// Result is the outcome of validating a template.
type Result struct {
	OK     bool
	Errors []RuleError
}

func (r *Result) add(rule, detail string) {
	r.Errors = append(r.Errors, RuleError{rule, detail})
	r.OK = false
}

// Validate checks a template against an env: interpolation (mandatory vars
// present, no residual placeholder), no-secrets, and named-volumes.
func Validate(t *Template, env map[string]string) Result {
	res := Result{OK: true}
	interpolated, missing := interpolate(t.Raw, env)
	for _, v := range missing {
		res.add("interpolation", "mandatory variable missing: "+v)
	}
	if residualVarRe.MatchString(interpolated) {
		res.add("interpolation", "residual un-interpolated placeholder")
	}
	checkNoSecrets(t.Raw, &res)
	checkNamedVolumes(interpolated, &res)
	return res
}

var (
	secretRe   = regexp.MustCompile(`(?i)(password|secret|private[_-]?key|api[_-]?key)\s*[:=]\s*["']?[^"'\s${][^"'\n]*`)
	varStripRe = regexp.MustCompile(`\$\{[^}]*\}`)
)

// checkNoSecrets rejects literal secret material in the raw template (values
// must come from ${...} at runtime, never hardcoded). ${...} references are
// stripped first so a variable NAMED *_PASSWORD does not trip the check.
func checkNoSecrets(raw string, res *Result) {
	if strings.Contains(raw, "PRIVATE KEY") {
		res.add("no-secrets", "embedded PRIVATE KEY block")
	}
	for _, line := range strings.Split(raw, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		stripped := varStripRe.ReplaceAllString(l, "")
		if secretRe.MatchString(stripped) {
			res.add("no-secrets", "literal sensitive value: "+l)
		}
	}
}

type volumesDoc struct {
	Services map[string]struct {
		Volumes []string `yaml:"volumes"`
	} `yaml:"services"`
	Volumes map[string]struct {
		Name     string `yaml:"name"`
		External *bool  `yaml:"external"`
	} `yaml:"volumes"`
}

// checkNamedVolumes ensures top-level volumes are named (deterministic) or
// external, and that services use no host bind mount except the bank pki/.
func checkNamedVolumes(interpolated string, res *Result) {
	var doc volumesDoc
	if err := yaml.Unmarshal([]byte(interpolated), &doc); err != nil {
		res.add("named-volumes", "invalid YAML after interpolation: "+err.Error())
		return
	}
	for logical, v := range doc.Volumes {
		if v.Name == "" && (v.External == nil || !*v.External) {
			res.add("named-volumes", "volume '"+logical+"' has no deterministic name:")
		}
	}
	for svc, s := range doc.Services {
		for _, vol := range s.Volumes {
			src := vol
			if i := strings.Index(vol, ":"); i >= 0 {
				src = vol[:i]
			}
			if isHostBind(src) && !strings.Contains(vol, "pki") {
				res.add("named-volumes", "host bind mount in service '"+svc+"': "+vol)
			}
		}
	}
}

func isHostBind(src string) bool {
	return strings.HasPrefix(src, "/") || strings.HasPrefix(src, "./") ||
		strings.HasPrefix(src, "../") || strings.HasPrefix(src, "~") ||
		strings.Contains(src, "$(pwd)") || strings.Contains(src, "${PWD}")
}

func sortedKeys(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
