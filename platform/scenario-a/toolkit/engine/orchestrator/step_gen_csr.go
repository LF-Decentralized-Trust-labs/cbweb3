// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/pki"
)

// genCSRStep generates the commercial bank's TLS keypair and CSR via the pki
// package. The private key never leaves SPOKE_DATA_DIR/pki/{bankCode}.key (0600)
// and is never transmitted; only the CSR is later sent to the central bank.
type genCSRStep struct {
	bankCode    string
	institution string
	dataDir     string
}

func newGenCSRStep(bankCode, institution, dataDir string) Step {
	return &genCSRStep{bankCode: bankCode, institution: institution, dataDir: dataDir}
}

func (s *genCSRStep) Name() string { return StepGenCSR }

func (s *genCSRStep) Check(_ context.Context) (bool, error) {
	_, err := os.Stat(filepath.Join(s.dataDir, "pki", s.bankCode+".csr"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *genCSRStep) Run(_ context.Context) error {
	return pki.GenerateBankCSR(s.bankCode, s.institution, filepath.Join(s.dataDir, "pki"))
}
