/*
|--------------------------------------------------------------------------
| Inbox Controller
|--------------------------------------------------------------------------
|
| GET /inboxes lists the inboxes of the workspace in play, GET
| /inboxes/{inbox} shows one, POST /inboxes adds one with its API
| channel, PATCH /inboxes/{inbox} changes it and DELETE removes it,
| moving its conversations. GET /inboxes/{inbox}/assignment-status
| explains its automatic assignment. Teams travel by public id and are
| translated here; ids of other workspaces' inboxes are not found.
|
*/

package v1

import (
	"context"
	"encoding/json"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/assignment"
	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxInboxes is the most inboxes a workspace lists, all on one page.
*/
const maxInboxes = 200

/*
InboxesOutput is the workspace's inboxes.
*/
type InboxesOutput struct {
	Body fhttp.Collection[resources.Inbox]
}

/*
InboxOutput is one inbox.
*/
type InboxOutput struct {
	Body fhttp.Resource[resources.Inbox]
}

/*
InboxController serves the inboxes of the workspace in play.
*/
type InboxController struct {
	List   *inboxes.ListInboxes
	Show   *inboxes.ShowInbox
	Create *inboxes.CreateInbox
	Change *inboxes.UpdateInbox
	Remove *inboxes.DeleteInbox
	Status *assignment.ShowAssignmentStatus
	IDs    *ids.Registry
}

/*
AssignmentStatusOutput is an inbox's automatic assignment as it stands.
*/
type AssignmentStatusOutput struct {
	Body fhttp.Resource[resources.AssignmentStatus]
}

/*
AssignmentStatus explains how the inbox assigns right now.
*/
func (c *InboxController) AssignmentStatus(
	ctx context.Context, in *requests.ShowAssignmentStatusRequest,
) (*AssignmentStatusOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInbox), in.Inbox)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	status, err := c.Status.Handle(ctx, assignment.StatusInput{
		Membership: membership(ctx), InboxID: id, ChannelType: enums.ChannelType(in.ChannelType),
	})
	if err != nil {
		return nil, err
	}

	return &AssignmentStatusOutput{Body: fhttp.NewResource(resources.NewAssignmentStatus(status))}, nil
}

/*
Index lists the inboxes, oldest first, on one page.
*/
func (c *InboxController) Index(ctx context.Context, _ *requests.NoInput) (*InboxesOutput, error) {
	rows, err := c.List.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.Inbox]{Items: resources.NewInboxes(rows), Limit: maxInboxes}

	return &InboxesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Get answers with one inbox.
*/
func (c *InboxController) Get(ctx context.Context, in *requests.ShowInboxRequest) (*InboxOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInbox), in.Inbox)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	inbox, err := c.Show.Handle(ctx, inboxes.ShowInboxInput{Membership: membership(ctx), InboxID: id})
	if err != nil {
		return nil, err
	}

	return &InboxOutput{Body: fhttp.NewResource(resources.NewInbox(inbox))}, nil
}

/*
Store adds an inbox and answers with it.
*/
func (c *InboxController) Store(ctx context.Context, in *requests.StoreInboxRequest) (*InboxOutput, error) {
	inbox, err := c.Create.Handle(ctx, inboxes.CreateInboxInput{Membership: membership(ctx), Name: in.Body.Name})
	if err != nil {
		return nil, err
	}

	return &InboxOutput{Body: fhttp.NewResource(resources.NewInbox(inbox))}, nil
}

/*
Update changes an inbox's name, default team and settings, and answers
with it as it now is.
*/
func (c *InboxController) Update(ctx context.Context, in *requests.UpdateInboxRequest) (*InboxOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInbox), in.Inbox)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	team, err := c.defaultTeam(in)
	if err != nil {
		return nil, err
	}

	settings, err := c.settings(in.Body.Settings)
	if err != nil {
		return nil, err
	}

	inbox, err := c.Change.Handle(ctx, inboxes.UpdateInboxInput{
		Membership: membership(ctx), InboxID: id, Name: in.Body.Name, DefaultTeam: team, Settings: settings,
	})
	if err != nil {
		return nil, err
	}

	return &InboxOutput{Body: fhttp.NewResource(resources.NewInbox(inbox))}, nil
}

/*
Destroy deletes an inbox, moving its conversations and channels to the
inbox move_to names.
*/
func (c *InboxController) Destroy(ctx context.Context, in *requests.DeleteInboxRequest) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInbox), in.Inbox)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	var moveTo *ids.UUID

	if in.MoveTo != "" {
		to, err := c.IDs.Decode(string(models.PrefixInbox), in.MoveTo)
		if err != nil {
			return nil, exceptions.ErrMoveToInvalid
		}

		moveTo = &to
	}

	err = c.Remove.Handle(ctx, inboxes.DeleteInboxInput{Membership: membership(ctx), InboxID: id, MoveTo: moveTo})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
defaultTeam reads the default team the update asks for: nil to keep
it, a choice without an id when it is sent as null.
*/
func (c *InboxController) defaultTeam(in *requests.UpdateInboxRequest) (*inboxes.TeamChoice, error) {
	if requests.Nulls(in.RawBody)["default_team_id"] {
		return &inboxes.TeamChoice{}, nil
	}

	if in.Body.DefaultTeamID == nil {
		return nil, nil
	}

	team, err := c.IDs.Decode(string(models.PrefixTeam), *in.Body.DefaultTeamID)
	if err != nil {
		return nil, exceptions.ErrInboxTeamNotFound.OnField("default_team_id")
	}

	return &inboxes.TeamChoice{ID: &team}, nil
}

/*
settings returns the settings patch with the overflow team's public id
turned into the internal one the inbox stores. A patch without one goes
on as it is, for the action to check.
*/
func (c *InboxController) settings(raw json.RawMessage) (json.RawMessage, error) {
	p, ok := readOverflow(raw)
	if !ok {
		return raw, nil
	}

	team, err := c.IDs.Decode(string(models.PrefixTeam), p.overflow)
	if err != nil {
		return nil, exceptions.ErrInboxTeamNotFound.OnField("settings.assignment.overflow_team_id")
	}

	p.rule["overflow_team_id"], _ = json.Marshal(team)
	p.top["assignment"], _ = json.Marshal(p.rule)

	return json.Marshal(p.top)
}

/*
settingsPatch is a settings patch taken apart down to the overflow team
it names.
*/
type settingsPatch struct {
	top      map[string]json.RawMessage
	rule     map[string]json.RawMessage
	overflow string
}

/*
readOverflow takes the patch apart, and reports whether it names an
overflow team by id.
*/
func readOverflow(raw json.RawMessage) (settingsPatch, bool) {
	var p settingsPatch

	if json.Unmarshal(raw, &p.top) != nil || p.top == nil {
		return p, false
	}

	if json.Unmarshal(p.top["assignment"], &p.rule) != nil || p.rule == nil {
		return p, false
	}

	return p, json.Unmarshal(p.rule["overflow_team_id"], &p.overflow) == nil && p.overflow != ""
}
