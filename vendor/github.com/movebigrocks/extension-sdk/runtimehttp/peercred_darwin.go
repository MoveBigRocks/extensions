//go:build darwin

package runtimehttp

import (
	"net"

	"golang.org/x/sys/unix"
)

// peerCredentialsSupported reports that this platform can identify a unix
// socket peer. macOS is where runtimes are developed locally, and it is also
// where the check carries the whole weight: the kernel does not consult a unix
// socket's file mode on connect, so the socket being 0600 stops nobody here.
func peerCredentialsSupported() bool { return true }

// peerUID returns the effective uid of the process on the other end of conn,
// taken from the credentials the kernel recorded for it at connect time.
func peerUID(conn net.Conn) (uint32, error) {
	return controlUnixConn(conn, func(fd int) (uint32, error) {
		credentials, err := unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			return 0, err
		}
		return credentials.Uid, nil
	})
}
