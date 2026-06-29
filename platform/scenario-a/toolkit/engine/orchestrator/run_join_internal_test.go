// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// testJoinManifest returns a minimal valid manifest for mode:join.
func testJoinManifest(dataDir string) *manifest.Manifest {
	return &manifest.Manifest{
		APIVersion: "cbweb3/v1",
		Kind:       "ParticipantDeployment",
		Metadata:   manifest.Metadata{Name: "commercial-bank-alpha"},
		Spec: manifest.Spec{
			Scenario:      "a",
			Role:          "commercial-bank",
			Mode:          "join",
			Spoke:         manifest.Spoke{ID: "spoke-brl", ChainID: 1337, Currency: "BRL"},
			JoinBundleRef: "./bundles/spoke-brl.bundle.yaml",
			Node: manifest.Node{
				AdvertisedHost: "bank.host",
				RPC:            &manifest.Port{Port: 8746},
				WS:             &manifest.Port{Port: 8756},
				P2P:            &manifest.Port{Port: 31403},
				DataDir:        dataDir,
			},
			Image:       "hyperledger/besu:25.8.0",
			KeyProvider: "kms://local-emulator",
			CertSource:  "self-signed",
		},
	}
}

func testJoinBundle() *bundle.JoinBundle {
	return &bundle.JoinBundle{
		Spec: bundle.BundleSpec{
			SpokeID:    "spoke-brl",
			Genesis:    bundle.GenesisSpec{Hash: "sha256:x", Content: "eyJ9"},
			Bootnode:   bundle.BootnodeSpec{Enode: "enode://abc@h:1"},
			Contracts:  bundle.ContractsSpec{RegistryAddress: "0x1234"},
			Validators: []bundle.ValidatorSpec{{Address: "0xCB", RPCURL: "http://cb:8645"}},
			CBEndpoint: "http://cb:8080/cr",
		},
	}
}

func testJoinDeps() JoinDeps {
	return JoinDeps{
		KeyProvider:         keyprovider.NewLocalKeyProvider(),
		BankCode:            "commercial-bank-alpha",
		Institution:         "Alpha Bank",
		BesuRPCURL:          "http://localhost:8746",
		ComposeTemplatePath: "/nonexistent/docker-compose.yaml",
	}
}

func mockJoinSteps(checkVal bool) ([]Step, []*mockStep) {
	mocks := make([]*mockStep, len(CanonicalJoinStepOrder))
	steps := make([]Step, len(CanonicalJoinStepOrder))
	for i, name := range CanonicalJoinStepOrder {
		mocks[i] = &mockStep{name: name, checkVal: checkVal}
		steps[i] = mocks[i]
	}
	return steps, mocks
}

// T008: all steps already done → no Run is called.
func TestRunJoin_AllStepsDone(t *testing.T) {
	dataDir := t.TempDir()
	m := testJoinManifest(dataDir)
	steps, mocks := mockJoinSteps(true) // every Check returns done=true

	var buf bytes.Buffer
	if err := runJoinWithSteps(context.Background(), m, testJoinBundle(), testJoinDeps(), &buf, steps); err != nil {
		t.Fatalf("RunJoin: %v", err)
	}
	for _, mk := range mocks {
		if mk.runCalled != 0 {
			t.Errorf("step %s: Run called %d times; want 0 (already done)", mk.name, mk.runCalled)
		}
	}
}

// T008: all steps pending → all 9 run, state persisted as done.
func TestRunJoin_AllStepsSucceed(t *testing.T) {
	dataDir := t.TempDir()
	m := testJoinManifest(dataDir)
	steps, mocks := mockJoinSteps(false)

	var buf bytes.Buffer
	if err := runJoinWithSteps(context.Background(), m, testJoinBundle(), testJoinDeps(), &buf, steps); err != nil {
		t.Fatalf("RunJoin: %v", err)
	}
	for _, mk := range mocks {
		if mk.runCalled != 1 {
			t.Errorf("step %s: Run called %d times; want 1", mk.name, mk.runCalled)
		}
	}
	state, _ := LoadState(dataDir)
	if len(state.Steps) != len(CanonicalJoinStepOrder) {
		t.Fatalf("state has %d steps; want %d", len(state.Steps), len(CanonicalJoinStepOrder))
	}
	for _, s := range state.Steps {
		if s.Status != "done" {
			t.Errorf("step %s: status=%q; want done", s.Step, s.Status)
		}
	}
}

// T034: a partial state resumes from the first not-done step.
func TestRunJoin_ResumeFromStep5(t *testing.T) {
	dataDir := t.TempDir()
	m := testJoinManifest(dataDir)

	// Pre-seed state: first 4 steps done, rest pending.
	var state ProvisioningState
	state.SpokeID = "spoke-brl"
	for i := 0; i < 4; i++ {
		state = markStep(state, CanonicalJoinStepOrder[i], "done", "2026-06-27T00:00:00Z")
	}
	if err := saveState(dataDir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	steps, mocks := mockJoinSteps(false)
	// The first 4 steps report done via Check (already complete).
	for i := 0; i < 4; i++ {
		mocks[i].checkVal = true
	}

	var buf bytes.Buffer
	if err := runJoinWithSteps(context.Background(), m, testJoinBundle(), testJoinDeps(), &buf, steps); err != nil {
		t.Fatalf("RunJoin: %v", err)
	}
	for i := 0; i < 4; i++ {
		if mocks[i].runCalled != 0 {
			t.Errorf("step %s (already done): Run called %d times; want 0", mocks[i].name, mocks[i].runCalled)
		}
	}
	for i := 4; i < len(mocks); i++ {
		if mocks[i].runCalled != 1 {
			t.Errorf("step %s: Run called %d times; want 1 (resume)", mocks[i].name, mocks[i].runCalled)
		}
	}
}

func TestRunJoin_NilBundle(t *testing.T) {
	dataDir := t.TempDir()
	m := testJoinManifest(dataDir)
	var buf bytes.Buffer
	err := runJoinWithSteps(context.Background(), m, nil, testJoinDeps(), &buf, nil)
	if !errors.Is(err, ErrBundleNotFound) {
		t.Errorf("expected ErrBundleNotFound, got %v", err)
	}
}

// T038: lock is honoured — a held lock yields ErrProvisioningLocked.
func TestRunJoin_ProvisioningLocked(t *testing.T) {
	dataDir := t.TempDir()
	m := testJoinManifest(dataDir)
	unlock, err := lockState(dataDir)
	if err != nil {
		t.Fatalf("pre-lock: %v", err)
	}
	defer unlock()

	steps, _ := mockJoinSteps(false)
	var buf bytes.Buffer
	err = runJoinWithSteps(context.Background(), m, testJoinBundle(), testJoinDeps(), &buf, steps)
	if !errors.Is(err, ErrProvisioningLocked) {
		t.Errorf("expected ErrProvisioningLocked, got %v", err)
	}
}
