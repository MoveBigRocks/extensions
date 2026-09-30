package runtimehttp

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/gin-gonic/gin"
)

// socketMode keeps a runtime's unix socket readable and writable only by the
// account the runtime runs as. The host supervisor is the sole legitimate peer:
// it runs as the same user and owns the socket directory.
const socketMode os.FileMode = 0o600

// verifiedPeerContextKey marks the base context of a connection whose peer
// credentials this process read from the kernel and accepted. The key is
// unexported and typed so no other package can set it by accident.
type verifiedPeerContextKey struct{}

// WithVerifiedPeer marks ctx as belonging to a connection this runtime
// authenticated. ListenAndServeUnixSocket applies it to every accepted
// connection; a test driving a handler through httptest uses it to stand in for
// that listener. Nothing else should call it, because marking a request the
// runtime never authenticated reopens the header-spoofing path
// ForwardedContextMiddleware exists to close.
func WithVerifiedPeer(ctx context.Context) context.Context {
	return context.WithValue(ctx, verifiedPeerContextKey{}, true)
}

// PeerVerified reports whether ctx arrived on a connection whose peer process
// runs as this runtime's user.
func PeerVerified(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	verified, ok := ctx.Value(verifiedPeerContextKey{}).(bool)
	return ok && verified
}

// authorizePeerUID admits a peer running as this runtime's own user, or as
// root. Anything else is a separate local account reaching for this runtime's
// socket, which is exactly the cross-extension impersonation the socket mode
// alone cannot stop.
func authorizePeerUID(uid uint32) error {
	runtimeUID := os.Geteuid()
	if uid == 0 || uid == uint32(runtimeUID) {
		return nil
	}
	return fmt.Errorf("peer uid %d is neither the runtime uid %d nor root", uid, runtimeUID)
}

// verifiedConn marks an accepted connection whose peer credentials passed
// authorizePeerUID. Only this type reaches peerConnContext, which is how a
// request proves it came in over an authenticated connection.
type verifiedConn struct {
	net.Conn
}

// verifiedPeerListener authenticates every inbound connection from the kernel's
// record of the peer's credentials before an HTTP handler can see it. That
// record is made at connect time and cannot be forged by the peer. It is the
// enforcing control rather than the socket mode, because BSD-derived kernels
// (macOS among them) do not check a unix socket's permissions on connect.
type verifiedPeerListener struct {
	net.Listener
	// readPeerUID is a field so a test can exercise the rejection path without
	// the privileges needed to run a second account. Production always uses the
	// kernel-backed reader.
	readPeerUID func(net.Conn) (uint32, error)
}

func newVerifiedPeerListener(listener net.Listener) *verifiedPeerListener {
	return &verifiedPeerListener{Listener: listener, readPeerUID: peerUID}
}

// Accept returns only connections whose peer passed the credential check. A
// refused peer is closed and accepting continues, so a hostile local process
// cannot stop the runtime from serving the host by hammering the socket.
func (l *verifiedPeerListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		uid, err := l.readPeerUID(conn)
		if err == nil {
			err = authorizePeerUID(uid)
		}
		if err != nil {
			fmt.Fprintf(gin.DefaultErrorWriter, "[MBR] refused runtime socket peer: %v\n", err)
			_ = conn.Close()
			continue
		}
		return &verifiedConn{Conn: conn}, nil
	}
}

// peerConnContext marks the base context of an authenticated connection, which
// is how ForwardedContextMiddleware tells a host-forwarded request from one
// that reached the handler some other way. It is the http.Server ConnContext
// hook, so every request on the connection inherits the mark.
func peerConnContext(ctx context.Context, conn net.Conn) context.Context {
	if _, ok := conn.(*verifiedConn); !ok {
		return ctx
	}
	return WithVerifiedPeer(ctx)
}

// listenPrivateUnix binds the runtime's socket so only the account it runs as
// can connect. Creation happens under a restrictive umask rather than a chmod
// afterwards, so the socket is never momentarily world-writable, and the
// resulting mode is asserted because a umask constrains creation but proves
// nothing about the result.
func listenPrivateUnix(socketPath string) (net.Listener, error) {
	listener, err := listenUnixRestricted(socketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socketPath, socketMode); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("chmod runtime socket: %w", err)
	}
	info, err := os.Stat(socketPath)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	if perm := info.Mode().Perm(); perm != socketMode {
		_ = listener.Close()
		return nil, fmt.Errorf("runtime socket mode is %04o, want %04o", perm, socketMode)
	}
	return listener, nil
}

// controlUnixConn reads a value from the connection's file descriptor. The
// descriptor is only valid for the duration of the callback, so the uid is
// copied out before returning.
func controlUnixConn(conn net.Conn, read func(fd int) (uint32, error)) (uint32, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("peer credentials need a unix connection, got %T", conn)
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var (
		uid     uint32
		readErr error
	)
	if err := raw.Control(func(fd uintptr) {
		uid, readErr = read(int(fd))
	}); err != nil {
		return 0, err
	}
	return uid, readErr
}
