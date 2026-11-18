// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
)

// BesuWSMaxActiveConnectionsVar is the operator knob for Besu's
// --rpc-ws-max-active-connections. Every Besu compose template reads it, and the toolkit
// writes the resolved value into the env it renders, so the ceiling in force is the one the
// toolkit validated rather than whatever the operator's shell happens to hold.
const BesuWSMaxActiveConnectionsVar = "BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS"

// DefaultBesuWSMaxActiveConnections replaces Besu's implicit 80, which was exhausted in
// production. 256 leaves room above the handful of legitimate clients (Paladin, relay, NOC,
// services), so a leak shows on besu_rpc_active_ws_connection_count well before it becomes an
// outage.
const DefaultBesuWSMaxActiveConnections = "256"

// Besu parses the option as a Java int; a larger value fails option parsing and the
// container restart-loops with a generic error.
const maxBesuWSMaxActiveConnections = math.MaxInt32

var positiveIntRe = regexp.MustCompile(`^[1-9][0-9]*$`)

// parseBesuWSMaxActiveConnections validates a ceiling: empty means the default; anything
// else must be an integer in [1, MaxInt32]. 0 would start a node that refuses every WebSocket
// client, and a typo would put Besu into a restart loop, so both are refused here, by name.
func parseBesuWSMaxActiveConnections(raw string) (string, error) {
	if raw == "" {
		return DefaultBesuWSMaxActiveConnections, nil
	}
	if !positiveIntRe.MatchString(raw) {
		return "", fmt.Errorf("%s must be a positive integer, got %q", BesuWSMaxActiveConnectionsVar, raw)
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n > maxBesuWSMaxActiveConnections {
		return "", fmt.Errorf("%s must be at most %d, got %q", BesuWSMaxActiveConnectionsVar, maxBesuWSMaxActiveConnections, raw)
	}
	return raw, nil
}

// besuWSMaxActiveConnections resolves the ceiling from the operator's environment.
func besuWSMaxActiveConnections() (string, error) {
	return parseBesuWSMaxActiveConnections(os.Getenv(BesuWSMaxActiveConnectionsVar))
}

// besuWSMaxActiveConnectionsOrEmpty is the resolved ceiling, or "" when the operator's value
// is invalid. ComposeEnv cannot return an error, so the start-besu steps validate first and
// fail with the message from besuWSMaxActiveConnections.
func besuWSMaxActiveConnectionsOrEmpty() string {
	v, err := besuWSMaxActiveConnections()
	if err != nil {
		return ""
	}
	return v
}
