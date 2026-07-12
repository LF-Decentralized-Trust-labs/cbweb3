package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// cbFrontendImage is the per-entity frontend image tag for a CB operator portal
// (governance/treasury/supervisor). The api-gateway URL is baked into the SPA at
// build time, so the tag encodes the gateway port to keep each entity's image
// distinct (e.g. cbweb3b/governance-frontend:gw16645).
func cbFrontendImage(app string, gatewayPort int) string {
	return fmt.Sprintf("cbweb3b/%s-frontend:gw%d", app, gatewayPort)
}

// nocImages are the NOC observability service images, built before `docker
// compose` for the noc stack. The Go services (noc-backend/noc-agent) build from
// their own service dir (self-contained go.mod/go.sum); the portal is a Node app
// built from the frontend mono dir. Shared by hub and spoke.
var nocImages = []struct{ image, dockerfile, context string }{
	{hubNocBackendImage, "backend/services/noc-backend/Dockerfile", "backend/services/noc-backend"},
	{hubNocAgentImage, "backend/services/noc-agent/Dockerfile", "backend/services/noc-agent"},
	{hubNocPortalImage, "frontend/apps/noc/Dockerfile", "frontend"},
}

// imageExists reports whether a local image is present (via the runner).
func imageExists(ctx context.Context, r exec.CommandRunner, image string) bool {
	_, err := r.Run(ctx, "docker", "image", "inspect", image)
	return err == nil
}

// buildImageIn builds image from dockerfileRel within contextRel (both relative
// to scenarioBDir), skipping when the image already exists. Shared by hub, spoke
// and join so every entity builds its soft service images before `compose up`
// (otherwise compose tries to PULL a local-only tag and fails). The context must
// match what the Dockerfile expects: Node apps build from the `frontend` mono
// dir; each Go service (noc-backend/noc-agent) builds from its OWN service dir,
// which carries the go.mod/go.sum the Dockerfile COPYs.
func buildImageIn(ctx context.Context, r exec.CommandRunner, scenarioBDir, image, dockerfileRel, contextRel string) error {
	if imageExists(ctx, r, image) {
		return nil
	}
	_, err := r.Run(ctx, "docker", "build", "-t", image,
		"-f", filepath.Join(scenarioBDir, dockerfileRel), filepath.Join(scenarioBDir, contextRel))
	return err
}

// buildFrontendImage builds a frontend SPA (frontend/apps/<app>) with per-entity
// VITE_* build args baked in (the SPA reads its API URL at BUILD time), skipping
// when the per-entity-tagged image already exists. The image tag must encode the
// entity (the API URL is baked), so each entity gets its own image. buildArgs maps
// the app's VITE arg names to values (e.g. governance→VITE_API_URL, treasury/
// supervisor→VITE_API_BASE_URL); all frontends serve on container port 80 (nginx).
func buildFrontendImage(ctx context.Context, r exec.CommandRunner, scenarioBDir, image, app string, buildArgs map[string]string) error {
	if imageExists(ctx, r, image) {
		return nil
	}
	args := []string{"build", "-t", image, "-f",
		filepath.Join(scenarioBDir, "frontend/apps/"+app+"/Dockerfile")}
	// Deterministic arg order for reproducible builds / stable test expectations.
	keys := make([]string, 0, len(buildArgs))
	for k := range buildArgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--build-arg", k+"="+buildArgs[k])
	}
	args = append(args, filepath.Join(scenarioBDir, "frontend"))
	_, err := r.Run(ctx, "docker", args...)
	return err
}
