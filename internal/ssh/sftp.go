package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// ProgressCallback 进度回调函数类型
type ProgressCallback func(written, total int64)

// TransferOptions 传输选项
type TransferOptions struct {
	ProgressCallback ProgressCallback
	UpdateInterval   time.Duration // 进度更新间隔，默认 200ms
}

// SFTPEntry 表示目录中的单个文件或文件夹信息
type SFTPEntry struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
}

// newSFTP 创建新的 SFTP 客户端
func newSFTP(sshClient *ssh.Client) (*sftp.Client, error) {
	return sftp.NewClient(sshClient, sftp.MaxPacket(1<<15))
}

// listDir 返回指定目录下的文件列表
func listDir(s *sftp.Client, dir string) ([]SFTPEntry, error) {
	entries, err := s.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]SFTPEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, SFTPEntry{
			Name:    e.Name(),
			Size:    e.Size(),
			Mode:    e.Mode().String(),
			ModTime: e.ModTime(),
			IsDir:   e.IsDir(),
		})
	}
	return out, nil
}

// contextReader 支持通过 context 取消的 Reader
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *contextReader) Read(p []byte) (int, error) {
	select {
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	default:
		return cr.r.Read(p)
	}
}

// progressWriter 带进度回调的 Writer
type progressWriter struct {
	total      int64
	totalSize  int64
	lastTick   time.Time
	interval   time.Duration
	onProgress ProgressCallback
}

func newProgressWriter(totalSize int64, callback ProgressCallback, interval time.Duration) *progressWriter {
	if interval == 0 {
		interval = 200 * time.Millisecond
	}
	return &progressWriter{
		totalSize:  totalSize,
		lastTick:   time.Now(),
		interval:   interval,
		onProgress: callback,
	}
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.total += int64(n)

	if time.Since(p.lastTick) >= p.interval {
		p.lastTick = time.Now()
		if p.onProgress != nil {
			p.onProgress(p.total, p.totalSize)
		}
	}
	return n, nil
}

// Flush 确保最终进度被回调一次
func (p *progressWriter) Flush() {
	if p.onProgress != nil {
		p.onProgress(p.total, p.totalSize)
	}
}

// upload 上传内存中的文件，支持 context 和进度回调
func upload(ctx context.Context, s *sftp.Client, content []byte, remote string, opts *TransferOptions) error {
	if opts == nil {
		opts = &TransferOptions{}
	}
	if content == nil {
		content = []byte{}
	}

	dst, err := s.Create(remote)
	if err != nil {
		return fmt.Errorf("create remote file: %w", err)
	}
	defer dst.Close()

	totalSize := int64(len(content))
	reader := bytes.NewReader(content)

	ctxReader := &contextReader{ctx: ctx, r: reader}
	pw := newProgressWriter(totalSize, opts.ProgressCallback, opts.UpdateInterval)

	if _, err = io.Copy(io.MultiWriter(dst, pw), ctxReader); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	pw.Flush()
	return nil
}

// resumeUpload 使用内存内容执行断点续传上传
func resumeUpload(ctx context.Context, s *sftp.Client, content []byte, remote string, opts *TransferOptions) error {
	if opts == nil {
		opts = &TransferOptions{}
	}
	if content == nil {
		content = []byte{}
	}

	stat, err := s.Stat(remote)
	var offset int64
	if err == nil {
		offset = stat.Size()
	}
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(content)) {
		offset = int64(len(content))
	}

	dst, err := s.OpenFile(remote, os.O_WRONLY|os.O_CREATE|os.O_APPEND)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer dst.Close()

	totalSize := int64(len(content))
	reader := bytes.NewReader(content[offset:])

	ctxReader := &contextReader{ctx: ctx, r: reader}

	var wrapped ProgressCallback
	if opts.ProgressCallback != nil {
		wrapped = func(written, _ int64) {
			opts.ProgressCallback(offset+written, totalSize)
		}
	}
	pw := newProgressWriter(totalSize, wrapped, opts.UpdateInterval)

	if _, err = io.Copy(io.MultiWriter(dst, pw), ctxReader); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	pw.Flush()
	return nil
}

