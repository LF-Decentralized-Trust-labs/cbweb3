// SPDX-License-Identifier: Apache-2.0

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
// distinct (e.g. cbweb3b/governance-frontend:gw16645). variant distinguishes builds
// whose baked URLs differ under the same gateway port (e.g. "-proxy" for a
// base-path-aware build); pass "" for the default host-port build.
func cbFrontendImage(app string, gatewayPort int, variant string) string {
	return fmt.Sprintf("cbweb3b/%s-frontend:gw%d%s", app, gatewayPort, variant)
}

// forceImageRebuild makes imageExists report "missing" for the whole run, so the build
// steps run against the current source. Image tags encode the baked build args, not the
// source tree: without this, editing a service or portal leaves the tag unchanged and
// every build gate skips, producing an apply that reports "done" while still serving the
// previous binary. Set once per process from the CLI (--rebuild); the toolkit is a
// single-run CLI, so a package-level switch is enough and keeps the build helpers'
// signatures (used across hub/spoke/join/observe) untouched.
var forceImageRebuild bool

// SetForceImageRebuild enables/disables the forced-rebuild behaviour for this run.
func SetForceImageRebuild(force bool) { forceImageRebuild = force }

// imageExists reports whether a local image is present (via the runner). Always false
// under --rebuild, so callers rebuild instead of trusting an existing tag.
func imageExists(ctx context.Context, r exec.CommandRunner, image string) bool {
	if forceImageRebuild {
		return false
	}
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
