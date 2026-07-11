package orchestrator

import (
	"context"
	"path/filepath"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

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
