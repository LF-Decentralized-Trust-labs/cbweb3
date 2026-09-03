// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/addrs"
)

// DeployedAddrs holds the contract addresses written by the deploy Go test scripts.
// Read from <SPOKE_DATA_DIR>/.deployed-addrs.env (key=value format, one per line).
type DeployedAddrs = addrs.DeployedAddrs

// parseDeployedAddrs reads a KEY=VALUE env file from path and populates DeployedAddrs.
// Returns an empty DeployedAddrs (not an error) when the file does not exist.
func parseDeployedAddrs(path string) (DeployedAddrs, error) {
	return addrs.ParseDeployedAddrs(path)
}

// addrsAppend appends a KEY=value line to the deployed-addrs env file at path.
func addrsAppend(path, key, value string) error {
	return addrs.AppendAddr(path, key, value)
}
