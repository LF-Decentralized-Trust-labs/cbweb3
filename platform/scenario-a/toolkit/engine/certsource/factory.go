// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"fmt"
	"strings"
)

const (
	selfSignedBare   = "self-signed"
	selfSignedScheme = "self-signed://"
	prodScheme       = "ca://"
)

// New parses the certSource value from spec.certSource and returns the
// appropriate CertSource implementation.
//
// Accepted forms:
//   - self-signed          → LocalCertSource (canonical local value, per concat.md §5.1)
//   - self-signed://<any>   → LocalCertSource (tolerated scheme form)
//   - ca://<any>            → prodCertSource stub (all methods return ErrNotImplemented)
//
// Returns an error for any value that does not match a recognised form.
func New(uri string) (CertSource, error) {
	switch {
	case uri == selfSignedBare || strings.HasPrefix(uri, selfSignedScheme):
		return NewLocalCertSource(), nil
	case strings.HasPrefix(uri, prodScheme):
		return &prodCertSource{}, nil
	default:
		return nil, fmt.Errorf("certsource: invalid value %q: must be %q, or begin with %q or %q", uri, selfSignedBare, selfSignedScheme, prodScheme)
	}
}
