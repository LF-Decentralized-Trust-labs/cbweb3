// SPDX-License-Identifier: Apache-2.0

package privacy

import (
	"context"
	"testing"
)

const zeroHash = "0x0000000000000000000000000000000000000000000000000000000000000000"

func TestPaladinBypass(t *testing.T) {
	t.Parallel()
	var p PaladinBypass
	ctx := context.Background()

	if h, err := p.MintNoto(ctx, "to", "100"); err != nil || h != zeroHash {
		t.Errorf("MintNoto = (%q, %v)", h, err)
	}
	if h, err := p.TransferZeto(ctx, "from", "to", "100"); err != nil || h != zeroHash {
		t.Errorf("TransferZeto = (%q, %v)", h, err)
	}
	if h, err := p.CreateNotoHTLC(ctx, "a", "b", "c"); err != nil || h != zeroHash {
		t.Errorf("CreateNotoHTLC = (%q, %v)", h, err)
	}
	if h, err := p.ClaimNotoHTLC(ctx, "id", "secret"); err != nil || h != zeroHash {
		t.Errorf("ClaimNotoHTLC = (%q, %v)", h, err)
	}
}
