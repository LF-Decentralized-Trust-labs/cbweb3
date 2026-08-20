// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

const (
	// nocLocalAdminBearer is any non-empty bearer: the toolkit only provisions
	// LOCAL NOC backends running with NOC_SKIP_AUTH=true (NoOp Keycloak accepts
	// any token + grants ROLE_NOC_ADMIN). Production is provisioned out of band.
	nocLocalAdminBearer = "local-dev"
	// nocBackendImage is the local NOC backend image the observe deployment builds.
	nocBackendImage = "cbweb3/noc-backend:local"
	// nocBackendPort is the fixed host port the NOC backend publishes (per-VM; the
	// CB's co-located portal points its VITE_NOC_BACKEND_URL here). Single source of
	// truth lives in the orchestrator package (found bakes the same port).
	nocBackendPort = orchestrator.NOCBackendPort
	// nocAgentImage is the local NOC agent image the observe deployment builds.
	nocAgentImage = "cbweb3/noc-agent:local"
	// nocAgentVolHelperImage seeds the rendered agent.yaml into a named volume.
	nocAgentVolHelperImage = "alpine:3.20"
)

// observeStepOrder is the linear step set reported for mode:observe.
var observeStepOrder = []string{"build-noc-backend", "start-noc-stack", "wait-noc-backend", "register-noc-spoke", "provision-noc-key", "build-noc-agent", "start-noc-agent"}

