package certsource

import "strings"

// New selects the implementation by URI:
//
//	self-signed | self-signed://<...>  -> local, in-memory CA per spoke
//	ca://<...>                         -> production stub (ErrNotImplemented)
//
// Any other URI returns ErrUnsupportedURI.
func New(uri string) (CertSource, error) {
	switch {
	case uri == "self-signed" || strings.HasPrefix(uri, "self-signed://"):
		return newLocal(), nil
	case strings.HasPrefix(uri, "ca://"):
		return prodCertSource{}, nil
	default:
		return nil, ErrUnsupportedURI
	}
}
