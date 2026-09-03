// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"testing"
)

func newTestGenTLSStep(spokeID string) *genTLSStep {
	return &genTLSStep{spokeID: spokeID}
}

func TestGenTLSStep_Check_False_NoCert(t *testing.T) {
	requireDocker(t)
	step := newTestGenTLSStep("spoke-test-gentls-nocert")
	cleanupVolume(t, step.tlsVolume())

	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when central-bank.crt does not exist")
	}
}

func TestGenTLSStep_Check_True_CertExists(t *testing.T) {
	requireDocker(t)
	step := newTestGenTLSStep("spoke-test-gentls-exists")
	cleanupVolume(t, step.tlsVolume())

	if err := writeVolumeFile(context.Background(), step.tlsVolume(), "central-bank.crt", []byte("CERT"), "0644"); err != nil {
		t.Fatal(err)
	}
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when central-bank.crt exists")
	}
}

func TestGenTLSStep_Run_CreatesCertAndKey(t *testing.T) {
	requireDocker(t)
	step := newTestGenTLSStep("spoke-test-gentls-run")
	cleanupVolume(t, step.tlsVolume())
	cleanupVolume(t, step.paladinConfigVolume())

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	for _, f := range []string{"central-bank.crt", "central-bank.key"} {
		if _, err := readVolumeFile(context.Background(), step.tlsVolume(), f); err != nil {
			t.Errorf("%s not created in volume %s: %v", f, step.tlsVolume(), err)
		}
	}
	for _, f := range []string{"tls.crt", "tls.key"} {
		if _, err := readVolumeFile(context.Background(), step.paladinConfigVolume(), f); err != nil {
			t.Errorf("%s not created in volume %s: %v", f, step.paladinConfigVolume(), err)
		}
	}
}

func TestGenTLSStep_Run_CertIsPEM(t *testing.T) {
	requireDocker(t)
	step := newTestGenTLSStep("spoke-test-gentls-pem")
	cleanupVolume(t, step.tlsVolume())
	cleanupVolume(t, step.paladinConfigVolume())

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := readVolumeFile(context.Background(), step.tlsVolume(), "central-bank.crt")
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	if len(data) == 0 {
		t.Error("central-bank.crt is empty")
	}
	if string(data[:27]) != "-----BEGIN CERTIFICATE-----" {
		t.Errorf("central-bank.crt does not start with PEM header: %s", data[:27])
	}
}

func TestGenTLSStep_Run_Idempotent_Check_TrueAfterRun(t *testing.T) {
	requireDocker(t)
	step := newTestGenTLSStep("spoke-test-gentls-idempotent")
	cleanupVolume(t, step.tlsVolume())
	cleanupVolume(t, step.paladinConfigVolume())

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check after Run: %v", err)
	}
	if !done {
		t.Error("Check should return true after successful Run")
	}
}
