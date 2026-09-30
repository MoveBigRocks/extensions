//go:build !unix

package runtimehttp

import (
	"fmt"
	"net"
	"runtime"
)

// listenUnixRestricted refuses to bind where the process cannot constrain the
// mode the socket is created with. ListenAndServeUnixSocket rejects these
// platforms earlier, because peer credentials are unavailable on them too; this
// keeps the package building and still fails closed should that order ever
// change.
func listenUnixRestricted(string) (net.Listener, error) {
	return nil, fmt.Errorf("runtime sockets cannot be created privately on %s", runtime.GOOS)
}
