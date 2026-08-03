// Package orchestrator is the idempotent step engine of the toolkit (TK-B6).
// It runs a dependency-ordered set of steps with Check→skip / Run→persist,
// durable per-step state, a flock, dry-run planning and a structured report.
// It reimplements the Scenario A engine pattern (not imported — Principle I).
package orchestrator

import "context"

// Status of a step in the report / state.
type Status string

const (
	StatusDone       Status = "done"
	StatusSkipped    Status = "skipped"
	StatusFailed     Status = "failed"
	StatusPlanned    Status = "planned"
	StatusSoftFailed Status = "soft-failed"
)

// Step is a unit of work: Check reports whether it is already satisfied
// (idempotency), Run performs the effect. Deps are step names that must run
// before it. A Soft step whose Run fails is reported as soft-failed and does
// NOT interrupt the run (e.g. add-noc-agent — observability, non-blocking).
type Step struct {
	Name  string
	Deps  []string
	Soft  bool
	Check func(ctx context.Context) (bool, error)
	Run   func(ctx context.Context) error
}

// StepResult is the per-step outcome in a Report.
type StepResult struct {
	Name   string `yaml:"name" json:"name"`
	Status Status `yaml:"status" json:"status"`
	Detail string `yaml:"detail,omitempty" json:"detail,omitempty"`
}

// Report is the structured outcome of a run (emitted as json|yaml).
type Report struct {
	Mode string `yaml:"mode" json:"mode"`
	// DataDir is the ABSOLUTE directory this run used for state, keys and volumes. It is reported
	// because node.dataDir is relative in the manifests: the same command run from two working
	// directories provisions two different entities, and until this was visible the only symptom was a
	// bank that had quietly been given a second identity.
	DataDir    string       `yaml:"dataDir,omitempty" json:"dataDir,omitempty"`
	Steps      []StepResult `yaml:"steps" json:"steps"`
	BundlePath string       `yaml:"bundlePath,omitempty" json:"bundlePath,omitempty"`
}
