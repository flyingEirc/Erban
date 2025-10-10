package ssh

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

// PortForward tracks active forward listeners. Now package-global.
type PortForward struct {
	locals   []*forwardTracker
	remotes  []*forwardTracker
	dynamics []*forwardTracker
}

// GlobalPortForward is the package-wide manager tracking active forwards.
var GlobalPortForward PortForward

type forwardTracker struct {
	listener net.Listener
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	closed   bool
	wg       sync.WaitGroup
}

func newForwardTracker(listener net.Listener) *forwardTracker {
	return &forwardTracker{
		listener: listener,
		conns:    make(map[net.Conn]struct{}),
	}
}

func (ft *forwardTracker) trackConn(conn net.Conn) bool {
	if conn == nil {
		return false
	}
	ft.mu.Lock()
	if ft.closed {
		ft.mu.Unlock()
		_ = conn.Close()
		return false
	}
	ft.conns[conn] = struct{}{}
	ft.mu.Unlock()
	return true
}

func (ft *forwardTracker) untrackConn(conn net.Conn) {
	if conn == nil {
		return
	}
	ft.mu.Lock()
	delete(ft.conns, conn)
	ft.mu.Unlock()
}

func (ft *forwardTracker) isClosed() bool {
	ft.mu.Lock()
	closed := ft.closed
	ft.mu.Unlock()
	return closed
}

func (ft *forwardTracker) Close() error {
	ft.mu.Lock()
	if ft.closed {
		ft.mu.Unlock()
		ft.wg.Wait()
		return nil
	}
	ft.closed = true
	conns := make([]net.Conn, 0, len(ft.conns))
	for c := range ft.conns {
		conns = append(conns, c)
	}
	listener := ft.listener
	ft.mu.Unlock()

	err := listener.Close()
	for _, c := range conns {
		_ = c.Close()
	}
	ft.wg.Wait()
	return err
}

// LocalForward starts local port forwarding: localAddr => remoteAddr via SSH.
func (s *Sshobject) LocalForward(localAddr, remoteAddr string) (func() error, error) {
	if s.client == nil {
		return nil, fmt.Errorf("ssh client not started")
	}
	ln, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	tracker := newForwardTracker(ln)
	GlobalPortForward.locals = append(GlobalPortForward.locals, tracker)
	LogInfof("Local forward %s => %s started", localAddr, remoteAddr)
	go func() {
		for {
			lc, err := ln.Accept()
			if err != nil {
				if tracker.isClosed() || errors.Is(err, net.ErrClosed) {
					return
				}
				// Log and backoff briefly before retrying
				LogErrorf("Local forward accept error: %v", err)
				continue
			}
			if !tracker.trackConn(lc) {
				continue
			}
			tracker.wg.Add(1)
			go func(c net.Conn) {
				defer tracker.wg.Done()
				defer tracker.untrackConn(c)
				defer c.Close()

				rc, err := s.client.Dial("tcp", remoteAddr)
				if err != nil {
					LogErrorf("Local forward dial remote failed: %v", err)
					return
				}
				if !tracker.trackConn(rc) {
					return
				}
				defer func() {
					LogInfof("退出")
					tracker.untrackConn(rc)
					rc.Close()
				}()
				proxyPipe(c, rc)
			}(lc)
		}
	}()
	cancel := func() error {
		LogInfof("Local forward %s => %s stopped", localAddr, remoteAddr)

		return tracker.Close()
	}
	return cancel, nil
}

// RemoteForward starts remote port forwarding: remoteBind => localTarget via SSH.
func (s *Sshobject) RemoteForward(remoteBind, localTarget string) (func() error, error) {
	if s.client == nil {
		return nil, fmt.Errorf("ssh client not started")
	}
	rln, err := s.client.Listen("tcp", remoteBind)
	if err != nil {
		return nil, err
	}
	tracker := newForwardTracker(rln)
	GlobalPortForward.remotes = append(GlobalPortForward.remotes, tracker)
	LogInfof("Remote forward %s => %s started", remoteBind, localTarget)
	go func() {
		for {
			rc, err := rln.Accept()
			LogInfof("进来一个连接")
			if err != nil {
				if tracker.isClosed() {
					return
				}
				// The underlying ssh listener may not expose net.ErrClosed reliably.
				LogErrorf("Remote forward accept error: %v", err)
				return
			}
			if !tracker.trackConn(rc) {
				continue
			}
			tracker.wg.Add(1)
			go func(c net.Conn) {
				defer tracker.wg.Done()
				defer tracker.untrackConn(c)
				defer c.Close()
				LogInfof("处理一个连接")
				lc, err := net.Dial("tcp", localTarget)
				if err != nil {
					LogErrorf("Remote forward dial local failed: %v", err)
					return
				}
				if !tracker.trackConn(lc) {
					lc.Close()
					return
				}
				defer func() {
					tracker.untrackConn(lc)
					lc.Close()
				}()
				proxyPipe(c, lc)
			}(rc)
		}
	}()
	cancel := func() error {
		LogInfof("Remote forward %s => %s stopped", remoteBind, localTarget)
		return tracker.Close()
	}
	return cancel, nil
}

