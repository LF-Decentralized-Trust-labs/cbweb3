package keyprovider

import (
	"net/url"
	"strings"
)

// New selects the implementation by URI:
//
//	kms://local-emulator[?seed=...]  -> local, seeded, in-memory
//	kms://<anything-else>            -> production stub (ErrNotImplemented)
//
// Any URI without the kms:// scheme returns ErrUnsupportedURI.
func New(uri string) (KeyProvider, error) {
	if !strings.HasPrefix(uri, "kms://") {
		return nil, ErrUnsupportedURI
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, ErrUnsupportedURI
	}
	if u.Host == "local-emulator" {
		return newLocal(u.Query().Get("seed")), nil
	}
	return prodKeyProvider{}, nil
}
