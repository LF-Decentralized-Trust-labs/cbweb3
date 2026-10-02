// SPDX-License-Identifier: Apache-2.0

// The Paladin key store must live on a persistent volume, never on tmpfs.
//
// The incident: `/data/keystore:mode=0777` sat in the tmpfs block of every
// Scenario A Paladin compose. That directory holds the bip32 seed of the
// hd_wallet key store, which derives all *salted* Pente endorsement identities
// (`funded_operator.<groupId>@<node>`). The plain `funded_operator` identity is
// served by a static inline key and kept working throughout, so nothing looked
// broken. A Pente privacy group writes its endorser set to the base ledger when
// it is created and that set is immutable, so the container restart of
// 2026-08-17 silently invalidated every group in the LNET environment: the first
// FX propose, eleven days later, reverted with PenteInvalidEndorser — reported
// by Paladin as the opaque "PD012214: Unable to decode revert data".
//
// Reproduced locally on 2026-09-03 to pin the mechanism, because two details of
// the original write-up were wrong in ways that matter here:
//   - A plain `docker restart` is enough. It rewrote /data/keystore/-seed.key
//     (sha256 43c7489e… → 601303110…) and the next propose on the pre-existing
//     group failed with PD012214. No recreation required.
//   - ptx_resolveVerifier does NOT detect this. Paladin caches the identity →
//     verifier mapping in its database on the persistent /data volume, so the
//     salted identity kept resolving to 0xae3027f2… across both seed changes.
//     Only signing re-derives. A runtime probe cannot replace this static guard.
//
// Recreating the groups is the only repair (docs/runbooks/pente-endorser-key-loss-recovery.md),
// so this guard is worth more than the usual regression test: it is the only
// cheap thing standing between a one-line compose edit and a dead environment.
//
// The check reads the key-store path out of the Paladin config templates rather
// than hardcoding /data/keystore. Moving the key store is legitimate; moving it
// onto tmpfs is not, and a guard pinned to the old literal would not notice.
package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// provisioningDir is the provisioning tree, relative to this package. Wider than
// templatesDir on purpose: the spike composes under spikes/ are what the central-bank
// template cites as its reference ("Matches the spk-02 found stack"), so a trap left
// there is a trap that gets copied back into the templates.
const provisioningDir = "../../../provisioning"

// tmpfsService is the slice of a compose service this test judges: the tmpfs
// mounts, and nothing else. Compose accepts tmpfs as a bare string or a list, so
// it is decoded loosely and normalised by tmpfsTargets.
type tmpfsService struct {
	Tmpfs   any    `yaml:"tmpfs"`
	Image   string `yaml:"image"`
	Command any    `yaml:"command"`
}

type tmpfsComposeFile struct {
	Services map[string]tmpfsService `yaml:"services"`
}

