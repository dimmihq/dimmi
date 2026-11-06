/*
|--------------------------------------------------------------------------
| AI Trace Controller
|--------------------------------------------------------------------------
|
| What the agent did and what it would do: a conversation's runs and
| one run's steps for the inbox's "why" panel, a month's usage for the
| settings overview, and the playground's test questions. The actions
| decide who may see what; this only maps ids and shapes.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AITraceController serves the agent's runs, usage and playground.
*/
type AITraceController struct {
	ListRuns   *ai.ListConversationRuns
	ShowRun    *ai.ShowRun
	ShowUsage  *ai.ShowUsage
	Playground *ai.AskPlayground
	IDs        *ids.Registry
}

/*
AIRunsOutput is a conversation's run list.
*/
type AIRunsOutput struct {
	Body fhttp.Collection[resources.AiRun]
}

/*
AIRunOutput is one run with its steps.
*/
type AIRunOutput struct {
	Body fhttp.Resource[resources.AiRun]
}

/*
AIUsageOutput is one month's usage.
*/
type AIUsageOutput struct {
	Body fhttp.Resource[resources.AiUsage]
}

/*
AIPlaygroundOutput is what the agent would do with a test question.
*/
type AIPlaygroundOutput struct {
	Body fhttp.Resource[resources.AiPlaygroundAnswer]
}

/*
Runs lists a conversation's agent runs, newest first.
*/
func (c *AITraceController) Runs(ctx context.Context, in *requests.AIRunsRequest) (*AIRunsOutput, error) {
	conv, err := c.decode(models.PrefixConversation, in.Conversation)
	if err != nil {
		return nil, err
	}

	runs, err := c.ListRuns.Handle(ctx, ai.ListRunsInput{Membership: membership(ctx), ConversationID: conv})
	if err != nil {
		return nil, err
	}

	out := make([]resources.AiRun, len(runs))
	for i, r := range runs {
		out[i] = resources.NewAiRun(r)
	}
	return &AIRunsOutput{Body: fhttp.NewCollection(pagination.Page[resources.AiRun]{Items: out, Limit: 100})}, nil
}

/*
Run shows one run with every traced step.
*/
func (c *AITraceController) Run(ctx context.Context, in *requests.AIRunRequest) (*AIRunOutput, error) {
	conv, err := c.decode(models.PrefixConversation, in.Conversation)
	if err != nil {
		return nil, err
	}
	run, err := c.decode(models.PrefixAiRun, in.Run)
	if err != nil {
		return nil, err
	}

	view, err := c.ShowRun.Handle(ctx, ai.ShowRunInput{Membership: membership(ctx), ConversationID: conv, RunID: run})
	if err != nil {
		return nil, err
	}
	return &AIRunOutput{Body: fhttp.NewResource(resources.NewAiRunView(*view))}, nil
}

/*
Usage answers one month's usage.
*/
func (c *AITraceController) Usage(ctx context.Context, in *requests.AIUsageRequest) (*AIUsageOutput, error) {
	view, err := c.ShowUsage.Handle(ctx, ai.UsageInput{Membership: membership(ctx), Period: in.Period})
	if err != nil {
		return nil, err
	}
	return &AIUsageOutput{Body: fhttp.NewResource(resources.NewAiUsage(*view))}, nil
}

/*
Ask runs a test question through the agent.
*/
func (c *AITraceController) Ask(ctx context.Context, in *requests.AIPlaygroundRequest) (*AIPlaygroundOutput, error) {
	input := ai.PlaygroundInput{Membership: membership(ctx), Message: in.Body.Message}
	if in.Body.Conversation != "" {
		conv, err := c.decode(models.PrefixConversation, in.Body.Conversation)
		if err != nil {
			return nil, err
		}
		input.ConversationID = &conv
	}

	answer, err := c.Playground.Handle(ctx, input)
	if err != nil {
		return nil, err
	}
	return &AIPlaygroundOutput{Body: fhttp.NewResource(resources.NewAiPlaygroundAnswer(*answer))}, nil
}

/*
decode reads a public id; anything else is not found.
*/
func (c *AITraceController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
