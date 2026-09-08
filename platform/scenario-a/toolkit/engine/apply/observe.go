// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/dockervolume"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
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
	// Single source of truth lives in the dockervolume package.
	nocAgentVolHelperImage = dockervolume.HelperImage
)

// nocKeycloakClientID is the realm client the NOC portal's operators authenticate with.
// Scenario A names it cbweb3-noc (Scenario B calls its own noc-portal — a naming drift, not
// a behavioural one).
const nocKeycloakClientID = "cbweb3-noc"

// nocPortalOrigins resolves the browser origins allowed to call this backend with
// credentials.
//
// Behind the proxy the portal is same-origin with the backend, so there is exactly one.
// Otherwise the manifest names them (spec.noc.portalOrigins), because this backend is
// shared by every CB portal on the host and only the operator knows which are deployed.
// Falls back to the founding CB's own portal convention when the manifest is silent, so an
// existing single-CB manifest keeps working rather than failing to start.
func nocPortalOrigins(m *manifest.Manifest, frontendHost string, proxyEnabled bool) string {
	if proxyEnabled {
		return orchestrator.ProxyOrigin(frontendHost)
	}
	// Nil-checked: spec.noc is optional, so a manifest without the block is normal and
	// must fall through to the convention rather than crash the apply.
	if m.Spec.NOC != nil && len(m.Spec.NOC.PortalOrigins) > 0 {
		return strings.Join(m.Spec.NOC.PortalOrigins, ",")
	}
	// The single-CB local convention: the NOC portal published by the founding CB.
	return fmt.Sprintf("http://%s:%d", frontendHostOrLocal(frontendHost), nocPortalPortLocal)
}

// nocPortalPortLocal is the NOC portal's published port under the single-host convention
// (the founding CB's RPC port + the NOC frontend offset). Scenario A's samples fix the CB at
// 8645, so the portal lands on 32645 — the value the observe manifest documents.
const nocPortalPortLocal = 32645

// containerReachableURL rewrites a host-facing "localhost" URL into one a CONTAINER can
// reach.
//
// spec.noc.keycloakURL was written for the browser, which is where the password grant used
// to run — and "localhost" is exactly right there. The grant now runs in the NOC backend,
// inside a container, where "localhost" is the container itself. Left alone, every existing
// manifest would break at login with a connection refused and nothing pointing at the cause.
func containerReachableURL(u string) string {
	for _, local := range []string{"//localhost:", "//127.0.0.1:"} {
		if strings.Contains(u, local) {
			return strings.Replace(u, local, "//host.docker.internal:", 1)
		}
	}
	return u
}

// nocCSRFSecretEnv is the variable the noc-stack template reads the secret from, and the
// one an operator sets to supply their own.
const nocCSRFSecretEnv = "NOC_CSRF_SECRET"

// nocCSRFSecretFile / nocCSRFSecretVolume locate the CSRF secret persisted for one NOC
// stack.
//
// A named volume, not a host file: this toolkit keeps secret material in volumes on purpose
// (package dockervolume pipes content straight from memory, so it never lands on the host
// disk even transiently). observe provisions no chain node and therefore has no dataDir of
// its own, so the volume IS this stack's persistent store — the role .deployed-addrs.env
// plays for a founding CB.
const nocCSRFSecretFile = "csrf-secret"

func nocCSRFSecretVolume(prefix string) string { return prefix + "_noc_secrets" }

// nocCSRFSecretStore is the persistence resolveNOCCSRFSecret needs, injected so the
// decision logic is testable without Docker.
type nocCSRFSecretStore struct {
	read  func(ctx context.Context, volume, filePath string) ([]byte, error)
	write func(ctx context.Context, volume, filePath string, content []byte, mode string) error
}

func defaultNOCCSRFSecretStore() nocCSRFSecretStore {
	return nocCSRFSecretStore{read: dockervolume.ReadFile, write: dockervolume.WriteFile}
}

