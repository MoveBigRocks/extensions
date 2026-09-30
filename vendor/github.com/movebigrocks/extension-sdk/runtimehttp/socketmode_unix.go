//go:build unix

package runtimehttp

import (
	"net"
	"sync"

	"golang.org/x/sys/unix"
)

// socketUmask leaves a newly created unix socket accessible only to its owner:
// 0777 &^ 0177 is 0600.
const socketUmask = 0o177

// umaskMu serializes the swap below. A umask is process-wide, so any file
// another goroutine created during the swap would inherit it. A runtime binds
// its socket once during bootstrap, before it serves anything, which is why
// taking that window here is safe and why nothing else in the SDK does it.
var umaskMu sync.Mutex

// listenUnixRestricted binds socketPath with the socket created owner-only,
// closing the window that a bind-then-chmod would leave open.
func listenUnixRestricted(socketPath string) (net.Listener, error) {
	umaskMu.Lock()
	defer umaskMu.Unlock()
	previous := unix.Umask(socketUmask)
	defer unix.Umask(previous)
	return net.Listen("unix", socketPath)
}
