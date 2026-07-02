// SPDX-License-Identifier: Apache-2.0

package certsource

import "context"

// prodCertSource is a stub for the production CA provider.
// All methods return ErrNotImplemented until PR-2 (Phase 4) is complete.
type prodCertSource struct{}

var _ CertSource = (*prodCertSource)(nil)

func (p *prodCertSource) IssueLeafCert(_ context.Context, _ []byte, _ string) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (p *prodCertSource) GetTrustAnchor(_ context.Context, _ string) ([]byte, error) {
	return nil, ErrNotImplemented
}
