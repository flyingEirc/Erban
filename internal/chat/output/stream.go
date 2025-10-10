package output

import (
    "bufio"
    "context"
    "io"
    "os"
    "sync"
    "time"
    "unicode/utf8"
)

type Pacer struct {
	mu       sync.Mutex
	cps      float64       // 目标 characters per second
	frameDur time.Duration // 帧间隔，建议 16~33ms
	burst    int           // 允许的突发字符数
	writer   *bufio.Writer // 包装的输出

	buf       []rune
	released  int // 已经释放给终端的字符总数
	startedAt time.Time
	ctx       context.Context
	Cancel    context.CancelFunc
	Done      chan struct{}
}

func NewPacer(cps float64, frameDur time.Duration, burst int) *Pacer {
    return NewPacerOut(cps, frameDur, burst, os.Stdout)
}

// NewPacerOut is like NewPacer but writes to a custom io.Writer.
// If w is nil, it falls back to os.Stdout.
func NewPacerOut(cps float64, frameDur time.Duration, burst int, w io.Writer) *Pacer {
    if w == nil {
        w = os.Stdout
    }
    ctx, cancel := context.WithCancel(context.Background())
    return &Pacer{
        mu:       sync.Mutex{},
        cps:      cps,
        frameDur: frameDur,
        burst:    burst,
        writer:   bufio.NewWriterSize(w, 1<<15), // 32KB 缓冲
        ctx:      ctx,
        Cancel:   cancel,
        Done:     make(chan struct{}),
    }
}

func (p *Pacer) Feed(s string) {
	runes := make([]rune, 0, len(s))
	for len(s) > 0 {
		if s[0] < utf8.RuneSelf {
			runes = append(runes, rune(s[0]))
			s = s[1:]
			continue
		}

		r, size := utf8.DecodeRuneInString(s)
		runes = append(runes, r)
		s = s[size:]
	}
	p.buf = append(p.buf, runes...)
}

func (p *Pacer) Start() {
	p.startedAt = time.Now()
	ticker := time.NewTicker(p.frameDur)
	defer ticker.Stop()

	if p.burst > 0 {
		n := min(p.burst, len(p.buf))
		p.writeNRunes(n)
		p.writer.Flush()
	}

	for {
		select {
		case <-p.ctx.Done():
			avail := len(p.buf)
			if avail > 0 {
				p.writeNRunes(avail)
			}
			p.Done <- struct{}{}
			return
		case now := <-ticker.C:
			elapsed := now.Sub(p.startedAt).Seconds()
			target := int(elapsed * p.cps)
			need := target - p.released
			if need <= 0 {
				continue
			}
			maxPerFrame := max(1, int(p.cps*float64(p.frameDur)/float64(time.Second))+2)
			need = min(need, maxPerFrame)

			avail := len(p.buf)
			n := min(need, avail)
			if n > 0 {
				p.writeNRunes(n)
			}

			_ = p.writer.Flush()
		}
	}
}

func (p *Pacer) writeNRunes(n int) {
	for i := 0; i < n; i++ {
		_, _ = p.writer.WriteString(string(p.buf[i]))
	}
	p.buf = p.buf[n:]
	p.released += n
}

func (p *Pacer) Wait() {
	<-p.Done
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
