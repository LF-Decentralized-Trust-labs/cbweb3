// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"strconv"
	"syscall"
)

// dockerSocketPath is where the noc-agent reads container logs from. It is bind-mounted
// read-only into the agent by every noc-agent compose template.
const dockerSocketPath = "/var/run/docker.sock"

// dockerSocketGID returns the gid that owns the Docker socket, or "" if it cannot be
// determined.
//
// The noc-agent image is non-root (uid 65532, finding R2-M-12) but the socket is
// normally root:docker mode 0660, so an unprivileged uid outside that group gets EACCES
// on every read. The agent discards that error (collector.collectLogs is called under
// `if err == nil`), so the failure surfaces as an empty Log Viewer with nothing in the
// agent's own output — the same degraded-behind-silence mode this finding set out to
// remove from the PKI. Passing the socket's gid as a supplementary group is the standard
// fix and keeps the container off root.
//
// A gid is a plain number and a bind mount preserves it, so on Linux — where this toolkit
// deploys — the host's view is the container's. Docker Desktop is the exception: it
// proxies the socket, so the two differ. Reading it from inside a throwaway container
// would be exact, but this function is called on every ComposeEnv, including from unit
// tests, so it stays a plain stat. A wrong or unreadable gid grants nothing rather than
// too much, and the agent reports the resulting loss of log access at startup instead of
// collecting nothing in silence.
func dockerSocketGID() string {
	fi, err := os.Stat(dockerSocketPath)
	if err != nil {
		return ""
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return strconv.FormatUint(uint64(st.Gid), 10)
}
