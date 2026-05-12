package api

import (
	"context"
	"net"
	"sync"
	"syscall"

	"google.golang.org/grpc/peer"
)

type contextKey int

const (
	pidKey contextKey = iota
)

var (
	pidMap = make(map[string]int)
	pidMu  sync.RWMutex
)

type CredsListener struct {
	net.Listener
}

func NewCredsListener(l net.Listener) net.Listener {
	return &CredsListener{l}
}

func (l *CredsListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	unixConn, ok := conn.(*net.UnixConn)
	if ok {
		rawConn, err := unixConn.SyscallConn()
		if err == nil {
			rawConn.Control(func(fd uintptr) {
				ucred, err := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
				if err == nil {
					pidMu.Lock()
					pidMap[conn.RemoteAddr().String()] = int(ucred.Pid)
					pidMu.Unlock()
				}
			})
		}
	}

	return &credsConn{conn}, nil
}

type credsConn struct {
	net.Conn
}

func (c *credsConn) Close() error {
	pidMu.Lock()
	delete(pidMap, c.RemoteAddr().String())
	pidMu.Unlock()
	return c.Conn.Close()
}

func GetPeerPID(ctx context.Context) (int, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return 0, false
	}

	pidMu.RLock()
	defer pidMu.RUnlock()
	pid, ok := pidMap[p.Addr.String()]
	return pid, ok
}
