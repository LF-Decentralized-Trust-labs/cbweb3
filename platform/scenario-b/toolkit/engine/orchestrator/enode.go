// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// EnodeReader returns the node's enode URL. Injectable so found-spoke is
// testable without a live Besu.
type EnodeReader func(ctx context.Context, rpcURL string) (string, error)

// adminNodeInfoEnode queries admin_nodeInfo over JSON-RPC and returns the enode.
func adminNodeInfoEnode(ctx context.Context, rpcURL string) (string, error) {
	body := []byte(`{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Result struct {
			Enode string `json:"enode"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Result.Enode == "" {
		return "", fmt.Errorf("admin_nodeInfo returned empty enode from %s", rpcURL)
	}
	return out.Result.Enode, nil
}

// privateDockerIP matches loopback (127.x) and the Docker default-bridge range
// (172.16–31.x) — neither is reachable by another stack, so an enode built from
// them is unusable as a cross-stack bootnode (mirrors scenario-a's guard).
var privateDockerIP = regexp.MustCompile(`(^|@)(127\.0\.0\.1|172\.(1[6-9]|2[0-9]|3[01])\.)`)

// resolveHostIP returns the host's primary LAN IP so the spoke bundle enode is a
// routable numeric address. Mirrors scenario-a's resolve-host-ip.sh: Besu 25.8.0
// reports 127.0.0.1 in admin_nodeInfo regardless of --nat-method, and it rejects
// hostnames in --bootnodes, so a joining bank on the same host must dial the CB's
// published P2P port via the LAN IP. Override with HOST_IP. The UDP dial sends no
// packets; it just picks the source IP of the default route.
func resolveHostIP() (string, error) {
	if v := os.Getenv("HOST_IP"); v != "" {
		return v, nil
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	ip := conn.LocalAddr().(*net.UDPAddr).IP.String()
	if privateDockerIP.MatchString(ip) {
		return "", fmt.Errorf("resolved host IP %q is loopback/docker-internal and not reachable cross-stack; set HOST_IP", ip)
	}
	return ip, nil
}

// rewriteEnodeHost replaces the host:port of an enode URL with host:port,
// preserving the public-key part and any query string (e.g. ?discport=0).
// admin_nodeInfo advertises the container-local address (127.0.0.1:30303),
// which no other stack can dial; the spoke bundle must instead carry the
// externally reachable endpoint (advertisedHost + the CB's published P2P port)
// so a joining bank can use it as its --bootnodes. Returns the input unchanged
// if it is not a parseable enode.
// enodeNodeID returns the node public key of an enode URL ("enode://<id>@host:port"),
// i.e. its stable node identity independent of the advertised endpoint. Empty when
// the URL has no recognizable id. Used to detect a bundle whose bootnode belongs to
// a node that no longer exists (spoke re-founded with a fresh key).
func enodeNodeID(enode string) string {
	at := strings.LastIndex(enode, "@")
	if at < 0 {
		return ""
	}
	return strings.TrimPrefix(enode[:at], "enode://")
}

func rewriteEnodeHost(enode, host string, port int) string {
	at := strings.LastIndex(enode, "@")
	if at < 0 {
		return enode
	}
	prefix := enode[:at+1] // "enode://<pubkey>@"
	rest := enode[at+1:]   // "<host>:<port>[?query]"
	query := ""
	if q := strings.IndexByte(rest, '?'); q >= 0 {
		query = rest[q:]
	}
	return fmt.Sprintf("%s%s:%d%s", prefix, host, port, query)
}
