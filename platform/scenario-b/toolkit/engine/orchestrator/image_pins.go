// SPDX-License-Identifier: Apache-2.0

package orchestrator

// This file is the one place in the package allowed to write an image reference as a
// literal. image_pins_test.go enforces that by scanning the package for the reference and
// exempting this file by name.
//
// Ported from Scenario A (engine/orchestrator/step_start_besu_found.go plus the guards in
// engine/apply/image_pins_test.go), after the guard audit found the drift runs both ways:
// A had centralised its pins and guarded them, B had the same version string copied into
// four steps with nothing tying them together. See docs/guard-parity.md.
//
// Why a copy matters more here than duplication usually does: raising the Besu version is
// a single decision that has to land in every place at once. This project has already paid
// for a missed copy — the pin reached the toolkit but not the old deploy/local tree, and
// nothing failed; the two stacks simply ran different Besu versions until someone noticed.
//
// docs/TOOLCHAIN.md is the authority on the value. This constant is where the code reads it.

// DefaultBesuImage is the Besu image every step defaults to when the caller supplies none.
const DefaultBesuImage = "hyperledger/besu:25.8.0"