// flatten renders a compose string-or-list field as one string for substring
// matching. Both forms appear in this tree (`command:` is a list here, but the
// bare-string form is equally valid compose), and a matcher that understood only
// one would silently pass the other.
func flatten(v any) string {
	switch tv := v.(type) {
	case nil:
		return ""
	case string:
		return tv
	case []any:
		parts := make([]string, 0, len(tv))
		for _, item := range tv {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

// keystorePath is one filesystem key store found in a Paladin config template.
type keystorePath struct {
	file string
	line int
	path string
}

// tmpfsMount is one tmpfs entry found in a compose file.
type tmpfsMount struct {
	file    string
	service string
	target  string
}

// filesystemKeyStoreType matches the line that selects the filesystem key store,
// and keyStorePathKey the `path:` that follows it inside the `filesystem:` block.
// Line-based rather than a YAML decode because these files are Go templates:
// `level: {{.LogLevel}}` is not parseable YAML.
var (
	filesystemKeyStoreType = regexp.MustCompile(`^type:\s*["']?filesystem["']?\s*$`)
	keyStorePathKey        = regexp.MustCompile(`^path:\s*["']?([^"'\s]+)["']?\s*$`)
)

// paladinKeystorePaths returns every filesystem key-store path declared in the
// Paladin config templates under root.
//
// It walks all files rather than globbing config.yaml*, because the spike keeps
// both a rendered config.yaml and its config.yaml.tmpl, and a future template may
// be named differently. The `filesystem` type line and the `path` that follows it
// are matched as a pair so an unrelated `path:` key elsewhere in the config — the
// SQLite DSN, the TLS material — cannot be mistaken for a key store.
func paladinKeystorePaths(t *testing.T, root string) []keystorePath {
	t.Helper()

	var found []keystorePath
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		inFilesystemKeyStore := false
		for i, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			if filesystemKeyStoreType.MatchString(trimmed) {
				inFilesystemKeyStore = true
				continue
			}
			if !inFilesystemKeyStore {
				continue
			}
			if m := keyStorePathKey.FindStringSubmatch(trimmed); m != nil {
				found = append(found, keystorePath{file: filepath.ToSlash(path), line: i + 1, path: m[1]})
				inFilesystemKeyStore = false
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return found
}

// composeFilesUnder returns every compose file under root. The spike stacks are
// named stack-*.yml rather than *compose*, so both spellings are matched.
func composeFilesUnder(t *testing.T, root string) []string {
	t.Helper()

	var found []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		isCompose := strings.Contains(name, "compose") || strings.HasPrefix(name, "stack-")
		if isCompose && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return found
}

// composeTmpfsMounts returns every tmpfs entry declared by every compose file
// under root.
func composeTmpfsMounts(t *testing.T, root string) []tmpfsMount {
	t.Helper()

	var found []tmpfsMount
	for _, path := range composeFilesUnder(t, root) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var cf tmpfsComposeFile
		if yaml.Unmarshal(raw, &cf) != nil {
			// Parametrized templates are still valid YAML; anything that is not is
			// not a compose file this guard can judge, so skip rather than fail.
			continue
		}
		for svc, def := range cf.Services {
			for _, entry := range tmpfsTargets(def.Tmpfs) {
				found = append(found, tmpfsMount{file: filepath.ToSlash(path), service: svc, target: entry})
			}
		}
	}
	return found
}

// tmpfsTargets normalises a compose tmpfs value into mount targets, dropping the
// `:mode=0777` suffix the long form carries.
func tmpfsTargets(v any) []string {
	var raw []string
	switch tv := v.(type) {
	case nil:
		return nil
	case string:
		raw = []string{tv}
	case []any:
		for _, item := range tv {
			if s, ok := item.(string); ok {
				raw = append(raw, s)
			}
		}
	default:
		return nil
	}

	targets := make([]string, 0, len(raw))
	for _, entry := range raw {
		target := entry
		if i := strings.Index(target, ":"); i >= 0 {
			target = target[:i]
		}
		if target = strings.TrimSpace(target); target != "" {
			targets = append(targets, target)
		}
	}
	return targets
}

// mkdirKeyStore matches a `mkdir` whose own arguments include a path ending in
// /keystore. The `[^&|;\n]*` cannot cross a shell separator, which is the whole
// point: the argument list has to belong to THAT mkdir.
//
// A first version of this guard just asked whether the command contained "mkdir"
// and "keystore" anywhere. It passed the mutation test — because the same command
// ends in `find /data/cb/keystore -type f -exec chmod 600 {} +`, so deleting the
// keystore from the mkdir left both substrings in place and the guard saw nothing.
// A check that a one-character edit walks past is worse than no check, since it
// also reports green.
var mkdirKeyStore = regexp.MustCompile(`\bmkdir\b[^&|;\n]*/keystore\b`)

// createsKeyStoreDir reports whether a compose command creates the Paladin key
// store directory.
func createsKeyStoreDir(cmd string) bool {
	return mkdirKeyStore.MatchString(cmd)
}

// tmpfsCoversPath reports whether a tmpfs mounted at target puts p in RAM. A
// parent mount counts: tmpfs on /data hides a key store at /data/keystore just as
// completely as tmpfs on the key store itself, which is the mistake a guard that
// compared only for equality would wave through.
func tmpfsCoversPath(target, p string) bool {
	if strings.TrimSpace(target) == "" || strings.TrimSpace(p) == "" {
		return false
	}
	target, p = filepath.Clean(target), filepath.Clean(p)
	if target == "/" {
		// Clean("/") keeps the trailing slash, so target+"/" below would be "//"
		// and match nothing. A tmpfs on / covers every absolute path.
		return strings.HasPrefix(p, "/")
	}
	return p == target || strings.HasPrefix(p, target+"/")
}

// TestPaladinKeyStore_NotOnTmpfs is the assertion the FX incident asks for.
func TestPaladinKeyStore_NotOnTmpfs(t *testing.T) {
	t.Parallel()

	keystores := paladinKeystorePaths(t, provisioningDir)
	// Three in the template tree alone (central-bank, its bank config, and the
	// commercial-bank template). A walk that found fewer matched too little and
	// this test would pass while proving nothing — the failure mode that matters
	// most in a guard.
	if len(keystores) < 3 {
		t.Fatalf("found %d filesystem key store(s) under %s, expected at least 3; the scan matched "+
			"too little, so this test proves nothing", len(keystores), provisioningDir)
	}

	mounts := composeTmpfsMounts(t, provisioningDir)
	if len(mounts) == 0 {
		t.Fatalf("found no tmpfs mounts under %s; every Paladin service declares /app/jna, so the "+
			"compose scan is broken and this test proves nothing", provisioningDir)
	}

	for _, m := range mounts {
		for _, ks := range keystores {
			if !tmpfsCoversPath(m.target, ks.path) {
				continue
			}
			t.Errorf("service %q in %s mounts tmpfs at %s, which puts the Paladin key store %s "+
				"(%s:%d) in RAM. The bip32 seed there derives every salted Pente endorsement "+
				"identity, a privacy group's endorser set is immutable once written to the base "+
				"ledger, and a container recreation regenerates the seed silently — every existing "+
				"group dies with PenteInvalidEndorser (reported as PD012214). Mount it on a named "+
				"volume; the data-init service already chmods the volume root 777.",
				m.service, m.file, m.target, ks.path, ks.file, ks.line)
		}
	}
}

// TestPaladinKeyStore_DirectoryIsPreCreated is the other half of the fix, and it
// exists because taking the tmpfs entry out was NOT sufficient on its own.
//
// The tmpfs mount was doing two jobs, not one: it made the path writable AND it
// created the directory. Paladin's filesystem key store creates neither — it
// refuses to boot with "PD020800: Path 'keystore' does not exist, or it is not a
// directory", the start-paladin step then fails on a health-check timeout five
// minutes later, and the error names a path that is right there in the config, so
// it reads like a config bug rather than a missing mkdir. That cost a full deploy
// cycle on 2026-09-03.
//
// So: any compose file that runs Paladin must also create the key store
// directory. The check is deliberately loose about HOW (an init container here,
// but an entrypoint would do) and strict about WHETHER.
func TestPaladinKeyStore_DirectoryIsPreCreated(t *testing.T) {
	t.Parallel()

	var checked int
	for _, path := range composeFilesUnder(t, provisioningDir) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var cf tmpfsComposeFile
		if yaml.Unmarshal(raw, &cf) != nil {
			continue
		}

		runsPaladin := false
		for _, svc := range cf.Services {
			if strings.Contains(strings.ToLower(svc.Image), "paladin") {
				runsPaladin = true
				break
			}
		}
		if !runsPaladin {
			continue
		}
		checked++

		creates := false
		for _, svc := range cf.Services {
			if createsKeyStoreDir(flatten(svc.Command)) {
				creates = true
				break
			}
		}
		if !creates {
			t.Errorf("%s runs Paladin but no service in it creates the key store directory. "+
				"Paladin's filesystem key store does not create its own path: it dies with "+
				"PD020800 and start-paladin fails on a health-check timeout. Add the keystore "+
				"dir to the data-init service's mkdir.", filepath.ToSlash(path))
		}
	}

	// Two template composes plus the two spike stacks run Paladin. Fewer means the
	// scan or the image match drifted and this guard proves nothing.
	if checked < 2 {
		t.Fatalf("only %d compose file(s) matched as running Paladin under %s; the scan is too "+
			"narrow for this guard to mean anything", checked, provisioningDir)
	}
}

// TestCreatesKeyStoreDir_RejectsTheEvasionThatFooledIt pins the matcher against
// the exact mutation that walked past its first version, plus the spellings this
// tree actually uses.
func TestCreatesKeyStoreDir_RejectsTheEvasionThatFooledIt(t *testing.T) {
	t.Parallel()

	creates := []string{
		"mkdir -p /data/cb/keystore && chmod -R 777 /data/cb",
		"mkdir -p /data/cb/keystore /data/ba/keystore && chmod -R 777 /data/cb /data/ba",
		"mkdir /data/bank/keystore",
		"mkdir -p /data/cb/keystore && chmod -R 777 /data/cb && find /data/cb/keystore -type f -exec chmod 600 {} +",
	}
	for _, cmd := range creates {
		if !createsKeyStoreDir(cmd) {
			t.Errorf("createsKeyStoreDir(%q) = false; this does create the key store directory", cmd)
		}
	}

	doesNot := []string{
		// The mutation that fooled the substring version: the mkdir no longer names
		// the key store, but the trailing find still mentions it.
		"mkdir -p /data/cb && chmod -R 777 /data/cb && find /data/cb/keystore -type f -exec chmod 600 {} +",
		"chmod -R 777 /data/cb /data/ba",
		"mkdir -p /data/cb && chmod -R 777 /data/cb",
		// Referencing the path without creating it is what PD020800 is about.
		"ls /data/cb/keystore",
		"",
	}
	for _, cmd := range doesNot {
		if createsKeyStoreDir(cmd) {
			t.Errorf("createsKeyStoreDir(%q) = true; nothing here creates the key store directory, "+
				"and Paladin will die with PD020800", cmd)
		}
	}
}

// TestTmpfsCoversPath_SeesParentMounts is the guard on the guard: the covering
// rule is what decides whether a finding is reported at all, so its edges are
// pinned rather than trusted.
func TestTmpfsCoversPath_SeesParentMounts(t *testing.T) {
	t.Parallel()

	covered := [][2]string{
		{"/data/keystore", "/data/keystore"},
		{"/data/keystore/", "/data/keystore"},
		{"/data", "/data/keystore"},
		{"/", "/data/keystore"},
		{"/data/", "/data/keystore/"},
	}
	for _, c := range covered {
		if !tmpfsCoversPath(c[0], c[1]) {
			t.Errorf("tmpfsCoversPath(%q, %q) = false; a tmpfs there does put the key store in RAM",
				c[0], c[1])
		}
	}

	// /app/jna is the one tmpfs these composes are supposed to keep, and
	// /data/keystore-backup is a sibling whose name merely shares a prefix — a
	// guard that flagged either would be one reviewers learn to skip.
	notCovered := [][2]string{
		{"/app/jna", "/data/keystore"},
		{"/data/keystore", "/data"},
		{"/data/keystore-backup", "/data/keystore"},
		{"/datax", "/data/keystore"},
		{"", "/data/keystore"},
		{"/data", ""},
	}
	for _, c := range notCovered {
		if tmpfsCoversPath(c[0], c[1]) {
			t.Errorf("tmpfsCoversPath(%q, %q) = true; the key store is not on that tmpfs and "+
				"flagging it would make the guard noise", c[0], c[1])
		}
	}
}

// TestTmpfsTargets_ReadsBothComposeForms pins the parsing of the tmpfs value.
// Compose accepts a bare string and a list, and the list form carries mode
// options after a colon; a parser that only understood one shape would let the
// other reintroduce the incident.
func TestTmpfsTargets_ReadsBothComposeForms(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want []string
	}{
		{"nil", nil, nil},
		{"bare string", "/data/keystore", []string{"/data/keystore"}},
		{"list", []any{"/app/jna:exec,mode=1777", "/data/keystore:mode=0777"},
			[]string{"/app/jna", "/data/keystore"}},
		{"list without options", []any{"/data/keystore"}, []string{"/data/keystore"}},
		{"non-string members are ignored", []any{"/app/jna", 42}, []string{"/app/jna"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tmpfsTargets(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("tmpfsTargets(%v) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("tmpfsTargets(%v)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}
