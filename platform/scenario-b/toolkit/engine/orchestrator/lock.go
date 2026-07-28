package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Lock is an exclusive flock on <dataDir>/.provisioning.lock. A second
// concurrent run is refused (non-blocking). An orphan lock (holder process
// died) is released automatically by the OS when its fd closes — so the engine
// never blocks forever.
type Lock struct {
	f    *os.File
	path string
}

// AcquireLock takes the exclusive, non-blocking lock; refuses if held.
func AcquireLock(dataDir string) (*Lock, error) {
	p := filepath.Join(dataDir, ".provisioning.lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another provisioning run holds the lock (%s): %w", p, err)
	}
	// Record pid for diagnostics of an orphan lock.
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return &Lock{f: f, path: p}, nil
}

// Release unlocks and closes.
func (l *Lock) Release() error {
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	return l.f.Close()
}
