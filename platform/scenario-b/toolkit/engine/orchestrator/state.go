// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// State is the durable per-step progress in <dataDir>/.provisioning-state.yaml.
type State struct {
	path  string
	Steps map[string]Status `yaml:"steps"`
}

// LoadState reads (or initializes) the state file under dataDir.
func LoadState(dataDir string) (*State, error) {
	p := filepath.Join(dataDir, ".provisioning-state.yaml")
	s := &State{path: p, Steps: map[string]Status{}}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(b, s); err != nil {
		return nil, err
	}
	if s.Steps == nil {
		s.Steps = map[string]Status{}
	}
	s.path = p
	return s, nil
}

func (s *State) Get(name string) Status { return s.Steps[name] }

// Set records a step status and persists atomically.
func (s *State) Set(name string, st Status) error {
	s.Steps[name] = st
	b, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
