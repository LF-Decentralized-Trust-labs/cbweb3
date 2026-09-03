// SPDX-License-Identifier: Apache-2.0

package app

import "strings"

// hubRegistryMode says how a hub contract client should be built for this entity.
type hubRegistryMode int

const (
	// hubRegistryDisabled: there is nothing to talk to — no contract address or no hub RPC.
	hubRegistryDisabled hubRegistryMode = iota
	// hubRegistryReadOnly: view calls only. The write methods on both registry adapters already
	// refuse a nil signer with an explicit error, so this mode cannot be escalated by accident.
	hubRegistryReadOnly
	// hubRegistryReadWrite: this entity signs its own hub transactions.
	hubRegistryReadWrite
)

func (m hubRegistryMode) String() string {
	switch m {
	case hubRegistryReadOnly:
		return "read-only"
	case hubRegistryReadWrite:
		return "read-write"
	default:
		return "disabled"
	}
}

// resolveHubRegistryMode decides whether to build a hub registry client, and with what powers.
//
// The distinction that matters here is between acting on the hub and looking at it. A commercial bank
// holds no hub signing key on purpose — the hub AMM admits only verified hub participants, so a bank's
// hub acts are delegated to its central bank. But the pair and currency registries are also the
// catalogue of what exists on the hub, and reading a catalogue is a view call.
//
// Requiring a signing key for the client conflated the two, and the consequence was silent: with no
// client the pair service falls back to this entity's own database, which never receives pair rows —
// they are written by the central banks that propose and confirm them. A funded corridor therefore
// rendered as an empty pool list on every bank portal, with no error anywhere to explain it.
func resolveHubRegistryMode(contractAddress, hubRPCURL, signerKey string) hubRegistryMode {
	if strings.TrimSpace(contractAddress) == "" || strings.TrimSpace(hubRPCURL) == "" {
		return hubRegistryDisabled
	}
	if strings.TrimSpace(signerKey) == "" {
		return hubRegistryReadOnly
	}
	return hubRegistryReadWrite
}
