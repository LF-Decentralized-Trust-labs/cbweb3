// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/manifest"
)

// The hub provisioned operators nobody could use.
//
// found-hub takes spec.adminUsers, creates them in the realm, and the sample manifest declares
// two (admin@hub.governance.gov, admin@hub.noc.gov). The hub serves a governance portal. And
// every login answered 503 AUTH_SERVICE_UNAVAILABLE, because hub-backend.compose.yaml declared
// only an api-gateway — there was no auth service to ask.
//
// That error was CORRECT, which is part of why it survived: the gateway already distinguishes an
// unreachable auth service from a wrong password, and it was telling the truth. From outside, the
// realm looks right, the portal loads, the credentials are the documented ones, and the failure
// names a service the operator has no reason to know is missing.
//
// The template's header used to say the hub "needs no auth/login". That was a real design
// decision, and the tree had already left it behind on every other axis: the realm has a
// hub-backend client with a secret, and two operator accounts are created on every found-hub.
// Reversing it is recorded in ADR-011; these tests are what keep the three halves agreeing.

func hubBackendServices(t *testing.T) map[string]yaml.Node {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(templatesDir, "hub-backend.compose.yaml"))
	if err != nil {
		t.Fatalf("read hub-backend.compose.yaml: %v", err)
	}
	var doc struct {
		Services map[string]yaml.Node `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse hub-backend.compose.yaml: %v", err)
	}
	if len(doc.Services) == 0 {
		t.Fatal("hub-backend.compose.yaml declares no services; the guard would pass vacuously")
	}
	return doc.Services
}

func TestHubBackendDeclaresAnAuthService(t *testing.T) {
	if _, ok := hubBackendServices(t)["auth"]; !ok {
		t.Error("hub-backend.compose.yaml declares no auth service, but found-hub provisions " +
			"spec.adminUsers into the hub realm. Those operators cannot log in: the gateway has " +
			"nobody to ask and answers AUTH_SERVICE_UNAVAILABLE to every credential.")
	}
}

// The gateway must know where to find it. A running auth service the gateway cannot address is
// the same 503 with more containers.
//
// Parsed, not grepped: a substring check passes on a commented-out line, on a renamed key that
// still contains the old one, and on the variable appearing in prose. The first version of this
// test did exactly that and a mutation walked straight through it.
func TestHubGatewayPointsAtItsAuthService(t *testing.T) {
	svc, ok := hubBackendServices(t)["api-gateway"]
	if !ok {
		t.Fatal("hub-backend.compose.yaml declares no api-gateway")
	}
	var decoded struct {
		Environment map[string]string `yaml:"environment"`
	}
	if err := svc.Decode(&decoded); err != nil {
		t.Fatalf("decode api-gateway: %v", err)
	}

	addr, ok := decoded.Environment["AUTH_GRPC_ADDR"]
	if !ok {
		t.Fatal("the hub api-gateway has no AUTH_GRPC_ADDR; it would keep answering " +
			"AUTH_SERVICE_UNAVAILABLE with an auth service running beside it")
	}
	// It has to point at the auth container on this entity's network, not at something that
	// merely parses. localhost would resolve inside the gateway's own container and answer
	// nothing.
	if !strings.Contains(addr, "-auth:") {
		t.Errorf("AUTH_GRPC_ADDR is %q, which does not name this entity's auth container", addr)
	}
	if strings.Contains(addr, "localhost") || strings.Contains(addr, "127.0.0.1") {
		t.Errorf("AUTH_GRPC_ADDR is %q — that address is the gateway's own container", addr)
	}
}

// The three halves that have to agree, checked together. This is the guard the original defect
// needed: each half was individually defensible, and only their combination was wrong.
func TestHubOperatorsCanReachAnAuthService(t *testing.T) {
	m := hubSampleManifest(t)
	if len(m.Spec.AdminUsers) == 0 {
		t.Skip("the hub sample declares no operators; nothing to reach an auth service for")
	}
	if _, ok := hubBackendServices(t)["auth"]; !ok {
		t.Errorf("the hub sample declares %d operator(s) and found-hub provisions them, but the "+
			"hub backend has no auth service. Either wire one or stop declaring operators — the "+
			"state in between creates accounts that cannot be used and a login screen that "+
			"answers 503.", len(m.Spec.AdminUsers))
	}
}

// The image the compose file names must be one the toolkit actually builds, or `compose up`
// tries to PULL a local-only tag.
//
// Asserted on the STEP's behaviour, not by scanning the source for a Dockerfile path. The first
// version grepped, and the grep matched a second occurrence elsewhere in the file — so deleting
// the build left the test green. A source scan cannot tell which of two matches is the one that
// runs; the Check can.
func TestHubBuildsTheAuthImageItRuns(t *testing.T) {
	// A runner that reports every image as absent: the Check must then say "not satisfied", so
	// the Run that builds them happens.
	c := HubConfig{Runner: &imageMissingRunner{}}
	step := findHubStep(t, c, "build-hub-backend-image")

	ok, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if ok {
		t.Error("build-hub-backend-image reports satisfied with no images present, so the auth " +
			"image is never built and `compose up` would try to pull cbweb3b/auth:local")
	}

	// And the narrower case that a single-image Check would miss: the gateway present, auth not.
	c = HubConfig{Runner: &onlyImageRunner{present: hubBackendImage}}
	step = findHubStep(t, c, "build-hub-backend-image")
	if ok, _ := step.Check(context.Background()); ok {
		t.Error("the step reports satisfied when only the gateway image exists; a host in that " +
			"state skips the build and fails at compose up, pulling a tag nobody published")
	}
}

// The Check saying "not satisfied" is only half of it: the Run has to actually build both. A
// Check that gates on two images while the Run builds one leaves the step permanently
// unsatisfied — it runs on every apply and still fails at compose up.
//
// This is the assertion the first version of this file was missing. It gated on the Check alone,
// so deleting the auth build from the Run changed nothing it could see.
func TestHubBackendRunBuildsBothImages(t *testing.T) {
	runner := &imageMissingRunner{}
	step := findHubStep(t, HubConfig{Runner: runner}, "build-hub-backend-image")

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	built := map[string]bool{}
	for _, call := range runner.FakeRunner.Calls {
		if len(call.Args) >= 4 && call.Args[0] == "build" && call.Args[1] == "-t" {
			built[call.Args[2]] = true
		}
	}
	for _, want := range []string{hubAuthImage, hubBackendImage} {
		if !built[want] {
			t.Errorf("build-hub-backend-image never builds %s (built: %v). compose up would try "+
				"to pull it, and it is a local-only tag.", want, keysOfBool(built))
		}
	}
}

func keysOfBool(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// hubSampleManifest is the hub deployment this project ships. Read from the samples rather than
// constructed, because the point is what the toolkit will actually provision.
func hubSampleManifest(t *testing.T) *manifest.ParticipantDeployment {
	t.Helper()
	for name, m := range sampleManifests(t) {
		if strings.Contains(name, "hub") || m.Spec.Mode == manifest.ModeFoundHub {
			return m
		}
	}
	t.Fatal("no hub manifest among the samples; the guard cannot run")
	return nil
}

// findHubStep returns one step of the found-hub plan by name, failing if it is absent — a step
// that vanished would otherwise make the assertions above pass by not running.
func findHubStep(t *testing.T, c HubConfig, name string) Step {
	t.Helper()
	for _, s := range FoundHubSteps(c) {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("found-hub has no step %q", name)
	return Step{}
}

// imageMissingRunner answers "no such image" to every `docker image inspect`, and succeeds at
// everything else. imageExists reads only the error.
type imageMissingRunner struct{ exec.FakeRunner }

func (r *imageMissingRunner) RunWithEnv(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	if isImageInspect(args) {
		return nil, errors.New("Error: No such image")
	}
	return r.FakeRunner.RunWithEnv(ctx, env, name, args...)
}

func (r *imageMissingRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.RunWithEnv(ctx, nil, name, args...)
}

// onlyImageRunner reports exactly one image as present. It is the case a Check on a single
// image cannot distinguish from "everything is built".
type onlyImageRunner struct {
	exec.FakeRunner
	present string
}

func (r *onlyImageRunner) RunWithEnv(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	if isImageInspect(args) {
		if len(args) > 0 && args[len(args)-1] == r.present {
			return nil, nil
		}
		return nil, errors.New("Error: No such image")
	}
	return r.FakeRunner.RunWithEnv(ctx, env, name, args...)
}

func (r *onlyImageRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.RunWithEnv(ctx, nil, name, args...)
}

func isImageInspect(args []string) bool {
	return len(args) >= 2 && args[0] == "image" && args[1] == "inspect"
}
