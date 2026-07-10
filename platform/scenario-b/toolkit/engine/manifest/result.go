// SPDX-License-Identifier: Apache-2.0

package manifest

// Finding is a single validation observation naming the offending field or
// resource and an actionable message.
type Finding struct {
	Field   string `yaml:"field" json:"field"`
	Message string `yaml:"message" json:"message"`
}

// Result collects all validation findings for a manifest (or a set of
// manifests). Findings are collected rather than returned on the first
// violation (FR-011): callers get the complete picture in one pass.
type Result struct {
	Errors   []Finding `yaml:"errors" json:"errors"`
	Warnings []Finding `yaml:"warnings" json:"warnings"`
}

// AddError appends a blocking finding.
func (r *Result) AddError(field, message string) {
	r.Errors = append(r.Errors, Finding{Field: field, Message: message})
}

// AddWarning appends a non-blocking finding.
func (r *Result) AddWarning(field, message string) {
	r.Warnings = append(r.Warnings, Finding{Field: field, Message: message})
}

// Valid reports whether the result carries no blocking errors. Warnings do not
// make a manifest invalid.
func (r Result) Valid() bool {
	return len(r.Errors) == 0
}

// Merge folds another result's findings into r.
func (r *Result) Merge(other Result) {
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
}
