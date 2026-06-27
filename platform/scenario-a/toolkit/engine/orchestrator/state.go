// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"gopkg.in/yaml.v3"
)

// ProvisioningState is the root of .provisioning-state.yaml.
type ProvisioningState struct {
	SpokeID string      `yaml:"spokeID"`
	Steps   []StepState `yaml:"steps"`
}

// StepState records the outcome of a single provisioning step.
type StepState struct {
	Step        string `yaml:"step"`
	Status      string `yaml:"status"`      // "pending" | "done" | "failed"
	CompletedAt string `yaml:"completedAt"` // ISO-8601 UTC; empty when not done
}

// ErrProvisioningLocked is returned when another process holds the file lock.
var ErrProvisioningLocked = errors.New("orchestrator: another process is provisioning this spoke")

// statusFor returns the status of a step from the state, defaulting to "pending".
func statusFor(state ProvisioningState, stepName string) string {
	for _, s := range state.Steps {
		if s.Step == stepName {
			return s.Status
		}
	}
	return "pending"
}

// loadState reads .provisioning-state.yaml from dir.
// Returns a state with all steps pending if the file does not exist.
func loadState(dir string) (ProvisioningState, error) {
	path := stateFilePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ProvisioningState{}, nil
		}
		return ProvisioningState{}, fmt.Errorf("read state file: %w", err)
	}
	var state ProvisioningState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return ProvisioningState{}, fmt.Errorf("parse state file: %w", err)
	}
	return state, nil
}

// saveState writes state to .provisioning-state.yaml atomically (temp file + rename).
func saveState(dir string, state ProvisioningState) error {
	path := stateFilePath(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := yaml.Marshal(&state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Write to temp file in same directory to ensure rename is atomic.
	tmp, err := os.CreateTemp(dir, ".provisioning-state.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename state file: %w", err)
	}
	return nil
}

// markStep updates or appends the named step to the given state and returns the new state.
func markStep(state ProvisioningState, stepName, status, completedAt string) ProvisioningState {
	for i, s := range state.Steps {
		if s.Step == stepName {
			state.Steps[i].Status = status
			state.Steps[i].CompletedAt = completedAt
			return state
		}
	}
	state.Steps = append(state.Steps, StepState{
		Step:        stepName,
		Status:      status,
		CompletedAt: completedAt,
	})
	return state
}

// lockState acquires an exclusive non-blocking file lock on .provisioning.lock in dir.
// Returns ErrProvisioningLocked if another process holds the lock.
// The returned unlock function releases the lock; call it via defer.
func lockState(dir string) (unlock func(), err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	lockPath := filepath.Join(dir, ".provisioning.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrProvisioningLocked
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

func stateFilePath(dir string) string {
	return filepath.Join(dir, ".provisioning-state.yaml")
}
