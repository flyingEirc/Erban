package ssh

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Mnitoer struct {
}

type Retary struct {
}

type Sshobject struct {
	Label  string
	Host   string
	User   string
	Passwd string
	Pem    string
	M      Mnitoer
	R      Retary
	config *ssh.ClientConfig
	client *ssh.Client
	P      Proxy
	Ftp    *sftp.Client
}

// CreateClient establishes the SSH connection for the given object.
// Returns an error if the connection (direct or via proxy) fails.
func CreateClient(s *Sshobject) error {
	if s == nil {
		return fmt.Errorf("nil ssh object")
	}
	return s.createClientImpl()
}

// Close terminates the SSH client and any related resources.
func Close(s *Sshobject) {
	if s == nil {
		return
	}
	s.closeImpl()
}

// ConnectAndStartStream is a convenience that creates the SSH client and
// immediately starts an interactive streaming shell. Useful for UIs that
// want a single call to establish and begin streaming.
func ConnectAndStartStream(s *Sshobject, out io.Writer, rows, cols int) (*StreamSession, error) {
	if s == nil {
		return nil, fmt.Errorf("nil ssh object")
	}
	// Ensure client is created
	if err := CreateClient(s); err != nil {
		return nil, err
	}
	// Start stream with provided terminal size
	return StartStream(s, out, rows, cols)
}

// SetProxy sets a proxy URL for the SSH connection. Accepts http(s) and socks5.
// Examples:
//
//	http://127.0.0.1:8080
//	127.0.0.1:8080 (defaults to http)
//	socks5://127.0.0.1:1080
func SetProxy(s *Sshobject, v string) error {
	if s == nil {
		return fmt.Errorf("nil ssh object")
	}
	// If the user provided an explicit scheme, use it; otherwise default to http.
	// Both http(s) and socks5(h) are allowed.
	def := "http"
	lower := strings.ToLower(strings.TrimSpace(v))
	if strings.HasPrefix(lower, "socks5") {
		def = "socks5"
	}
	u, err := normalizeProxyURL(v, def, "http", "https", "socks5", "socks5h")
	if err != nil {
		return err
	}
	s.P.URL = u.String()
	return nil
}

func InitWithPasswd(host, user, passwd string) *Sshobject {
	config := &ssh.ClientConfig{
		Timeout: 30 * time.Second,
		User:    user,
		Auth: []ssh.AuthMethod{
			ssh.Password(passwd),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //  fixhostkey(defaultKnownHostsPath),
	}

	return &Sshobject{
		Host:   host,
		User:   user,
		Passwd: passwd,
		config: config,
	}
}

func InitWithPem(host, user string, pem []byte) *Sshobject {
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		if fl, e := NewFileLogger("log.txt"); e == nil && fl != nil {
			fl.Errorf("Private key parse failed: %v", err)
		}
		return &Sshobject{Host: host, User: user, Pem: ""}
	}

	config := &ssh.ClientConfig{
		Timeout: 30 * time.Second,
		User:    user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //fixhostkey(defaultKnownHostsPath),
	}

	return &Sshobject{
		Host:   host,
		User:   user,
		Pem:    "",
		config: config,
	}
}

// createClientImpl contains the original implementation for creating the client.
// It is used by both the top-level CreateClient function and the legacy method wrapper.
func (s *Sshobject) createClientImpl() error {
	LogInfof("SSH connecting to %s", s.Host)
	if s.Ftp != nil {
		_ = s.Ftp.Close()
		s.Ftp = nil
	}
	if s.config == nil {
		err := fmt.Errorf("SSH config not initialized for %s", s.Host)
		LogErrorf(err.Error())
		return err
	}
	// Use user-configured proxy if provided; otherwise connect directly.
	if ps := strings.TrimSpace(s.P.URL); ps != "" {
		proxyURL, perr := url.Parse(ps)
		if perr != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
			err := fmt.Errorf("invalid proxy URL '%s': %v", ps, perr)
			LogErrorf(err.Error())
			return err
		}
		c, err := dialThroughProxy(proxyURL, s.Host)
		if err != nil {
			e := fmt.Errorf("proxy connect failed via %s: %v", proxyURL.String(), err)
			LogErrorf(e.Error())
			return e
		}
		// Handshake SSH over the established tunnel
		conn, chans, reqs, err := ssh.NewClientConn(c, s.Host, s.config)
		if err != nil {
			_ = c.Close()
			e := fmt.Errorf("SSH handshake failed via proxy %s: %v", proxyURL.String(), err)
			LogErrorf(e.Error())
			return e
		}
		s.client = ssh.NewClient(conn, chans, reqs)
		LogInfof("SSH connected to %s via proxy %s", s.Host, proxyURL.String())
		return nil
	}

	// Fallback: direct connection
	client, err := ssh.Dial("tcp", s.Host, s.config)
	if err != nil {
		e := fmt.Errorf("direct connect failed to %s: %v", s.Host, err)
		LogErrorf(e.Error())
		return e
	}
	s.client = client
	LogInfof("SSH connected to %s (direct)", s.Host)
	return nil
}

