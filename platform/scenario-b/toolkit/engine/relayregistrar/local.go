// SPDX-License-Identifier: Apache-2.0

package relayregistrar

import (
	"context"
	"sync"
)

// localRegistrar keeps registered spokes in memory (dev/tests). Register is an
// idempotent upsert by spoke ID.
type localRegistrar struct {
	mu     sync.Mutex
	spokes map[string]Spoke
}

func newLocal() *localRegistrar {
	return &localRegistrar{spokes: make(map[string]Spoke)}
}

func (l *localRegistrar) Register(_ context.Context, s Spoke) error {
	if err := validate(s); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.spokes[s.ID] = s // idempotent upsert
	return nil
}

func (l *localRegistrar) List() []Spoke {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Spoke, 0, len(l.spokes))
	for _, s := range l.spokes {
		out = append(out, s)
	}
	return out
}
