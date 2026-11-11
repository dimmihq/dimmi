/*
|--------------------------------------------------------------------------
| Knowledge Source Options
|--------------------------------------------------------------------------
|
| The values a setting can take, listed from the connected service, so
| a repository or a branch is picked from a list instead of typed, and
| what the workspace's knowledge cost this month.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
KnowledgeOptions is the values a setting can take.
*/
type KnowledgeOptions struct {
	Object  string           `json:"object"`
	Options []sources.Option `json:"options"`
}

/*
KnowledgeOptionsOutput wraps the values a setting can take.
*/
type KnowledgeOptionsOutput struct {
	Body fhttp.Resource[KnowledgeOptions]
}

/*
Options lists the values a setting of a source being added can take.
*/
func (c *KnowledgeSourceController) Options(ctx context.Context, in *requests.KnowledgeOptionsRequest,
) (*KnowledgeOptionsOutput, error) {
	input, err := c.input(ctx, requests.KnowledgeSourceBody{Type: in.Body.Type, Settings: in.Body.Settings})
	if err != nil {
		return nil, err
	}

	options, err := c.Engine.Options(ctx, input, in.Body.Field)
	if err != nil {
		return nil, err
	}

	if options == nil {
		options = []sources.Option{}
	}

	out := KnowledgeOptions{Object: "knowledge_source_options", Options: options}

	return &KnowledgeOptionsOutput{Body: fhttp.NewResource(out)}, nil
}

/*
KnowledgeUsageOutput is the month's knowledge usage.
*/
type KnowledgeUsageOutput struct {
	Body fhttp.Resource[resources.KnowledgeUsage]
}

/*
Usage shows what the workspace's knowledge cost this month, per source.
*/
func (c *KnowledgeSourceController) Usage(ctx context.Context, _ *requests.NoInput) (*KnowledgeUsageOutput, error) {
	lines, err := c.Engine.Usage(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	period := knowledge.MonthStart(c.Engine.Clock.Now()).Format("2006-01-02")

	return &KnowledgeUsageOutput{Body: fhttp.NewResource(resources.NewKnowledgeUsage(period, lines))}, nil
}
