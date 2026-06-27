// SPDX-License-Identifier: Apache-2.0

package keyprovider

import (
	"fmt"
	"strings"
)

const schemePrefix = "kms://"

// New parses the keyProvider URI from spec.keyProvider and returns the
// appropriate KeyProvider implementation.
//
// Accepted URI forms:
//   - kms://local-emulator  → LocalKeyProvider (in-memory, keys start empty)
//   - kms://<any-other>     → prodKeyProvider stub (all methods return ErrNotImplemented)
//
// Returns an error for any URI that does not begin with "kms://".
func New(uri string) (KeyProvider, error) {
	if !strings.HasPrefix(uri, schemePrefix) {
		return nil, fmt.Errorf("keyprovider: invalid URI %q: must begin with %q", uri, schemePrefix)
	}
	backend := strings.TrimPrefix(uri, schemePrefix)
	switch backend {
	case "local-emulator":
		return NewLocalKeyProvider(), nil
	default:
		return &prodKeyProvider{}, nil
	}
}
