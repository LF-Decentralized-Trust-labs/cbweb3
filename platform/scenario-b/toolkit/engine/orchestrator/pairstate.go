package orchestrator

import (
	"context"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// pairID is the deterministic sovereign-pair id: "W-<A>-<B>".
func pairID(symbolA, symbolB string) string {
	return "W-" + symbolA + "-" + symbolB
}

// pairStatus reads PairRegistry.getPair(pairId) via `cast call` and reports the
// on-chain status ("PROPOSED"/"ACTIVE") and whether the pair exists. A revert or
// empty result means the pair does not exist yet. It goes through the injectable
// CommandRunner so found-spoke's sovereign tail is testable without a live chain.
func pairStatus(ctx context.Context, runner exec.CommandRunner, rpcURL, pairRegistry, id string) (status string, exists bool, err error) {
	out, rerr := runner.Run(ctx, "cast", "call", pairRegistry, "getPair(string)", id, "--rpc-url", rpcURL)
	if rerr != nil {
		// A revert (pair not found) is not a hard error: the pair simply does not exist.
		return "", false, nil
	}
	s := strings.ToUpper(string(out))
	switch {
	case strings.Contains(s, "ACTIVE"):
		return "ACTIVE", true, nil
	case strings.Contains(s, "PROPOSED"):
		return "PROPOSED", true, nil
	case strings.TrimSpace(s) == "":
		return "", false, nil
	default:
		// Numeric enum encodings: getPair returns the status enum; 1 == ACTIVE, 0 == PROPOSED.
		if strings.Contains(s, "\n1") || strings.HasSuffix(strings.TrimSpace(s), "1") {
			return "ACTIVE", true, nil
		}
		return "PROPOSED", true, nil
	}
}
