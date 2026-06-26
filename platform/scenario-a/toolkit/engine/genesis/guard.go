// SPDX-License-Identifier: Apache-2.0

// Package genesis provides the genesis existence guard for the Scenario A
// provisioning toolkit. Every "apply" command in the engine MUST call
// GuardGenesis as its first action, before invoking
// "besu operator generate-blockchain-config" or any file write.
//
// Genesis-complete signal:
//
//	The canonical signal that genesis has been generated is the presence of
//	a non-empty, parseable file at {workspacePath}/genesis/genesis.json.
//	This is consistent with how startBesu.sh copies the file at line 119–121.
//
// State machine (GenesisState → GuardDecision):
//
//	GenesisAbsent  → DecisionProceed
//	GenesisPresent + forceReinit=false → DecisionSkip
//	GenesisPresent + forceReinit=true  → DecisionProceed
//	GenesisCorrupt + forceReinit=false → DecisionAbort
//	GenesisCorrupt + forceReinit=true  → DecisionProceed
//
// The guard is read-only — it never modifies any file on disk.
// Every call to GuardGenesis emits exactly one GenesisLogEvent JSON line
// to the provided io.Writer (pass os.Stdout in production callers):
//
//	DecisionSkip    → "genesis_skipped" (INFO)
//	DecisionProceed → "genesis_proceed" (INFO)
//	DecisionAbort   → "genesis_error"   (ERROR)
//
// Note: the guard emits "genesis_proceed" — a decision to allow generation —
// NOT "genesis_created". Because the guard never writes, it cannot know that
// generation succeeded; the engine entry point that actually runs
// "besu operator generate-blockchain-config" is responsible for emitting the
// "genesis_created" event after that command completes successfully.
package genesis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// GenesisState represents what the guard found at the genesis path on disk.
type GenesisState int

const (
	GenesisAbsent  GenesisState = iota // genesis/genesis.json does not exist
	GenesisPresent                     // genesis/genesis.json exists and is valid JSON with non-zero size
	GenesisCorrupt                     // genesis/genesis.json exists but is empty or not parseable as JSON
)

// GuardDecision is the action the orchestration engine must take.
type GuardDecision int

const (
	DecisionProceed GuardDecision = iota // genesis is absent; run besu operator generate-blockchain-config
	DecisionSkip                         // genesis is present and valid; skip generation
	DecisionAbort                        // genesis is corrupt; exit non-zero
)

// GuardResult carries the guard's decision and enough information to log a structured event.
type GuardResult struct {
	Decision    GuardDecision
	Reason      string
	GenesisPath string
	SpokeID     string
}

// GenesisLogEvent is a structured log record emitted to stdout as a single-line JSON object.
type GenesisLogEvent struct {
	Event       string `json:"event"`
	SpokeID     string `json:"spoke_id"`
	GenesisPath string `json:"genesis_path"`
	Timestamp   string `json:"timestamp"`
	Severity    string `json:"severity"`
	Service     string `json:"service"`
	Reason      string `json:"reason"`
}

func genesisFilePath(workspacePath string) string {
	return filepath.Join(workspacePath, "genesis", "genesis.json")
}

// CheckGenesis inspects workspacePath/genesis/genesis.json and returns its state.
// It does NOT log; callers use the returned GenesisState to build log events.
func CheckGenesis(workspacePath string) (GenesisState, error) {
	path := genesisFilePath(workspacePath)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GenesisAbsent, nil
		}
		return GenesisAbsent, fmt.Errorf("stat genesis file: %w", err)
	}

	if info.Size() == 0 {
		return GenesisCorrupt, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return GenesisAbsent, fmt.Errorf("read genesis file: %w", err)
	}

	if !json.Valid(data) {
		return GenesisCorrupt, nil
	}

	return GenesisPresent, nil
}

func emitLogEvent(w io.Writer, event, severity, reason, spokeID, genesisPath string) {
	logEvent := GenesisLogEvent{
		Event:       event,
		SpokeID:     spokeID,
		GenesisPath: genesisPath,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Severity:    severity,
		Service:     "provisioning-toolkit",
		Reason:      reason,
	}
	jsonBytes, err := json.Marshal(logEvent)
	if err != nil {
		fallback := `{"event":"genesis_error","severity":"ERROR","reason":"failed to marshal log event"}`
		fmt.Fprintln(w, fallback)
		return
	}
	if _, err := fmt.Fprintln(w, string(jsonBytes)); err != nil {
		fallback := `{"event":"genesis_error","severity":"ERROR","reason":"failed to write log event"}`
		fmt.Fprintln(w, fallback)
	}
}

// GuardGenesis is the main entry point for the engine.
// It checks genesis state, applies the forceReinit flag, emits a structured log
// event to w (pass os.Stdout in production), and returns a GuardResult.
// It returns a non-nil error only for filesystem I/O failures unrelated to
// genesis state (e.g., permission denied reading the directory).
func GuardGenesis(spokeID, workspacePath string, forceReinit bool, w io.Writer) (GuardResult, error) {
	path := genesisFilePath(workspacePath)

	state, err := CheckGenesis(workspacePath)
	if err != nil {
		return GuardResult{}, err
	}

	switch state {
	case GenesisPresent:
		if !forceReinit {
			result := GuardResult{
				Decision:    DecisionSkip,
				Reason:      "genesis already present",
				GenesisPath: path,
				SpokeID:     spokeID,
			}
			emitLogEvent(w, "genesis_skipped", "INFO", result.Reason, spokeID, path)
			return result, nil
		}
		result := GuardResult{
			Decision:    DecisionProceed,
			Reason:      "genesis present, force-reinit enabled",
			GenesisPath: path,
			SpokeID:     spokeID,
		}
		emitLogEvent(w, "genesis_proceed", "INFO", result.Reason, spokeID, path)
		return result, nil
	case GenesisAbsent:
		result := GuardResult{
			Decision:    DecisionProceed,
			Reason:      "genesis absent",
			GenesisPath: path,
			SpokeID:     spokeID,
		}
		emitLogEvent(w, "genesis_proceed", "INFO", result.Reason, spokeID, path)
		return result, nil
	case GenesisCorrupt:
		if !forceReinit {
			result := GuardResult{
				Decision:    DecisionAbort,
				Reason:      "genesis corrupt: invalid or empty file",
				GenesisPath: path,
				SpokeID:     spokeID,
			}
			emitLogEvent(w, "genesis_error", "ERROR", result.Reason, spokeID, path)
			return result, nil
		}
		result := GuardResult{
			Decision:    DecisionProceed,
			Reason:      "genesis corrupt, force-reinit enabled",
			GenesisPath: path,
			SpokeID:     spokeID,
		}
		emitLogEvent(w, "genesis_proceed", "INFO", result.Reason, spokeID, path)
		return result, nil
	default:
		return GuardResult{}, errors.New("unknown genesis state")
	}
}
