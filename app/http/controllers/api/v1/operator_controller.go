/*
|--------------------------------------------------------------------------
| Operator Controller
|--------------------------------------------------------------------------
|
| Teammates talk with Operator here: they read and change its settings,
| open and read their threads, ask it questions, have it brief them on a
| conversation, decide the changes it proposes and manage what it
| remembers. The actions decide who may do what; this maps ids and
| shapes.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorSettingsOutput is the workspace's Operator settings.
*/
type OperatorSettingsOutput struct {
	Body fhttp.Resource[resources.OperatorSettings]
}

/*
OperatorThreadsOutput is the caller's threads.
*/
type OperatorThreadsOutput struct {
	Body fhttp.Collection[resources.OperatorThread]
}

/*
OperatorThreadOutput is one thread with its messages and changesets.
*/
type OperatorThreadOutput struct {
	Body fhttp.Resource[resources.OperatorThreadView]
}

/*
OperatorAnswerOutput is one answered question.
*/
type OperatorAnswerOutput struct {
	Body fhttp.Resource[resources.OperatorAnswer]
}

/*
OperatorController serves Operator's settings, threads and questions.
*/
type OperatorController struct {
	Settings *operatorchat.Settings
	Threads  *operatorchat.Threads
	IDs      *ids.Registry
}

/*
ShowSettings returns Operator's settings.
*/
func (c *OperatorController) ShowSettings(ctx context.Context, _ *requests.NoInput) (*OperatorSettingsOutput, error) {
	s, err := c.Settings.Show(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	return &OperatorSettingsOutput{Body: fhttp.NewResource(resources.NewOperatorSettings(s))}, nil
}

/*
UpdateSettings merges a change into Operator's settings.
*/
func (c *OperatorController) UpdateSettings(ctx context.Context, in *requests.UpdateOperatorSettingsRequest,
) (*OperatorSettingsOutput, error) {
	s, err := c.Settings.Update(ctx, operatorchat.SettingsInput{
		Membership: membership(ctx), Enabled: in.Body.Enabled, BriefOnOpen: in.Body.BriefOnOpen,
		TriageOnArrival: in.Body.TriageOnArrival, Instructions: in.Body.Instructions, Policies: in.Body.Policies,
		Model: in.Body.Model,
	})
	if err != nil {
		return nil, err
	}
	return &OperatorSettingsOutput{Body: fhttp.NewResource(resources.NewOperatorSettings(s))}, nil
}

/*
Index lists the caller's threads, only those about the whole workspace
when filter[scope] is workspace, or about one conversation when
filter[conversation] names it.
*/
func (c *OperatorController) Index(ctx context.Context, _ *requests.NoInput) (*OperatorThreadsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)
	query := r.URL.Query()
	filter := operatorchat.ListFilter{WorkspaceOnly: query.Get("filter[scope]") == "workspace"}
	if named := query.Get("filter[conversation]"); named != "" {
		id, err := c.decode(models.PrefixConversation, named)
		if err != nil {
			return nil, err
		}
		filter.Conversation = &id
	}
	rows, err := c.Threads.List(ctx, membership(ctx), filter)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.OperatorThread]{Items: resources.NewOperatorThreads(rows), Limit: 100}
	return &OperatorThreadsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store opens the caller's thread about a conversation, or a new thread
about the workspace.
*/
func (c *OperatorController) Store(ctx context.Context, in *requests.StoreOperatorThreadRequest,
) (*OperatorThreadOutput, error) {
	conversation, err := decodeField(c.IDs, models.PrefixConversation, in.Body.ConversationID, exceptions.ErrNotFound)
	if err != nil {
		return nil, err
	}
	t, err := c.Threads.Open(ctx, membership(ctx), conversation, in.Body.Fresh)
	if err != nil {
		return nil, err
	}
	return threadOutput(t), nil
}

/*
Show returns one of the caller's threads.
*/
func (c *OperatorController) Show(ctx context.Context, in *requests.OperatorThreadRequest,
) (*OperatorThreadOutput, error) {
	id, err := c.decode(models.PrefixOperatorThread, in.Thread)
	if err != nil {
		return nil, err
	}
	t, err := c.Threads.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return threadOutput(t), nil
}

/*
Destroy deletes one of the caller's threads.
*/
func (c *OperatorController) Destroy(ctx context.Context, in *requests.OperatorThreadRequest) (*struct{}, error) {
	id, err := c.decode(models.PrefixOperatorThread, in.Thread)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Threads.Delete(ctx, membership(ctx), id)
}

/*
Ask asks Operator a question on a thread and answers once it is done;
the text streams to the caller's channel meanwhile.
*/
func (c *OperatorController) Ask(ctx context.Context, in *requests.AskOperatorRequest) (*OperatorAnswerOutput, error) {
	id, err := c.decode(models.PrefixOperatorThread, in.Thread)
	if err != nil {
		return nil, err
	}
	answer, err := c.Threads.Ask(ctx, operatorchat.AskInput{
		Membership: membership(ctx), ThreadID: id, Body: in.Body.Body,
	})
	if err != nil {
		return nil, err
	}
	return &OperatorAnswerOutput{Body: fhttp.NewResource(resources.NewOperatorAnswer(answer))}, nil
}

/*
Brief has Operator brief the caller on a conversation, on their thread
about it.
*/
func (c *OperatorController) Brief(ctx context.Context, in *requests.BriefConversationRequest,
) (*OperatorThreadOutput, error) {
	id, err := c.decode(models.PrefixConversation, in.Conversation)
	if err != nil {
		return nil, err
	}
	thread, err := decodeField(c.IDs, models.PrefixOperatorThread, in.Body.ThreadID, exceptions.ErrNotFound)
	if err != nil {
		return nil, err
	}
	t, err := c.Threads.Brief(ctx, operatorchat.BriefInput{
		Membership: membership(ctx), Conversation: id, Thread: thread, Attach: in.Body.Attach,
	})
	if err != nil {
		return nil, err
	}
	return threadOutput(t), nil
}

/*
decode reads a public id; anything else is not found.
*/
func (c *OperatorController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}

/*
threadOutput shapes a thread with what it holds.
*/
func threadOutput(t operatorchat.Thread) *OperatorThreadOutput {
	view := resources.NewOperatorThreadView(t.Thread, t.Messages, t.Changesets)
	return &OperatorThreadOutput{Body: fhttp.NewResource(view)}
}
