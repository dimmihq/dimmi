/*
|--------------------------------------------------------------------------
| Source Preview
|--------------------------------------------------------------------------
|
| Before a source is saved, its settings are tried against the real
| service: the account they reach, how many documents the first page
| lists, how many of those the patterns keep, and a few of them by name.
|
*/

package knowledgesources

import (
	"context"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
Preview checks the settings and counts what the filters would keep of
the listing's first page, for the add flow's live count.
*/
func (e *Engine) Preview(ctx context.Context, in SourceInput) (PreviewView, error) {
	if !in.Membership.Can("knowledge:manage") {
		return PreviewView{}, exceptions.ErrForbidden
	}

	d, err := e.driverOf(in.Type)
	if err != nil {
		return PreviewView{}, err
	}

	cfg, err := e.connected(ctx, in.Membership.Workspace.ID, d, configOf(splitSecrets(d.Describe(), in.Settings)))
	if err := checkError(err); err != nil {
		return PreviewView{}, err
	}

	result, err := d.Check(ctx, cfg)
	if err := checkError(err); err != nil {
		return PreviewView{}, err
	}

	page, err := d.List(ctx, cfg, "")
	if err := checkError(err); err != nil {
		return PreviewView{}, err
	}

	out := PreviewView{Account: result.Account, Estimate: result.Estimate, More: page.Next != ""}
	for _, r := range page.Refs {
		out.Listed++

		if sources.Allowed(clean(in.Include), clean(in.Exclude), r.Path) {
			out.Matched++
			if len(out.Sample) < 8 {
				out.Sample = append(out.Sample, PreviewItem{Title: r.Title, Path: r.Path, URL: r.URL})
			}
		}
	}

	return out, nil
}

/*
PreviewView is what the add flow shows before a source is saved: the
account reached, the service's own estimate, how many documents the
first page listed and the filters keep, a sample, and whether more
pages follow.
*/
type PreviewView struct {
	Account  string
	Estimate int
	Listed   int
	Matched  int
	More     bool
	Sample   []PreviewItem
}

/*
PreviewItem is one document of the preview's sample.
*/
type PreviewItem struct {
	Title string
	Path  string
	URL   string
}
