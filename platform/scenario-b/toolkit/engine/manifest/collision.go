// SPDX-License-Identifier: Apache-2.0

package manifest

import "fmt"

// ValidateSet checks cross-manifest invariants over a set of manifests
// (FR-008): chainId↔network consistency, unique declared node ports, unique
// founding spoke.id and unique metadata.name. Every collision finding names
// BOTH conflicting manifests.
//
// Only DECLARED values are checked. This phase does not model any
// derived-offset port scheme — that belongs to a later phase — so no ports
// beyond the ones written in the manifest are considered.
//
// A manifest identifies a network: a hub manifest declares the "hub" network;
// a found-spoke or join manifest declares (or joins) the spoke network keyed by
// spoke.id. A joining bank legitimately shares its spoke's chainId and spoke.id
// with the founding central bank, so those are NOT collisions — a collision is
// the same chainId claimed by two DIFFERENT networks (or two founders of the
// same spoke).
func ValidateSet(set []*ParticipantDeployment) Result {
	var r Result

	// metadata.name uniqueness.
	firstByName := map[string]string{}
	for _, pd := range set {
		name := pd.Metadata.Name
		if name == "" {
			continue
		}
		id := manifestID(pd)
		if prev, ok := firstByName[name]; ok {
			r.AddError("metadata.name", fmt.Sprintf("duplicate metadata.name %q shared by manifests %q and %q", name, prev, id))
			continue
		}
		firstByName[name] = id
	}

	// chainId ↔ network consistency: a chainId may be reused only by manifests
	// on the SAME network (same networkKey). Two different networks claiming the
	// same chainId is a collision.
	type origin struct {
		id      string
		field   string
		network string
	}
	firstByChain := map[int]origin{}
	for _, pd := range set {
		id := manifestID(pd)
		net := networkKey(pd)
		for _, o := range chainIDs(pd) {
			prev, ok := firstByChain[o.value]
			if !ok {
				firstByChain[o.value] = origin{id: id, field: o.field, network: net}
				continue
			}
			if prev.network != net || net == "" {
				r.AddError("chainId", fmt.Sprintf("chainId %d collides between manifests %q (%s) and %q (%s)",
					o.value, prev.id, prev.field, id, o.field))
			}
		}
	}

	// Declared node port uniqueness (rpc/ws/p2p). Each node is a distinct
	// container/host binding, so ports must be globally unique even for nodes on
	// the same network.
	firstByPort := map[int]origin{}
	for _, pd := range set {
		id := manifestID(pd)
		for _, o := range declaredPorts(pd) {
			if prev, ok := firstByPort[o.value]; ok {
				r.AddError("node.port", fmt.Sprintf("port %d collides between manifests %q (%s) and %q (%s)",
					o.value, prev.id, prev.field, id, o.field))
				continue
			}
			firstByPort[o.value] = origin{id: id, field: o.field}
		}
	}

	// spoke.id founder uniqueness: a spoke may be founded by exactly one
	// found-spoke manifest. (join manifests reuse an existing spoke.id and are
	// not counted here.)
	firstFounderSpoke := map[string]string{}
	for _, pd := range set {
		if pd.Spec.Mode != ModeFoundSpoke || pd.Spec.Spoke == nil || pd.Spec.Spoke.ID == "" {
			continue
		}
		sid := pd.Spec.Spoke.ID
		id := manifestID(pd)
		if prev, ok := firstFounderSpoke[sid]; ok {
			r.AddError("spoke.id", fmt.Sprintf("spoke.id %q is founded by more than one manifest: %q and %q", sid, prev, id))
			continue
		}
		firstFounderSpoke[sid] = id
	}

	return r
}

type chainOrigin struct {
	value int
	field string
}

func chainIDs(pd *ParticipantDeployment) []chainOrigin {
	var out []chainOrigin
	if pd.Spec.Hub != nil {
		out = append(out, chainOrigin{value: pd.Spec.Hub.ChainID, field: "hub.chainId"})
	}
	if pd.Spec.Spoke != nil {
		out = append(out, chainOrigin{value: pd.Spec.Spoke.ChainID, field: "spoke.chainId"})
	}
	return out
}

func declaredPorts(pd *ParticipantDeployment) []chainOrigin {
	n := pd.Spec.Node
	if n == nil {
		return nil
	}
	var out []chainOrigin
	if n.RPC != nil {
		out = append(out, chainOrigin{value: n.RPC.Port, field: "node.rpc.port"})
	}
	if n.WS != nil {
		out = append(out, chainOrigin{value: n.WS.Port, field: "node.ws.port"})
	}
	if n.P2P != nil {
		out = append(out, chainOrigin{value: n.P2P.Port, field: "node.p2p.port"})
	}
	return out
}

// networkKey returns the identity of the network a manifest declares or joins.
// Hub manifests all reference the single "hub" network; spoke manifests
// reference the spoke keyed by spoke.id. An empty key means the network cannot
// be determined (treated as always-colliding to surface the problem).
func networkKey(pd *ParticipantDeployment) string {
	if pd.Spec.Hub != nil || pd.Spec.Mode == ModeFoundHub {
		return "hub"
	}
	if pd.Spec.Spoke != nil && pd.Spec.Spoke.ID != "" {
		return "spoke:" + pd.Spec.Spoke.ID
	}
	return ""
}

// manifestID returns a stable label for a manifest used in collision messages.
func manifestID(pd *ParticipantDeployment) string {
	if pd.Metadata.Name != "" {
		return pd.Metadata.Name
	}
	return "<unnamed>"
}