// runObserveMode stands up the NOC control-plane data plane (db + backend) for a
// spoke, then registers the spoke + provisions the founding agent's key. The
// portal is deployed by the CB's found (co-located); agents run with each entity.
func runObserveMode(ctx context.Context, in ApplyInput) (ApplyResult, error) {
	m := in.Manifest
	result := ApplyResult{Mode: "observe", DryRun: in.DryRun}

	if in.NOCBundlePath == "" {
		result.Status, result.Error = "failed", "observe: nocBundleRef is required"
		return result, fmt.Errorf("%s", result.Error)
	}
	nb, err := bundle.LoadNOC(in.NOCBundlePath)
	if err != nil {
		result.Status, result.Error = "failed", fmt.Sprintf("observe: invalid NOC bundle %q: %v", in.NOCBundlePath, err)
		return result, err
	}
	result.Spoke = nb.SpokeID

	// Behind the per-host proxy (spec.proxy == enable) the NOC backend is routed same-origin
	// under /a/noc-api/ (so an HTTPS portal makes no blocked mixed-content call). The portal
	// route (/a/noc/) is owned by the CB's found; this deployment adds only the backend route.
	proxyEnabled := m.Spec.Proxy == "enable"
	fHost := m.Spec.FrontendHost

	if in.DryRun {
		result.Status = "dry-run"
		for _, n := range observeStepOrder {
			result.Steps = append(result.Steps, StepResult{Name: n, Status: "planned"})
			if n == "start-noc-stack" && proxyEnabled {
				result.Steps = append(result.Steps, StepResult{Name: "start-noc-proxy", Status: "planned"})
			}
		}
		return result, nil
	}

	prefix := "cbweb3-" + m.Metadata.Name // e.g. cbweb3-noc-brazil
	backendURL := fmt.Sprintf("http://localhost:%d", nocBackendPort)

	steps := &observeSteps{result: &result}

	// 1) Build the noc-backend image from its own service dir.
	backendDir := in.RepoRoot + "/backend/services/noc-backend"
	if err := steps.run(ctx, "build-noc-backend", func(ctx context.Context) error {
		return runDocker(ctx, nil, "build", "-t", nocBackendImage, "-f", backendDir+"/Dockerfile", backendDir)
	}); err != nil {
		return result, err
	}

	// 2) Bring up db + backend.
	env := append(os.Environ(),
		"NOC_PREFIX="+prefix,
		"NOC_DB_USER=noc",
		"NOC_DB_PASSWORD=noc",
		"NOC_DB_NAME=noc",
		"NOC_BACKEND_PORT="+strconv.Itoa(nocBackendPort),
		"NOC_BACKEND_IMAGE="+nocBackendImage,
		"NOC_NET_NAME="+prefix+"-net",
		"NOC_VOLUME_PREFIX="+prefix,
	)
	if proxyEnabled {
		// Backend CORS collapses to the single proxy origin (vs the local "*" default).
		env = append(env, "NOC_FRONTEND_ORIGIN="+orchestrator.ProxyOrigin(fHost))
	}
	if err := steps.run(ctx, "start-noc-stack", func(ctx context.Context) error {
		return runDocker(ctx, env, "compose", "-p", prefix, "-f", in.NOCStackComposePath, "up", "-d")
	}); err != nil {
		return result, err
	}

	// 2b) Behind the proxy: route the NOC backend same-origin under /a/noc-api/ and attach
	// the proxy to this observe network so it can reach the backend by name. Soft — a proxy
	// problem never fails observe. The portal route (/a/noc/) is written by the CB's found.
	if proxyEnabled {
		if err := steps.run(ctx, "start-noc-proxy", func(ctx context.Context) error {
			return orchestrator.NewNOCBackendProxyStep(m.Spec.Proxy, fHost,
				prefix+"-net", prefix+"-noc-backend:8080").Run(ctx)
		}); err != nil {
			return result, err
		}
	}

	// 3) Wait for the backend to be ready.
	if err := steps.run(ctx, "wait-noc-backend", func(ctx context.Context) error {
		return waitHTTPOK(ctx, backendURL+"/health", 90*time.Second)
	}); err != nil {
		return result, err
	}

	// 4) Register the spoke under its deterministic UUID (idempotent).
	if err := steps.run(ctx, "register-noc-spoke", func(ctx context.Context) error {
		body, _ := json.Marshal(map[string]any{
			"id": nb.SpokeUUID, "name": nb.Name, "currency_code": nb.CurrencyCode, "jurisdiction": nb.Jurisdiction,
		})
		return nocAdminPost(ctx, backendURL, "/api/v1/admin/spokes", body,
			http.StatusCreated, http.StatusOK, http.StatusConflict)
	}); err != nil {
		return result, err
	}

	// 5) Provision the founding CB agent's key (idempotent server-side).
	if err := steps.run(ctx, "provision-noc-key", func(ctx context.Context) error {
		body, _ := json.Marshal(map[string]string{
			"raw_key":  orchestrator.DeterministicAgentKey(nb.SpokeID, orchestrator.NOCFoundingAgentLabel),
			"spoke_id": nb.SpokeUUID,
			"hint":     nb.SpokeID + "-" + orchestrator.NOCFoundingAgentLabel,
		})
		return nocAdminPost(ctx, backendURL, "/api/v1/admin/agents/provision-key", body,
			http.StatusCreated, http.StatusOK)
	}); err != nil {
		return result, err
	}

	// 6) Build the CB's noc-agent image (co-located with this observe deployment).
	agentDir := in.RepoRoot + "/backend/services/noc-agent"
	if err := steps.run(ctx, "build-noc-agent", func(ctx context.Context) error {
		return runDocker(ctx, nil, "build", "-t", nocAgentImage, "-f", agentDir+"/Dockerfile", agentDir)
	}); err != nil {
		return result, err
	}

	// 7) Render the CB agent.yaml, seed it into a config volume, and start the
	// agent. It probes the bundle's components (host.docker.internal endpoints)
	// and pushes to the co-located backend; docker.sock lets it tail container logs.
	if err := steps.run(ctx, "start-noc-agent", func(ctx context.Context) error {
		agentCfg, err := orchestrator.RenderAgentYAML(nb, "http://host.docker.internal:"+strconv.Itoa(nocBackendPort),
			orchestrator.DeterministicAgentKey(nb.SpokeID, orchestrator.NOCFoundingAgentLabel), 15)
		if err != nil {
			return err
		}
		cfgVol := prefix + "_noc_agent_cfg"
		if err := seedVolumeFile(ctx, cfgVol, "agent.yaml", agentCfg); err != nil {
			return err
		}
		agentName := prefix + "-noc-agent"
		_ = runDocker(ctx, nil, "rm", "-f", agentName) // idempotent re-create
		return runDocker(ctx, nil, nocAgentRunArgs(agentName, cfgVol)...)
	}); err != nil {
		return result, err
	}

	result.Status = "success"
	return result, nil
}

