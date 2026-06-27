// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"fmt"
	"strings"
)

const (
	selfSignedScheme = "self-signed://"
	prodScheme       = "ca://"
)

// New parses the certSource URI from spec.certSource and returns the
// appropriate CertSource implementation.
//
// Accepted URI forms:
//   - self-signed://<any>  → LocalCertSource (in-memory, self-signed CA per spoke)
//   - ca://<any>           → prodCertSource stub (all methods return ErrNotImplemented)
//
// Returns an error for any URI that does not begin with a recognised scheme.
func New(uri string) (CertSource, error) {
	switch {
	case strings.HasPrefix(uri, selfSignedScheme):
		return NewLocalCertSource(), nil
	case strings.HasPrefix(uri, prodScheme):
		return &prodCertSource{}, nil
	default:
		return nil, fmt.Errorf("certsource: invalid URI %q: must begin with %q or %q", uri, selfSignedScheme, prodScheme)
	}
}
