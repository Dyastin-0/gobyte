package gobyte

import (
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

func DefaultBar(n int64, text string, p *mpb.Progress) *mpb.Bar {
	bar := p.AddBar(n,
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
