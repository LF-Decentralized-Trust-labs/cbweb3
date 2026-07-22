// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
)

// besuContainerP2PPort is the fixed in-container Besu devp2p port. On a single
// host all spoke nodes share one Docker network and peer on this port; the
// host-published P2P port (manifest node.p2p.port) is only for host access and is
// irrelevant for in-network peering.
const besuContainerP2PPort = 30303

// localizedEndpoints holds CB endpoints from the join bundle adapted for a
// single-host local deployment (feature 018). The bundle is built from the CB's
// in-Docker advertised host; the host-run toolkit and the separately-networked
// bank cannot use those names/ports directly.
type localizedEndpoints struct {
	// validators are reached by the host-run toolkit (vote-qbft) at localhost:<published rpc>.
	validators []bundle.ValidatorSpec
	// bootnodeEnode peers over the shared spoke network on the container-internal P2P port.
	bootnodeEnode string
	// cbCertEndpoint is the CB credential-request URL the host-run toolkit POSTs to.
	cbCertEndpoint string
	// cbAPIBaseForBank is the CB api-gateway base URL the bank's backend container uses.
	cbAPIBaseForBank string
}

// rawEndpoints returns the bundle endpoints unmodified (non-local profiles, where
// the advertised host is already routable from the joining host).
func rawEndpoints(b *bundle.JoinBundle) localizedEndpoints {
	return localizedEndpoints{
		validators:       b.Spec.Validators,
		bootnodeEnode:    b.Spec.Bootnode.Enode,
		cbCertEndpoint:   b.Spec.CBEndpoint,
		cbAPIBaseForBank: centralBankAPIURL(b.Spec.CBEndpoint),
	}
}

// localizeBundleEndpoints adapts the bundle for a single-host local deployment:
//   - the bank's Besu peers with the CB over the shared spoke network using the
//     container-internal P2P port (the advertised host resolves via a network alias);
//   - the host-run toolkit reaches the CB's JSON-RPC and api-gateway via published
//     host ports on localhost;
//   - the bank's backend container reaches the CB api-gateway via host.docker.internal.
//
// The CB's published Besu RPC port is read from the bundle's validator RPC URL, and
// the CB api-gateway host port is derived from it (entityPorts band).
func localizeBundleEndpoints(b *bundle.JoinBundle) (localizedEndpoints, error) {
	out := rawEndpoints(b)
	if len(b.Spec.Validators) == 0 {
		return out, fmt.Errorf("localize: bundle has no validators")
	}
	cbBesuRPCPort, err := portFromURL(b.Spec.Validators[0].RPCURL)
	if err != nil {
		return out, fmt.Errorf("localize: read CB besu rpc port: %w", err)
	}
	cbAPIPort := entityPorts(cbBesuRPCPort).APIGateway

	vs := make([]bundle.ValidatorSpec, len(b.Spec.Validators))
	for i, v := range b.Spec.Validators {
		u, err := rewriteURLHost(v.RPCURL, "localhost", 0)
		if err != nil {
			return out, fmt.Errorf("localize validator %d: %w", i, err)
		}
		vs[i] = bundle.ValidatorSpec{Address: v.Address, RPCURL: u}
	}
	out.validators = vs

	enode, err := setEnodePort(b.Spec.Bootnode.Enode, besuContainerP2PPort)
	if err != nil {
		return out, fmt.Errorf("localize enode: %w", err)
	}
	out.bootnodeEnode = enode

	// The CB's onboarding HTTP endpoints are only rewritten to the single-host
	// aliases when the bundle carries an in-Docker CB endpoint (a compose service
	// name on the in-container port). In a cross-VM deploy (environment: local but
	// the CB and the joining bank on separate hosts) the founding CB bakes its
	// routable host and host-published api-gateway port into the bundle; both
	// "localhost" (host-run toolkit) and "host.docker.internal" (bank backend
	// container) would then loop back to the joining host — the bank's own gateway
	// — instead of reaching the CB, so the onboarding request never lands at the
	// CB (my-status returns "no onboarding request found"). Keep the routable
	// endpoints from rawEndpoints in that case. This mirrors scenario-b, which
	// bakes a routable CBGateway into the join bundle and consumes it verbatim.
	if isCrossHostCBEndpoint(b.Spec.CBEndpoint) {
		return out, nil
	}

	cert, err := rewriteURLHost(b.Spec.CBEndpoint, "localhost", cbAPIPort)
	if err != nil {
		return out, fmt.Errorf("localize cb endpoint: %w", err)
	}
	out.cbCertEndpoint = cert

	apiBase, err := rewriteURLHost(centralBankAPIURL(b.Spec.CBEndpoint), "host.docker.internal", cbAPIPort)
	if err != nil {
		return out, fmt.Errorf("localize cb api base: %w", err)
	}
	out.cbAPIBaseForBank = apiBase

	return out, nil
}

