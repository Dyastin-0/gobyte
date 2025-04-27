package progress

import (
	"io"
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type Progress struct {
	progress *mpb.Progress
	mu       sync.Mutex
}

type CopyN struct {
	reader io.Reader
	bar    *mpb.Bar
}

func (c *CopyN) Read(p []byte) (n int, err error) {
	n, err = c.reader.Read(p)
	c.bar.IncrBy(n)
	return
}

func New() *Progress {
	return &Progress{
		progress: mpb.New(),
	}
}

func (p *Progress) NewBar(dst io.Writer, src io.Reader, n int64, text string) *mpb.Bar {
	p.mu.Lock()
	defer p.mu.Unlock()

	bar := p.progress.AddBar(n,
		mpb.PrependDecorators(
			decor.Name(text, decor.WC{W: 12, C: decor.DindentRight}),
			decor.CountersKibiByte(" % .2f / % .2f", decor.WCSyncWidth),
		),
		mpb.AppendDecorators(
			decor.Elapsed(1, decor.WC{W: 12, C: decor.DindentRight}),
		),
	)

	return bar
}

func (p *Progress) Execute(dst io.Writer, src io.Reader, n int64, bar *mpb.Bar) (int64, error) {
	proxyReader := &CopyN{
		reader: src,
		bar:    bar,
	}
	return io.CopyN(dst, proxyReader, n)
}

func (p *Progress) Wait() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.progress.Wait()
}

func (p *Progress) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.progress != nil {
		p.progress.Wait()
	}

	p.progress = mpb.New()
}
