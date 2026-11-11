/*
|--------------------------------------------------------------------------
| Knowledge Source Preview Resource
|--------------------------------------------------------------------------
|
| How the add flow's preview reads over the API: the account a source's
| settings reach and how much of it they would read, with a sample.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/knowledgesources"
)

/*
KnowledgeSourcePreview is what a source would read before it is saved.
*/
type KnowledgeSourcePreview struct {
	Object   string                         `json:"object"`
	Account  string                         `json:"account"`
	Estimate int                            `json:"estimate"`
	Listed   int                            `json:"listed"`
	Matched  int                            `json:"matched"`
	More     bool                           `json:"more"`
	Sample   []KnowledgeSourcePreviewSample `json:"sample"`
}

/*
KnowledgeSourcePreviewSample is one document of a preview's sample.
*/
type KnowledgeSourcePreviewSample struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	URL   string `json:"url"`
}

/*
NewKnowledgeSourcePreview shapes a preview.
*/
func NewKnowledgeSourcePreview(p knowledgesources.PreviewView) KnowledgeSourcePreview {
	out := KnowledgeSourcePreview{
		Object: "knowledge_source_preview", Account: p.Account, Estimate: p.Estimate, Listed: p.Listed,
		Matched: p.Matched, More: p.More, Sample: []KnowledgeSourcePreviewSample{},
	}

	for _, s := range p.Sample {
		out.Sample = append(out.Sample, KnowledgeSourcePreviewSample(s))
	}

	return out
}