// resolveNOCCSRFSecret returns the CSRF secret for this stack: the operator's if they set
// one, otherwise the value persisted for this stack, otherwise a fresh random value it
// persists before returning.
//
// RANDOM AND PERSISTED, not derived from the stack prefix. Derivation gave the property
// that matters — the same value after a restart, so a browser holding a good session does
// not start getting 403 on every action — but it bought it with a key anyone can recompute:
// the prefix is public (docker ps, the manifest, the volume names). The secret keys the HMAC
// that makes a CSRF token unforgeable, and that HMAC is the net for the case where CORS and
// the custom header have already failed (a sibling subdomain, a MITM on plain HTTP — and
// COOKIE_SECURE defaults to false locally). A computable key makes the net decoration, while
// the guard's own comment claims a token the server never issued cannot validate.
//
// The operator's value wins and is returned unchanged, so a deployment that manages its own
// secret is honoured rather than silently overridden.
func resolveNOCCSRFSecret(ctx context.Context, prefix string, st nocCSRFSecretStore) (string, error) {
	if v := strings.TrimSpace(os.Getenv(nocCSRFSecretEnv)); v != "" {
		return v, nil
	}
	vol := nocCSRFSecretVolume(prefix)
	b, err := st.read(ctx, vol, nocCSRFSecretFile)
	switch {
	case err == nil:
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, nil
		}
		// Present but empty: mint one rather than start the backend with a blank secret,
		// which it would replace with a per-process random value that dies on restart.
	case errors.Is(err, dockervolume.ErrNotFound):
		// First apply for this stack.
	default:
		return "", fmt.Errorf("read the NOC CSRF secret: %w", err)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate a NOC CSRF secret: %w", err)
	}
	secret := hex.EncodeToString(raw)
	if err := st.write(ctx, vol, nocCSRFSecretFile, []byte(secret), "0600"); err != nil {
		return "", fmt.Errorf("persist the NOC CSRF secret: %w", err)
	}
	return secret, nil
}

// nocKeycloakURL reads the optional realm URL, tolerating an absent spec.noc block.
func nocKeycloakURL(m *manifest.Manifest) string {
	if m == nil || m.Spec.NOC == nil {
		return ""
	}
	return strings.TrimSpace(m.Spec.NOC.KeycloakURL)
}

// frontendHostOrLocal mirrors the rest of the toolkit's default.
func frontendHostOrLocal(h string) string {
	if strings.TrimSpace(h) == "" {
		return "localhost"
	}
	return h
}

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
	// CORS origin(s). Mandatory, and never "*": the NOC session is a cookie now, and a
	// browser refuses to send credentials to a wildcard origin — a wildcard would let the
	// portal log in and then be anonymous on every request, with nothing to explain it.
	//
	// A LIST because one NOC backend is shared here: every CB on the host serves its own
	// NOC portal and they all point at this same port. Scenario B differs — its NOC stack
	// owns its portal — which is why this is a manifest field there and not here.
	env = append(env, "NOC_FRONTEND_ORIGIN="+nocPortalOrigins(m, fHost, proxyEnabled))
	// Realm for the backend's password grant. It moved here from the PORTAL's build args
	// when the login moved to the server: a cookie the browser cannot read can only be set
	// by a server, so the browser no longer talks to Keycloak.
	//
	// The client id goes out UNCONDITIONALLY. It is a constant of this scenario, not a
	// manifest value, and gating it on spec.noc.keycloakURL left the backend falling back to
	// the template's default — so NOC_SKIP_AUTH=false, which the template invites, would
	// have authenticated against the wrong client on every manifest that names no realm.
	env = append(env, "NOC_KEYCLOAK_CLIENT_ID="+nocKeycloakClientID)
	if kcURL := nocKeycloakURL(m); kcURL != "" {
		// Translated for the container: the manifest's "localhost" is the BROWSER's view,
		// and the grant now runs inside this backend.
		env = append(env, "NOC_KEYCLOAK_URL="+containerReachableURL(kcURL))
	}
	// The CSRF secret is resolved INSIDE the step so a failure to read or mint it is
	// reported as start-noc-stack's, next to the compose call it is configuration for.
	if err := steps.run(ctx, "start-noc-stack", func(ctx context.Context) error {
		secret, err := resolveNOCCSRFSecret(ctx, prefix, defaultNOCCSRFSecretStore())
		if err != nil {
			return err
		}
		// Appended only when the operator did NOT export their own. Appending
		// unconditionally would put two entries with the same key into one environment and
		// leave which one wins to the C library — with the operator's value the one at risk
		// of losing, which is the opposite of what the template promises.
		stackEnv := env
		if os.Getenv(nocCSRFSecretEnv) == "" {
			stackEnv = append(append([]string{}, env...), nocCSRFSecretEnv+"="+secret)
		}
		return runDocker(ctx, stackEnv, "compose", "-p", prefix, "-f", in.NOCStackComposePath, "up", "-d")
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
