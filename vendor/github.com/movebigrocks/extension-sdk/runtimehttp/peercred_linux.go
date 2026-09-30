//go:build linux

package runtimehttp

import (
	"net"

	"golang.org/x/sys/unix"
)

// peerCredentialsSupported reports that this platform can identify a unix
// socket peer. Linux is where runtimes ship in production.
func peerCredentialsSupported() bool { return true }

// peerUID returns the effective uid of the process on the other end of conn,
// taken from the credentials the kernel recorded for it at connect time.
func peerUID(conn net.Conn) (uint32, error) {
	return controlUnixConn(conn, func(fd int) (uint32, error) {
		credentials, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			return 0, err
		}
		return credentials.Uid, nil
	})
}