// closeImpl contains the original implementation for closing the client.
func (s *Sshobject) closeImpl() {
	if s.Ftp != nil {
		_ = s.Ftp.Close()
		s.Ftp = nil
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
}

// // 轮询本地窗口尺寸，变化时通知远端
// func watchResize(fd int, sess *ssh.Session, stop <-chan struct{}) {
// 	w, h, _ := term.GetSize(fd)
// 	lastW, lastH := w, h

// 	ticker := time.NewTicker(500 * time.Millisecond)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			if term.IsTerminal(fd) {
// 				if nw, nh, err := term.GetSize(fd); err == nil {
// 					if nw != lastW || nh != lastH {
// 						// 注意 WindowChange 的参数顺序是 (rows, cols)
// 						_ = sess.WindowChange(nh, nw)
// 						lastW, lastH = nw, nh
// 					}
// 				}
// 			}
// 		case <-stop:
// 			return
// 		}
// 	}
// }

// StreamSession represents an interactive SSH session with stream-based IO.
type StreamSession struct {
	sess   *ssh.Session
	stdinW *io.PipeWriter
	done   chan struct{}
}

// StartStream starts an interactive shell, wiring stdout/stderr to out.
// Returns a StreamSession whose Write method sends data to remote stdin.
func StartStream(s *Sshobject, out io.Writer, rows, cols int) (*StreamSession, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("ssh client not started")
	}
	sess, err := s.client.NewSession()
	if err != nil {
		LogErrorf("New session failed: %v", err)
		return nil, err
	}

	// Request PTY
	if rows <= 0 {
		rows = 40
	}
	if cols <= 0 {
		cols = 120
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = sess.Close()
		return nil, err
	}

	pr, pw := io.Pipe()
	sess.Stdin = pr
	sess.Stdout = out
	sess.Stderr = out

	if err := sess.Shell(); err != nil {
		_ = sess.Close()
		_ = pw.Close()
		return nil, err
	}
	// Wait in background so remote can run until closed; clean pipe when done.
	st := &StreamSession{sess: sess, stdinW: pw, done: make(chan struct{})}
	go func() {
		_ = sess.Wait()
		_ = pw.Close()
		_ = pr.Close()
		_ = sess.Close()
		LogInfof("SSH connection to %s has benn closed", s.Host)
		close(st.done)
	}()
	return st, nil
}

// Write sends data to the remote shell stdin.
func (st *StreamSession) Write(p []byte) (int, error) {
	if st == nil || st.stdinW == nil {
		return 0, fmt.Errorf("session closed")
	}
	return st.stdinW.Write(p)
}

// Close terminates the interactive session.
func (st *StreamSession) Close() error {
	if st == nil || st.sess == nil {
		return nil
	}
	defer LogInfof("")
	return st.sess.Close()
}

// Resize changes the remote PTY size to rows x cols.
func (st *StreamSession) Resize(rows, cols int) error {
	if st == nil || st.sess == nil {
		return fmt.Errorf("session closed")
	}
	return st.sess.WindowChange(rows, cols)
}

// Done returns a channel that is closed when the remote shell exits.
func (st *StreamSession) Done() <-chan struct{} {
	if st == nil || st.done == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return st.done
}

