package certsource

import "context"

// prodCertSource is the production stub: real PKI/CA integration is a later
// phase. Every operation returns ErrNotImplemented.
type prodCertSource struct{}

func (prodCertSource) IssueLeafCert(context.Context, []byte, string) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (prodCertSource) GetTrustAnchor(context.Context, string) ([]byte, error) {
	return nil, ErrNotImplemented
}
