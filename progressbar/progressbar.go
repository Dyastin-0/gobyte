package progressbar

import (
	"io"
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type ProgressBar struct {
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

func New() *ProgressBar {
	return &ProgressBar{
		progress: mpb.New(),
	}
}

func (pb *ProgressBar) NewBar(dst io.Writer, src io.Reader, n int64, text string) *mpb.Bar {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	bar := pb.progress.AddBar(n,
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

func (pb *ProgressBar) Execute(dst io.Writer, src io.Reader, n int64, bar *mpb.Bar) (int64, error) {
	proxyReader := &CopyN{
		reader: src,
		bar:    bar,
	}
	return io.CopyN(dst, proxyReader, n)
}

func (pb *ProgressBar) Wait() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.progress.Wait()
}

func (pb *ProgressBar) Reset() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.progress != nil {
		pb.progress.Wait()
	}

	pb.progress = mpb.New()
}