// download 下载远程文件到本地路径
func download(ctx context.Context, s *sftp.Client, remote, local string, opts *TransferOptions) error {
	if opts == nil {
		opts = &TransferOptions{}
	}

	src, err := s.Open(remote)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat remote file: %w", err)
	}
	totalSize := stat.Size()

	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return fmt.Errorf("create local dir: %w", err)
	}

	dst, err := os.Create(local)
	if err != nil {
		return fmt.Errorf("create local file: %w", err)
	}
	defer dst.Close()

	ctxReader := &contextReader{ctx: ctx, r: src}
	pw := newProgressWriter(totalSize, opts.ProgressCallback, opts.UpdateInterval)

	if _, err = io.Copy(io.MultiWriter(dst, pw), ctxReader); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	pw.Flush()
	return nil
}

// downloadToBuffer 下载远程文件并返回内存数据
func downloadToBuffer(ctx context.Context, s *sftp.Client, remote string, opts *TransferOptions) ([]byte, error) {
	if opts == nil {
		opts = &TransferOptions{}
	}

	src, err := s.Open(remote)
	if err != nil {
		return nil, fmt.Errorf("open remote file: %w", err)
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat remote file: %w", err)
	}
	totalSize := stat.Size()

	buf := bytes.NewBuffer(make([]byte, 0, totalSize))

	ctxReader := &contextReader{ctx: ctx, r: src}
	pw := newProgressWriter(totalSize, opts.ProgressCallback, opts.UpdateInterval)

	if _, err = io.Copy(io.MultiWriter(buf, pw), ctxReader); err != nil {
		return nil, fmt.Errorf("copy data: %w", err)
	}

	pw.Flush()
	return buf.Bytes(), nil
}

// EnsureSFTP 确保当前 SSH 会话持有一个有效的 SFTP 客户端
func EnsureSFTP(obj *Sshobject) error {
	if obj == nil {
		return fmt.Errorf("nil ssh object")
	}
	if obj.Ftp != nil {
		return nil
	}
	if obj.client == nil {
		if err := CreateClient(obj); err != nil {
			return err
		}
	}
	if obj.client == nil {
		return fmt.Errorf("ssh client unavailable")
	}
	client, err := newSFTP(obj.client)
	if err != nil {
		return err
	}
	obj.Ftp = client
	return nil
}

// ensureSFTPClient 返回一个可用的 SFTP 客户端实例
func ensureSFTPClient(obj *Sshobject) (*sftp.Client, error) {
	if err := EnsureSFTP(obj); err != nil {
		return nil, err
	}
	return obj.Ftp, nil
}

// SFTPListDirectory 列出远程目录内容
func SFTPListDirectory(ctx context.Context, obj *Sshobject, dir string) ([]SFTPEntry, error) {
	client, err := ensureSFTPClient(obj)
	if err != nil {
		return nil, err
	}
	return listDir(client, dir)
}

// SFTPUploadBytes 将内存中的文件内容上传到远程服务器
func SFTPUploadBytes(ctx context.Context, obj *Sshobject, remote string, content []byte, opts *TransferOptions) error {
	client, err := ensureSFTPClient(obj)
	if err != nil {
		return err
	}
	return upload(ctx, client, content, remote, opts)
}

// SFTPResumeUploadBytes 使用内存内容进行断点续传
func SFTPResumeUploadBytes(ctx context.Context, obj *Sshobject, remote string, content []byte, opts *TransferOptions) error {
	client, err := ensureSFTPClient(obj)
	if err != nil {
		return err
	}
	return resumeUpload(ctx, client, content, remote, opts)
}

// SFTPDownloadToFile 下载远程文件到本地路径
func SFTPDownloadToFile(ctx context.Context, obj *Sshobject, remote, local string, opts *TransferOptions) error {
	client, err := ensureSFTPClient(obj)
	if err != nil {
		return err
	}
	return download(ctx, client, remote, local, opts)
}

// SFTPDownloadBytes 下载远程文件并返回字节内容
func SFTPDownloadBytes(ctx context.Context, obj *Sshobject, remote string, opts *TransferOptions) ([]byte, error) {
	client, err := ensureSFTPClient(obj)
	if err != nil {
		return nil, err
	}
	return downloadToBuffer(ctx, client, remote, opts)
}
