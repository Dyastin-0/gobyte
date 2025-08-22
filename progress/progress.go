package progress

import (
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type Progress struct {
	progress *mpb.Progress
}

func New() *Progress {
	return &Progress{
		progress: mpb.New(),
	}
}

func (p *Progress) NewBar(n int64, text string) *mpb.Bar {
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

func (p *Progress) Wait() {
	p.progress.Wait()
}

func (p *Progress) Reset() {
	if p.progress != nil {
		p.progress.Wait()
	}

	p.progress = mpb.New()
}
