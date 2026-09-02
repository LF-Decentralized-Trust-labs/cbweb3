// SPDX-License-Identifier: Apache-2.0

package orchestrator

import "testing"

// TEMPORARY — proves the CI gate rejects a broken toolkit test. Reverted in the
// commit that follows.
func TestZZGateProbe_DeliberateFailure(t *testing.T) {
	t.Fatal("deliberate failure: proving the toolkit CI gate rejects a broken test")
}