// DynamicForward starts a SOCKS5 proxy on localSocks that tunnels via SSH.
func (s *Sshobject) DynamicForward(localSocks string) (func() error, error) {
	if s.client == nil {
		return nil, fmt.Errorf("ssh client not started")
	}
	ln, err := net.Listen("tcp", localSocks)
	if err != nil {
		return nil, err
	}
	tracker := newForwardTracker(ln)
	GlobalPortForward.dynamics = append(GlobalPortForward.dynamics, tracker)
	LogInfof("Dynamic SOCKS5 %s started", localSocks)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				if tracker.isClosed() || errors.Is(err, net.ErrClosed) {
					return
				}
				LogErrorf("Dynamic SOCKS accept error: %v", err)
				continue
			}
			if !tracker.trackConn(c) {
				continue
			}
			tracker.wg.Add(1)
			go func(conn net.Conn) {
				defer tracker.wg.Done()
				s.handleSocks5(conn, tracker)
			}(c)
		}
	}()
	cancel := func() error {
		LogInfof("Dynamic SOCKS5 %s stopped", localSocks)
		return tracker.Close()
	}
	return cancel, nil
}

func proxyPipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	LogInfof("进入IO交换")
	go func() { _, _ = io.Copy(a, b); closeWrite(a); done <- struct{}{} }()
	go func() { _, _ = io.Copy(b, a); closeWrite(b); done <- struct{}{} }()
	<-done
	<-done
}

func closeWrite(c net.Conn) {
	type closeWriter interface{ CloseWrite() error }
	if cw, ok := c.(closeWriter); ok {
		_ = cw.CloseWrite()
	} else {
		_ = c.Close()
	}
}

// Minimal SOCKS5 handler: no auth, CONNECT only.
func (s *Sshobject) handleSocks5(c net.Conn, tracker *forwardTracker) {
	if tracker != nil {
		defer tracker.untrackConn(c)
	}
	defer c.Close()
	br := bufio.NewReader(c)
	bw := bufio.NewWriter(c)

	// handshake
	ver, _ := br.ReadByte()
	if ver != 0x05 {
		return
	}
	nm, _ := br.ReadByte()
	for i := 0; i < int(nm); i++ {
		_, _ = br.ReadByte()
	}
	_, _ = bw.Write([]byte{0x05, 0x00}) // no-auth
	_ = bw.Flush()

	// request: VER CMD RSV ATYP DST.ADDR DST.PORT
	header := make([]byte, 4)
	if _, err := io.ReadFull(br, header); err != nil {
		return
	}
	if header[0] != 0x05 || header[1] != 0x01 { // CONNECT only
		bw.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		bw.Flush()
		return
	}
	var host string
	switch header[3] {
	case 0x01: // IPv4
		addr := make([]byte, 4)
		if _, err := io.ReadFull(br, addr); err != nil {
			return
		}
		host = net.IP(addr).String()
	case 0x03: // Domain
		ln, _ := br.ReadByte()
		d := make([]byte, int(ln))
		if _, err := io.ReadFull(br, d); err != nil {
			return
		}
		host = string(d)
	case 0x04: // IPv6
		addr := make([]byte, 16)
		if _, err := io.ReadFull(br, addr); err != nil {
			return
		}
		host = net.IP(addr).String()
	default:
		bw.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		bw.Flush()
		return
	}
	portb := make([]byte, 2)
	if _, err := io.ReadFull(br, portb); err != nil {
		return
	}
	port := int(portb[0])<<8 | int(portb[1])
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	rc, err := s.client.Dial("tcp", target)
	if err != nil {
		LogErrorf("SOCKS dial via SSH failed: %v", err)
		bw.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		bw.Flush()
		return
	}
	if tracker != nil && !tracker.trackConn(rc) {
		rc.Close()
		return
	}
	defer func() {
		if tracker != nil {
			tracker.untrackConn(rc)
		}
		rc.Close()
	}()

	// success
	bw.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	bw.Flush()
	proxyPipe(c, rc)
}