// runJoinNOCAgent is a best-effort bring-up of a joining bank's noc-agent so the
// bank's OWN Besu node reports health + logs to the CB's (remote) NOC backend.
// Opt-in via spec.noc.backendURL (the founding CB VM's :8090). The bank registers
// under the SAME deterministic spoke UUID as the CB (a peer of the same spoke),
// but with its own deterministic agent key (label = bankId), provisioned against
// the remote admin API. Never fails the join — the node is a healthy peer without it.
func runJoinNOCAgent(ctx context.Context, in ApplyInput, spokeID, bankID string, besuRPCPort int) error {
	backendURL := strings.TrimRight(in.Manifest.Spec.NOC.BackendURL, "/")
	nb := bundle.NOCBundle{
		SpokeID:      spokeID,
		SpokeUUID:    orchestrator.DeterministicUUID(spokeID),
		Name:         spokeID,
		CurrencyCode: in.Manifest.Spec.Spoke.Currency,
		Jurisdiction: spokeID,
		Components: []bundle.NOCComponent{{
			Name:          "besu-" + bankID,
			Type:          "BESU",
			Endpoint:      fmt.Sprintf("http://host.docker.internal:%d", besuRPCPort),
			ContainerName: fmt.Sprintf("cbweb3-%s-besu.%s", spokeID, bankID),
		}},
	}
	key := orchestrator.DeterministicAgentKey(spokeID, bankID)

	// 1) Provision the bank's key on the remote CB backend (idempotent server-side).
	body, _ := json.Marshal(map[string]string{
		"raw_key":  key,
		"spoke_id": nb.SpokeUUID,
		"hint":     spokeID + "-" + bankID,
	})
	if err := nocAdminPost(ctx, backendURL, "/api/v1/admin/agents/provision-key", body,
		http.StatusCreated, http.StatusOK); err != nil {
		return err
	}

	// 2) Build the noc-agent image (co-located with the bank node).
	agentDir := in.RepoRoot + "/backend/services/noc-agent"
	if err := runDocker(ctx, nil, "build", "-t", nocAgentImage, "-f", agentDir+"/Dockerfile", agentDir); err != nil {
		return err
	}

	// 3) Render agent.yaml, seed it into a config volume, start the agent. It probes
	// the bank's own Besu (host.docker.internal) and pushes to the remote backend;
	// docker.sock lets it tail the bank node's container logs.
	interval := 15
	if in.Manifest.Spec.NOC.PushIntervalSeconds > 0 {
		interval = in.Manifest.Spec.NOC.PushIntervalSeconds
	}
	agentCfg, err := orchestrator.RenderAgentYAML(nb, backendURL, key, interval)
	if err != nil {
		return err
	}
	prefix := "cbweb3-" + in.Manifest.Metadata.Name // e.g. cbweb3-bank-cb1-brazil
	cfgVol := prefix + "_noc_agent_cfg"
	if err := seedVolumeFile(ctx, cfgVol, "agent.yaml", agentCfg); err != nil {
		return err
	}
	agentName := prefix + "-noc-agent"
	_ = runDocker(ctx, nil, "rm", "-f", agentName) // idempotent re-create
	return runDocker(ctx, nil, nocAgentRunArgs(agentName, cfgVol)...)
}

// nocAgentRunArgs builds the `docker run` argv for a noc-agent container. Shared by the
// founding CB and the joining bank so the two bring-ups cannot drift apart.
func nocAgentRunArgs(agentName, cfgVol string) []string {
	args := []string{"run", "-d", "--name", agentName,
		"-v", cfgVol + ":/etc/noc-agent:ro",
		"-v", dockerSocketPath + ":" + dockerSocketPath + ":ro",
		"--add-host", "host.docker.internal:host-gateway",
		"-e", "AGENT_CONFIG_PATH=/etc/noc-agent/agent.yaml"}
	// The agent image is non-root (uid 65532, finding R2-M-12) and the socket above is
	// root:docker 0660, so it needs that group to tail logs at all. Omitted when the gid
	// cannot be read: the agent then reports it at startup rather than silently
	// collecting no logs.
	if gid := dockerSocketGID(); gid != "" {
		args = append(args, "--group-add", gid)
	}
	return append(args, "--restart", "always", nocAgentImage)
}

// seedVolumeFile writes content into <volume>/<name> via a throwaway container
// (base64 over argv, no host file), so the agent.yaml lands in a named volume.
func seedVolumeFile(ctx context.Context, volume, name string, content []byte) error {
	b64 := base64.StdEncoding.EncodeToString(content)
	script := "printf %s '" + b64 + "' | base64 -d > /t/" + name
	return runDocker(ctx, nil, "run", "--rm", "-v", volume+":/t", nocAgentVolHelperImage, "sh", "-c", script)
}

// observeSteps records step outcomes into the result as each runs.
type observeSteps struct{ result *ApplyResult }

func (s *observeSteps) run(ctx context.Context, name string, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		s.result.Steps = append(s.result.Steps, StepResult{Name: name, Status: "failed", Error: err.Error()})
		s.result.Status, s.result.Error = "failed", fmt.Sprintf("%s: %v", name, err)
		return err
	}
	s.result.Steps = append(s.result.Steps, StepResult{Name: name, Status: "success"})
	return nil
}

// runDocker runs `docker <args>` with optional env, returning combined output on error.
func runDocker(ctx context.Context, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	if env != nil {
		cmd.Env = env
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

// nocAdminPost POSTs JSON to a NOC admin endpoint, treating okCodes as success.
func nocAdminPost(ctx context.Context, backendURL, path string, body []byte, okCodes ...int) error {
	url := strings.TrimRight(backendURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+nocLocalAdminBearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	for _, ok := range okCodes {
		if resp.StatusCode == ok {
			return nil
		}
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("POST %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
}

// waitHTTPOK polls url until it returns 2xx or the timeout elapses.
func waitHTTPOK(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("endpoint %s not ready within %s", url, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}
