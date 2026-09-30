//go:build !linux && !darwin

package runtimehttp

import (
	"fmt"
	"net"
	"runtime"
)

// peerCredentialsSupported reports false wherever the SDK has no kernel-backed
// way to identify a socket peer. Runtimes ship on Linux and are developed on
// macOS, both of which are covered; rather than serve a socket it cannot
// authenticate, ListenAndServeUnixSocket refuses to start here. The fallback
// therefore fails closed instead of quietly trusting whoever connects.
func peerCredentialsSupported() bool { return false }

// peerUID always fails here, so a connection that somehow reached the listener
// is refused rather than trusted.
func peerUID(net.Conn) (uint32, error) {
	return 0, fmt.Errorf("peer credentials are unavailable on %s", runtime.GOOS)
}
