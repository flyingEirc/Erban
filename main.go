package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	chatmodel "github.com/flyingeirc/erban/internal/chat/model"
	chatoutput "github.com/flyingeirc/erban/internal/chat/output"
	sshpkg "github.com/flyingeirc/erban/internal/ssh"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Bridge object exposing SSH controls to the frontend
	ssh := &SSHBridge{}
	// Bridge object exposing Chat(OpenAI) controls to the frontend
	chat := &ChatBridge{}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Erban",
		Width:     1500,
		Height:    900,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows: &windows.Options{
			IsZoomControlEnabled: false,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			ssh.startup(ctx)
			chat.startup(ctx)
		},
		Bind: []interface{}{
			ssh,
			chat,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// SSHBridge provides Wails-exposed methods that delegate to internal SSH helpers.
type SSHBridge struct {
	ctx     context.Context
	keyFile string
	keys    map[string]string // filename -> base64 PEM

	mu       sync.Mutex
	sessions map[string]*sessionState
}

type sessionState struct {
	obj *sshpkg.Sshobject
	ses *sshpkg.StreamSession

	fwdSeq int
	fwd    map[string]*forwardHandle
}

// SFTPListResult 表示 SFTP 目录列表操作的返回数据
type SFTPListResult struct {
	Entries []sshpkg.SFTPEntry `json:"entries,omitempty"`
	Error   string             `json:"error,omitempty"`
}

// SFTPDownloadResult 表示 SFTP 下载操作的返回数据
type SFTPDownloadResult struct {
	Data  []byte `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func (b *SSHBridge) startup(ctx context.Context) { b.ctx = ctx }

// ensureSessionLocked returns an existing session or creates a new entry.
func (b *SSHBridge) ensureSessionLocked(id string) *sessionState {
	if id == "" {
		return nil
	}
	if b.sessions == nil {
		b.sessions = make(map[string]*sessionState)
	}
	sess, ok := b.sessions[id]
	if !ok {
		sess = &sessionState{}
		b.sessions[id] = sess
	}
	return sess
}

// getSessionLocked returns a session if present. Caller must hold the mutex.
func (b *SSHBridge) getSessionLocked(id string) *sessionState {
	if id == "" || b.sessions == nil {
		return nil
	}
	return b.sessions[id]
}

// requireSessionObject 取得已初始化的 SSH 会话对象
func (b *SSHBridge) requireSessionObject(sessionID string) (*sshpkg.Sshobject, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("invalid session id")
	}
	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.obj == nil {
		b.mu.Unlock()
		return nil, fmt.Errorf("ssh object not initialized")
	}
	obj := sess.obj
	b.mu.Unlock()
	return obj, nil
}

func (b *SSHBridge) closeSessionLocked(id string, sess *sessionState, remove bool) {
	if sess == nil {
		return
	}
	_ = stopForwardsLocked(sess)
	if sess.ses != nil {
		_ = sess.ses.Close()
		sess.ses = nil
	}
	if sess.obj != nil {
		sshpkg.Close(sess.obj)
		sess.obj = nil
	}
	if remove && b.sessions != nil && id != "" {
		delete(b.sessions, id)
	}
}

func stopForwardsLocked(sess *sessionState) error {
	if sess == nil || len(sess.fwd) == 0 {
		return nil
	}
	var firstErr error
	for id, h := range sess.fwd {
		if h != nil && h.cancel != nil {
			if err := h.cancel(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		delete(sess.fwd, id)
	}
	sess.fwdSeq = 0
	return firstErr
}

// InitWithPasswd initializes an SSH object with username/password auth.
func (b *SSHBridge) InitWithPasswd(sessionID, host, user, passwd string) {
	if sessionID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	sess := b.ensureSessionLocked(sessionID)
	b.closeSessionLocked(sessionID, sess, false)
	sess.obj = sshpkg.InitWithPasswd(host, user, passwd)
}

// InitWithPem initializes an SSH object with private key auth using base64 pem data.
// The third argument should be a base64-encoded PEM key content.
func (b *SSHBridge) InitWithPem(sessionID, host, user, pemBase64 string) {
	if sessionID == "" {
		return
	}
	data, err := base64.StdEncoding.DecodeString(pemBase64)
	if err != nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	sess := b.ensureSessionLocked(sessionID)
	b.closeSessionLocked(sessionID, sess, false)
	sess.obj = sshpkg.InitWithPem(host, user, data)
}

// ---- Simple keystore persisted to JSON ----
func (b *SSHBridge) ensureKeyStorePath() string {
	if b.keyFile != "" {
		return b.keyFile
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	appdir := filepath.Join(dir, "Erban")
	_ = os.MkdirAll(appdir, 0o755)
	b.keyFile = filepath.Join(appdir, "keys.json")
	return b.keyFile
}

func (b *SSHBridge) loadKeys() {
	var path string
	if b.keys != nil {
		return
	}
	b.keys = map[string]string{}

	if b.keyFile == "" {
		path = b.ensureKeyStorePath()
	} else {
		path = b.keyFile
	}

	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return
	}
	_ = json.Unmarshal(data, &b.keys)
}

func (b *SSHBridge) saveKeys() error {
	if b.keys == nil {
		b.keys = map[string]string{}
	}

	data, err := json.MarshalIndent(b.keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(b.keyFile, data, 0o600)
}

// ----- Port forwarding (local/remote/dynamic) -----

type forwardHandle struct {
	id     string
	mode   string // local | remote | dynamic
	from   string
	to     string // target or ""
	cancel func() error
}

// StartLocalForward starts a local forward: localAddr => remoteAddr via SSH.
// Returns empty string on success; otherwise error text.
func (b *SSHBridge) StartLocalForward(sessionID, localAddr, remoteAddr string) string {
	if sessionID == "" {
		return "invalid session id"
	}

	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.obj == nil {
		b.mu.Unlock()
		return "ssh object not initialized"
	}
	obj := sess.obj
	b.mu.Unlock()

	cancel, err := obj.LocalForward(localAddr, remoteAddr)
	if err != nil {
		return err.Error()
	}

	b.mu.Lock()
	sess = b.getSessionLocked(sessionID)
	if sess == nil || sess.obj != obj {
		b.mu.Unlock()
		if cancel != nil {
			_ = cancel()
		}
		return "session not available"
	}
	if sess.fwd == nil {
		sess.fwd = map[string]*forwardHandle{}
	}
	sess.fwdSeq++
	id := fmt.Sprintf("lf-%d", sess.fwdSeq)
	sess.fwd[id] = &forwardHandle{id: id, mode: "local", from: localAddr, to: remoteAddr, cancel: cancel}
	b.mu.Unlock()

	return ""
}

// StartRemoteForward starts a remote forward: remoteBind => localTarget via SSH.
// Returns empty string on success; otherwise error text.
func (b *SSHBridge) StartRemoteForward(sessionID, remoteBind, localTarget string) string {
	if sessionID == "" {
		return "invalid session id"
	}

	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.obj == nil {
		b.mu.Unlock()
		return "ssh object not initialized"
	}
	obj := sess.obj
	b.mu.Unlock()

	cancel, err := obj.RemoteForward(remoteBind, localTarget)
	if err != nil {
		return err.Error()
	}

	b.mu.Lock()
	sess = b.getSessionLocked(sessionID)
	if sess == nil || sess.obj != obj {
		b.mu.Unlock()
		if cancel != nil {
			_ = cancel()
		}
		return "session not available"
	}
	if sess.fwd == nil {
		sess.fwd = map[string]*forwardHandle{}
	}
	sess.fwdSeq++
	id := fmt.Sprintf("rf-%d", sess.fwdSeq)
	sess.fwd[id] = &forwardHandle{id: id, mode: "remote", from: remoteBind, to: localTarget, cancel: cancel}
	b.mu.Unlock()

	return ""
}

// StartDynamicForward starts a SOCKS5 proxy bound on localSocks that tunnels via SSH.
// Returns empty string on success; otherwise error text.
func (b *SSHBridge) StartDynamicForward(sessionID, localSocks string) string {
	if sessionID == "" {
		return "invalid session id"
	}

	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.obj == nil {
		b.mu.Unlock()
		return "ssh object not initialized"
	}
	obj := sess.obj
	b.mu.Unlock()

	cancel, err := obj.DynamicForward(localSocks)
	if err != nil {
		return err.Error()
	}

	b.mu.Lock()
	sess = b.getSessionLocked(sessionID)
	if sess == nil || sess.obj != obj {
		b.mu.Unlock()
		if cancel != nil {
			_ = cancel()
		}
		return "session not available"
	}
	if sess.fwd == nil {
		sess.fwd = map[string]*forwardHandle{}
	}
	sess.fwdSeq++
	id := fmt.Sprintf("df-%d", sess.fwdSeq)
	sess.fwd[id] = &forwardHandle{id: id, mode: "dynamic", from: localSocks, to: "", cancel: cancel}
	b.mu.Unlock()

	return ""
}

// ListForwards returns a JSON array of current forwards with id/mode/from/to.
func (b *SSHBridge) ListForwards(sessionID string) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	sess := b.getSessionLocked(sessionID)
	if sess == nil || len(sess.fwd) == 0 {
		return "[]"
	}

	type item struct{ ID, Mode, From, To string }
	out := make([]item, 0, len(sess.fwd))
	for _, h := range sess.fwd {
		out = append(out, item{ID: h.id, Mode: h.mode, From: h.from, To: h.to})
	}
	data, _ := json.Marshal(out)
	return string(data)
}

// StopForward stops a specific forward by id returned in ListForwards.
// Returns empty string on success; otherwise error text.
func (b *SSHBridge) StopForward(sessionID, id string) string {
	if id == "" {
		return "invalid id"
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.fwd == nil {
		return "not found"
	}
	h, ok := sess.fwd[id]
	if !ok || h == nil {
		return "not found"
	}
	if h.cancel != nil {
		if err := h.cancel(); err != nil {
			return err.Error()
		}
	}
	delete(sess.fwd, id)
	return ""
}

// StopAllForwards cancels all active forwards.
func (b *SSHBridge) StopAllForwards(sessionID string) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	sess := b.getSessionLocked(sessionID)
	if sess == nil {
		return ""
	}
	if err := stopForwardsLocked(sess); err != nil {
		return err.Error()
	}
	return ""
}

// KeyPut stores/updates a key by filename with its base64-encoded content.
// Returns empty string on success; otherwise error text.
func (b *SSHBridge) KeyPut(filename, pemBase64 string) string {
	if filename == "" || pemBase64 == "" {
		return "invalid filename or content"
	}

	b.loadKeys()

	if b.keys == nil {
		b.keys = map[string]string{}
	}
	b.keys[filename] = pemBase64
	if err := b.saveKeys(); err != nil {
		return err.Error()
	}
	return ""
}

// KeyGet returns the base64-encoded content by filename (empty if not present).
func (b *SSHBridge) KeyGet(filename string) string {
	if filename == "" {
		return ""
	}
	b.loadKeys()
	val := b.keys[filename]
	return val
}

// KeyDelete removes a key by filename. Returns empty string on success.
func (b *SSHBridge) KeyDelete(filename string) string {
	if filename == "" {
		return "invalid filename"
	}
	b.loadKeys()

	delete(b.keys, filename)

	if err := b.saveKeys(); err != nil {
		return err.Error()
	}
	return ""
}

// SetProxy configures proxy URL (http/https/socks5), returns error text on failure.
func (b *SSHBridge) SetProxy(sessionID, v string) string {
	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.obj == nil {
		b.mu.Unlock()
		return "ssh object not initialized"
	}
	obj := sess.obj
	b.mu.Unlock()

	if err := sshpkg.SetProxy(obj, v); err != nil {
		return err.Error()
	}
	return ""
}

// CreateClient establishes the SSH connection.
func (b *SSHBridge) CreateClient(sessionID string) string {
	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.obj == nil {
		b.mu.Unlock()
		return "ssh object not initialized"
	}
	obj := sess.obj
	b.mu.Unlock()

	if err := sshpkg.CreateClient(obj); err != nil {
		return err.Error()
	}
	if err := sshpkg.EnsureSFTP(obj); err != nil {
		sshpkg.LogErrorf("SFTP init failed (session=%s): %v", sessionID, err)
		return err.Error()
	}
	return ""
}

// // Start starts an interactive shell session.
// func (b *SSHBridge) Start(sessionID string) string {
// 	b.mu.Lock()
// 	sess := b.getSessionLocked(sessionID)
// 	if sess == nil || sess.obj == nil {
// 		b.mu.Unlock()
// 		return "ssh object not initialized"
// 	}
// 	if sess.ses != nil {
// 		_ = sess.ses.Close()
// 		sess.ses = nil
// 	}
// 	obj := sess.obj
// 	b.mu.Unlock()

// 	ew := &eventWriter{ctx: b.ctx, sessionID: sessionID}
// 	stream, err := sshpkg.StartStream(obj, ew, 56, 120)
// 	if err != nil {
// 		return err.Error()
// 	}

// 	b.mu.Lock()
// 	sess = b.getSessionLocked(sessionID)
// 	if sess == nil || sess.obj != obj {
// 		b.mu.Unlock()
// 		_ = stream.Close()
// 		return "session not available"
// 	}
// 	sess.ses = stream
// 	b.mu.Unlock()

// 	go b.watchSession(sessionID, stream)
// 	return ""
// }

// Connect creates the SSH client and starts the interactive session in one call.
func (b *SSHBridge) Connect(sessionID string) string {
	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.obj == nil {
		b.mu.Unlock()
		return "ssh object not initialized"
	}
	if sess.ses != nil {
		_ = sess.ses.Close()
		sess.ses = nil
	}
	obj := sess.obj
	b.mu.Unlock()

	ew := &eventWriter{ctx: b.ctx, sessionID: sessionID}
	stream, err := sshpkg.ConnectAndStartStream(obj, ew, 40, 120)
	if err != nil {
		return err.Error()
	}
	if err := sshpkg.EnsureSFTP(obj); err != nil {
		sshpkg.LogErrorf("SFTP init failed (session=%s): %v", sessionID, err)
		return err.Error()
	}

	b.mu.Lock()
	sess = b.getSessionLocked(sessionID)
	if sess == nil || sess.obj != obj {
		b.mu.Unlock()
		_ = stream.Close()
		return "session not available"
	}
	sess.ses = stream
	b.mu.Unlock()

	go b.watchSession(sessionID, stream)
	return ""
}

// Close terminates the SSH connection.
func (b *SSHBridge) Close(sessionID string) {
	if sessionID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	sess := b.getSessionLocked(sessionID)
	if sess != nil {
		b.closeSessionLocked(sessionID, sess, true)
	}
}

// Send writes a line to the interactive session (appends newline if missing).
func (b *SSHBridge) Send(sessionID, data string) string {
	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.ses == nil {
		b.mu.Unlock()
		return "session not started"
	}
	stream := sess.ses
	b.mu.Unlock()

	if !endsWithNewline(data) {
		data += "\n"
	}
	if _, err := stream.Write([]byte(data)); err != nil {
		return err.Error()
	}
	return ""
}

// Write forwards raw data to the interactive session without appending a newline.
// This is suitable for integrating with xterm.js `onData` events so special keys
// and control sequences pass through correctly.
func (b *SSHBridge) Write(sessionID, data string) string {
	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.ses == nil {
		b.mu.Unlock()
		return "session not started"
	}
	stream := sess.ses
	b.mu.Unlock()

	if _, err := stream.Write([]byte(data)); err != nil {
		return err.Error()
	}
	return ""
}

// Resize changes the backend PTY size.
func (b *SSHBridge) Resize(sessionID string, rows, cols int) string {
	b.mu.Lock()
	sess := b.getSessionLocked(sessionID)
	if sess == nil || sess.ses == nil {
		b.mu.Unlock()
		return "session not started"
	}
	stream := sess.ses
	b.mu.Unlock()

	if err := stream.Resize(rows, cols); err != nil {
		return err.Error()
	}
	return ""
}

// SFTPList 列出远程服务器目录内容
func (b *SSHBridge) SFTPList(sessionID, remoteDir string) *SFTPListResult {
	if remoteDir == "" {
		remoteDir = "."
	}
	obj, err := b.requireSessionObject(sessionID)
	if err != nil {
		sshpkg.LogErrorf("SFTP list failed: %v", err)
		return &SFTPListResult{Error: err.Error()}
	}

	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	entries, err := sshpkg.SFTPListDirectory(ctx, obj, remoteDir)
	if err != nil {
		sshpkg.LogErrorf("SFTP list failed (session=%s, dir=%s): %v", sessionID, remoteDir, err)
		return &SFTPListResult{Error: err.Error()}
	}
	return &SFTPListResult{Entries: entries}
}

// SFTPUpload 上传内存中的文件内容到远程路径
func (b *SSHBridge) SFTPUpload(sessionID, remotePath string, content []byte) string {
	if remotePath == "" {
		return "invalid remote path"
	}
	obj, err := b.requireSessionObject(sessionID)
	if err != nil {
		sshpkg.LogErrorf("SFTP upload failed: %v", err)
		return err.Error()
	}

	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	if err := sshpkg.SFTPUploadBytes(ctx, obj, remotePath, content, nil); err != nil {
		sshpkg.LogErrorf("SFTP upload failed (session=%s, path=%s): %v", sessionID, remotePath, err)
		return err.Error()
	}
	return ""
}

// SFTPResumeUpload 以断点续传方式上传文件
func (b *SSHBridge) SFTPResumeUpload(sessionID, remotePath string, content []byte) string {
	if remotePath == "" {
		return "invalid remote path"
	}
	obj, err := b.requireSessionObject(sessionID)
	if err != nil {
		sshpkg.LogErrorf("SFTP resume upload failed: %v", err)
		return err.Error()
	}

	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	if err := sshpkg.SFTPResumeUploadBytes(ctx, obj, remotePath, content, nil); err != nil {
		sshpkg.LogErrorf("SFTP resume upload failed (session=%s, path=%s): %v", sessionID, remotePath, err)
		return err.Error()
	}
	return ""
}

// SFTPDownload 下载远程文件并返回字节数据
func (b *SSHBridge) SFTPDownload(sessionID, remotePath string) *SFTPDownloadResult {
	if remotePath == "" {
		return &SFTPDownloadResult{Error: "invalid remote path"}
	}
	obj, err := b.requireSessionObject(sessionID)
	if err != nil {
		sshpkg.LogErrorf("SFTP download failed: %v", err)
		return &SFTPDownloadResult{Error: err.Error()}
	}

	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	data, err := sshpkg.SFTPDownloadBytes(ctx, obj, remotePath, nil)
	if err != nil {
		sshpkg.LogErrorf("SFTP download failed (session=%s, path=%s): %v", sessionID, remotePath, err)
		return &SFTPDownloadResult{Error: err.Error()}
	}
	return &SFTPDownloadResult{Data: data}
}

func (b *SSHBridge) watchSession(sessionID string, stream *sshpkg.StreamSession) {
	if stream == nil {
		return
	}
	<-stream.Done()
	runtime.EventsEmit(b.ctx, fmt.Sprintf("ssh:ended:%s", sessionID))
	b.mu.Lock()
	defer b.mu.Unlock()

	sess := b.getSessionLocked(sessionID)
	if sess == nil {
		return
	}
	if sess.ses == stream {
		sess.ses = nil
	}
	_ = stopForwardsLocked(sess)
}

// eventWriter emits SSH output chunks to the frontend as events.
type eventWriter struct {
	ctx       context.Context
	sessionID string
}

func (w *eventWriter) Write(p []byte) (int, error) {
	if w == nil || w.ctx == nil {
		return len(p), nil
	}
	runtime.EventsEmit(w.ctx, fmt.Sprintf("ssh:output:%s", w.sessionID), string(p))
	return len(p), nil
}

func endsWithNewline(s string) bool {
	if s == "" {
		return false
	}
	r := s[len(s)-1]
	return r == '\n' || r == '\r'
}

// ChatBridge exposes OpenAI chat methods to the frontend with cancel support.
type ChatBridge struct {
	ctx      context.Context
	mu       sync.Mutex
	sessions map[string]*chatSession
	cfg      *chatmodel.Mc
}

type chatSession struct {
	chat   *chatmodel.Chat
	cancel context.CancelFunc
	pacer  *chatoutput.Pacer
}

func (c *ChatBridge) startup(ctx context.Context) { c.ctx = ctx }

// OpenAI configures the global AI settings (decoupled from SSH/tab sessions).
// proxy must be a full URL like "http://127.0.0.1:10808" or empty to use environment.
// provider specifies the API format: "openai" (default), "anthropic", or "gemini" (currently used by adapters).
// baseurl is the custom API URL; if empty, uses official API based on provider.
// Returns empty string on success, otherwise an error message.
func (c *ChatBridge) OpenAI(proxy, model, reason, provider, key, baseurl string) string {
	var u url.URL
	if proxy != "" {
		pu, err := url.Parse(proxy)
		if err != nil {
			return err.Error()
		}
		if pu != nil {
			u = *pu
		}
	}

	mc := &chatmodel.Mc{
		Proxy:   u,
		Model:   model,
		Reason:  reason,
		Key:     key,
		Baseurl: baseurl,
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Store global config; new chat sessions will be created lazily with this config.
	c.cfg = mc
	// Invalidate existing session chat objects so they pick up new config on next Start.
	if c.sessions != nil {
		for _, s := range c.sessions {
			if s != nil {
				// Do not cancel running sessions here; just force re-init on next Start.
				s.chat = nil
			}
		}
	}
	return ""
}

// Start begins a streaming chat for the session. It emits output chunks via
// Wails events channel: "chat:output:<sessionID>" and emits "chat:ended:<sessionID>" when done.
func (c *ChatBridge) Start(sessionID, text string) string {
	if sessionID == "" {
		return "invalid session id"
	}
	c.mu.Lock()
	if c.sessions == nil {
		c.sessions = make(map[string]*chatSession)
	}
	sess := c.sessions[sessionID]
	if sess == nil {
		sess = &chatSession{}
		c.sessions[sessionID] = sess
	}
	if sess.chat == nil {
		if c.cfg == nil {
			c.mu.Unlock()
			return "chat not configured"
		}
		sess.chat = chatmodel.Openai(c.cfg)
	}
	// Cancel any previous run
	if sess.cancel != nil {
		sess.cancel()
		sess.cancel = nil
	}
	if sess.pacer != nil {
		sess.pacer.Cancel()
		sess.pacer = nil
	}

	// Create cancelable context derived from app context
	ctx, cancel := context.WithCancel(c.ctx)
	sess.cancel = cancel

	// Writer that forwards pacer output to frontend events
	w := &chatEventWriter{ctx: c.ctx, sessionID: sessionID}
	p := chatoutput.NewPacerOut(500, 10*time.Millisecond, 15, w)
	sess.pacer = p
	c.mu.Unlock()

	go func() {
		sess.chat.Start(ctx, text, p)
		runtime.EventsEmit(c.ctx, fmt.Sprintf("chat:ended:%s", sessionID))
		c.mu.Lock()
		if s := c.sessions[sessionID]; s != nil {
			s.cancel = nil
			s.pacer = nil
		}
		c.mu.Unlock()
	}()

	return ""
}

// Cancel stops an in-progress chat for the given session.
func (c *ChatBridge) Cancel(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if sessionID == "" {
		return
	}
	sess := c.sessions[sessionID]
	if sess == nil {
		return
	}
	if sess.cancel != nil {
		sess.cancel()
		sess.cancel = nil
	}
	if sess.pacer != nil {
		sess.pacer.Cancel()
		sess.pacer = nil
	}
}

// chatEventWriter emits chat output chunks to the frontend.
type chatEventWriter struct {
	ctx       context.Context
	sessionID string
}

func (w *chatEventWriter) Write(p []byte) (int, error) {
	if w == nil || w.ctx == nil {
		return len(p), nil
	}
	runtime.EventsEmit(w.ctx, fmt.Sprintf("chat:output:%s", w.sessionID), string(p))
	return len(p), nil
}