// ===== Backward-compatible thin wrappers =====
// These maintain existing method names while delegating to the new helpers.

// Createclient preserves the existing exported method signature.
func (s *Sshobject) Createclient() { CreateClient(s) }

// Close preserves the existing exported method signature.
func (s *Sshobject) Close() { Close(s) }

// stdLogger type moved to log.go

// Known hosts storage (relative to repo root by default)
const defaultKnownHostsPath = "internal/ssh/known_hosts"

// fixhostkey returns an ssh.HostKeyCallback which checks the fingerprint in known_hosts
// and, if unknown, prompts the user to confirm and saves it.
func fixhostkey(path string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		if hasFingerprint(path, fp) {
			return nil
		}
		fmt.Printf("Unknown host key for %s (%s)\nFingerprint (SHA256): %s\nTrust this host and add to %s? (yes/no): ", hostname, remote.String(), fp, path)
		br := bufio.NewReader(os.Stdin)
		line, _ := br.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "yes" && line != "y" {
			return fmt.Errorf("host key not accepted by user")
		}
		if dir := filepath.Dir(path); dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
		hostField := normalizeKnownHostName(hostname)
		// OpenSSH known_hosts line: hostnames keytype base64key  # include fingerprint as comment for quick lookup
		lineToAdd := fmt.Sprintf("%s %s %s # %s\n", hostField, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()), fp)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("failed to write known_hosts: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(lineToAdd); err != nil {
			return fmt.Errorf("failed to append known_hosts: %w", err)
		}
		return nil
	}
}

func normalizeKnownHostName(hostname string) string {
	if strings.HasPrefix(hostname, "[") && strings.Contains(hostname, "]:") {
		return hostname
	}
	if strings.Contains(hostname, ":") {
		return "[" + hostname + "]"
	}
	return hostname
}

func hasFingerprint(path, fingerprint string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), fingerprint)
}

// normalizeProxyURL validates and normalizes a proxy URL.
// If scheme is missing, defaultScheme is prepended. Only schemes in allowed are accepted.
func normalizeProxyURL(raw, defaultScheme string, allowed ...string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("empty proxy")
	}
	val := strings.TrimSpace(raw)
	if !strings.Contains(val, "://") {
		val = defaultScheme + "://" + val
	}
	u, err := url.Parse(val)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid proxy: missing scheme or host")
	}
	ok := false
	ls := strings.ToLower(u.Scheme)
	for _, a := range allowed {
		if ls == strings.ToLower(a) {
			ok = true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("unsupported proxy scheme: %s", u.Scheme)
	}

	host := u.Host
	// Validate optional port if present
	hasPort := false
	if strings.HasPrefix(host, "[") {
		// IPv6 literal
		if idx := strings.LastIndex(host, "]:"); idx != -1 {
			hasPort = true
		}
	} else if strings.Count(host, ":") == 1 {
		hasPort = true
	}
	if hasPort {
		_, port, err := net.SplitHostPort(host)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy host:port: %w", err)
		}
		if p, e := strconv.Atoi(port); e != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid proxy port: %s", port)
		}
	}
	return u, nil
}

// SetSOCKSProxy sets a SOCKS5 proxy. Accepts forms like:
//  1. "127.0.0.1:1080"
//  2. "user:pass@127.0.0.1:1080"
//  3. "socks5://127.0.0.1:1080" or "socks5://user:pass@host:port"
func (s *Sshobject) SetSOCKSProxy(v string) error {
	u, err := normalizeProxyURL(v, "socks5", "socks5", "socks5h")
	if err != nil {
		return err
	}
	s.P.URL = u.String()
	return nil
}

// SetHTTPProxy sets an HTTP proxy. Accepts forms like:
//  1. "127.0.0.1:8080"
//  2. "user:pass@127.0.0.1:8080"
//  3. "http://127.0.0.1:8080" or "http://user:pass@host:port"
//  4. "https://host:port"（目前仍按明文 CONNECT 处理）
func (s *Sshobject) SetHTTPProxy(v string) error {
	u, err := normalizeProxyURL(v, "http", "http", "https")
	if err != nil {
		return err
	}
	s.P.URL = u.String()
	return nil
}
