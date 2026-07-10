// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads the YAML file at path and parses it into a *ParticipantDeployment.
// It does not validate field values or per-mode presence — use Validate for
// that. Parse errors are returned with a clear, single-line message (no stack
// traces).
func Load(path string) (*ParticipantDeployment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	var pd ParticipantDeployment
	if err := dec.Decode(&pd); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("parse manifest %q: file is empty", path)
		}
		return nil, fmt.Errorf("parse manifest %q: %s", path, cleanYAMLError(err))
	}
	// Reject multi-document YAML: a manifest file must hold exactly one document.
	if err := dec.Decode(new(ParticipantDeployment)); err == nil {
		return nil, fmt.Errorf("parse manifest %q: multiple YAML documents found; a manifest file must contain exactly one document", path)
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse manifest %q: %s", path, cleanYAMLError(err))
	}

	return &pd, nil
}

// LoadSet loads a set of manifests in order, returning the parsed manifests
// in the same order as paths. It fails fast on the first parse error so the
// caller can report a usage/parse failure (exit code 2) distinctly from
// validation errors.
func LoadSet(paths []string) ([]*ParticipantDeployment, error) {
	out := make([]*ParticipantDeployment, 0, len(paths))
	for _, p := range paths {
		pd, err := Load(p)
		if err != nil {
			return nil, err
		}
		out = append(out, pd)
	}
	return out, nil
}

// cleanYAMLError trims the yaml.v3 error to a compact, single-line message.
func cleanYAMLError(err error) string {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "yaml: ")
	msg = strings.ReplaceAll(msg, "\n", "; ")
	return strings.TrimSpace(msg)
}
