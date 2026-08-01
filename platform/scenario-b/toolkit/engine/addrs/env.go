package addrs

import (
	"bufio"
	"os"
	"strings"
)

// AppendAddr sets KEY=value in an .env file as an idempotent upsert: replaces an
// existing line with the same key, or appends it; creates the file if missing.
func AppendAddr(envPath, key, value string) error {
	var lines []string
	found := false
	if b, err := os.ReadFile(envPath); err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(b)))
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
				lines = append(lines, key+"="+value)
				found = true
			} else {
				lines = append(lines, line)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	out := strings.Join(lines, "\n") + "\n"
	tmp := envPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, envPath)
}

// HasAddrKey reports whether the .env file has a non-empty value for key.
func HasAddrKey(envPath, key string) bool {
	b, err := os.ReadFile(envPath)
	if err != nil {
		return false
	}
	prefix := key + "="
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, prefix) && strings.TrimSpace(line[len(prefix):]) != "" {
			return true
		}
	}
	return false
}

// HasAddr reports whether the .env file already has KEY=value exactly.
func HasAddr(envPath, key, value string) bool {
	b, err := os.ReadFile(envPath)
	if err != nil {
		return false
	}
	target := key + "=" + value
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == target {
			return true
		}
	}
	return false
}

// ReadAddr returns the value of key in an .env file, or "" when the file or the key is absent.
// The runtime-discovered contract addresses are written by earlier steps (AppendAddr) and read
// back by later ones, so a step that needs an address does not have to be handed it.
func ReadAddr(envPath, key string) string {
	b, err := os.ReadFile(envPath)
	if err != nil {
		return ""
	}
	prefix := key + "="
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
