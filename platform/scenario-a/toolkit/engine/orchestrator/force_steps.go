// SPDX-License-Identifier: Apache-2.0

package orchestrator

// ForcedSteps names steps that must run even when their Check reports the work
// already done. It exists for --rebuild.
//
// The Checks that guard this engine's build steps are liveness probes — does
// /healthz answer, does the portal serve a page — and none of them can see the
// source tree. So after editing a Go service or a React app the container is still
// healthy, Check answers true, and the `docker compose up --build` inside Run never
// happens: the apply reports the step satisfied while the stack serves the previous
// binary.
//
// Only steps whose Run is safe to repeat may be forced. Forcing removes the guard
// that would otherwise hide a non-idempotent Run, so the set below is deliberately
// small and every member is a compose bring-up.
type ForcedSteps map[string]bool

// RebuildForcedSteps returns the steps --rebuild forces for a mode, or nothing for a
// mode that has none.
//
// mode:observe is absent on purpose rather than by omission: it builds the
// noc-backend image and composes the stack up unconditionally on every apply, with
// no Check to satisfy, so it is already what --rebuild asks other modes to become.
func RebuildForcedSteps(mode string) ForcedSteps {
	switch mode {
	case "found":
		// The central bank's own service images and operator portals. Both Runs are
		// `docker compose up -d --build` against a per-entity project, which rebuilds
		// from source and lets compose recreate the containers on the new image id.
		return ForcedSteps{
			StepStartCBBackend:  true,
			StepStartCBFrontend: true,
		}
	case "join":
		// The same two for a commercial bank, under this mode's step names.
		return ForcedSteps{
			StepStartBackend:      true,
			StepStartBankFrontend: true,
		}
	default:
		return nil
	}
}