// isCrossHostCBEndpoint reports whether the CB endpoint in the join bundle is a
// cross-host (routable) endpoint that must be used verbatim, versus an in-Docker
// single-host endpoint (a compose service name on the in-container port) that
// must be localized to localhost / host.docker.internal.
//
// The discriminator is a non-loopback numeric IP host: the founding CB bakes a
// routable IP (from node.advertisedHost, or the resolved host LAN IP — same path
// as the enode rewrite) when the deployment spans hosts, whereas the canonical
// single-host bundle carries a Docker compose service name (never an IP). A CB
// host given as a DNS name is treated as single-host (localized); express the CB
// as a routable IP to deploy across hosts.
func isCrossHostCBEndpoint(cbEndpoint string) bool {
	u, err := url.Parse(cbEndpoint)
	if err != nil {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && !ip.IsLoopback()
}

// portFromURL extracts the numeric port from a URL.
func portFromURL(raw string) (int, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return 0, err
	}
	p := u.Port()
	if p == "" {
		return 0, fmt.Errorf("no port in %q", raw)
	}
	return strconv.Atoi(p)
}

// rewriteURLHost replaces the host of a URL. When newPort > 0 it also replaces the
// port; otherwise the existing port is preserved. Path/query are kept.
func rewriteURLHost(raw, newHost string, newPort int) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	port := u.Port()
	if newPort > 0 {
		port = strconv.Itoa(newPort)
	}
	if port != "" {
		u.Host = net.JoinHostPort(newHost, port)
	} else {
		u.Host = newHost
	}
	return u.String(), nil
}

// isRoutableHost reports whether h is an externally routable host (a real IP or
// DNS name) rather than a loopback / docker-internal name that only resolves on
// the same Docker host. It gates the cross-VM Paladin transport wiring: when the
// CB advertised host is routable, the CB exposes its Paladin gRPC on the fixed
// peer port 9000 and joining banks add an extra_hosts entry pointing the CB
// Paladin's on-chain hostname (its container name) at that routable host.
func isRoutableHost(h string) bool {
	switch strings.ToLower(strings.TrimSpace(h)) {
	case "", "localhost", "127.0.0.1", "::1", "host.docker.internal", "host-gateway":
		return false
	}
	return true
}

// paladinDialHost returns the host a peer publishes on-chain as its Paladin gRPC
// transport endpoint (dns:///<host>:9000), and that other nodes dial to reach it.
//
// Paladin's reliable transport is bidirectional: resolving a remote verifier is a
// request AND a reply, so BOTH nodes must be able to dial each other's registered
// endpoint. On a single host every node's container name resolves over the shared
// spoke Docker network, so the container hostname works in both directions. Across
// VMs it does not — and giving only the joining bank an extra_hosts entry for the
// CB (the one-directional routable override) fixes the request leg but not the CB's
// reply leg, so create-pente-context hangs. Advertising the node's own routable
// host instead makes the endpoint dialable from any host with no per-peer
// extra_hosts; the host is added to the transport cert SAN (see addTransportSAN)
// so the dns:/// TLS handshake still validates. Non-routable (single-host)
// advertisedHost keeps the container hostname — unchanged behavior.
func paladinDialHost(advertisedHost, containerHostname string) string {
	if isRoutableHost(advertisedHost) {
		return strings.TrimSpace(advertisedHost)
	}
	return containerHostname
}

// setEnodePort replaces the port in an enode URI (enode://<id>@<host>:<port>),
// preserving the host (which resolves on the shared spoke network).
func setEnodePort(enode string, port int) (string, error) {
	atIdx := strings.LastIndex(enode, "@")
	if atIdx < 0 {
		return "", fmt.Errorf("enode missing @ separator: %q", enode)
	}
	hostPort := enode[atIdx+1:]
	host := hostPort
	if i := strings.LastIndex(hostPort, ":"); i >= 0 {
		host = hostPort[:i]
	}
	return fmt.Sprintf("%s@%s:%d", enode[:atIdx], host, port), nil
}
