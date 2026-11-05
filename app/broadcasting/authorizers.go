/*
|--------------------------------------------------------------------------
| Channel Authorizers
|--------------------------------------------------------------------------
|
| Who may listen where is decided the way the API decides who may read:
| the caller must be an active member of the workspace, through a
| credential that holds the permission, and a conversation must be one
| the member may see, by the same visibility rule the conversation lists
| use. A refusal never says why, so a channel name cannot be used to
| probe whether a conversation or member exists elsewhere.
|
| The messenger widget never asks: the server subscribes a visitor to
| their own contact's channel when they connect.
|
*/

package broadcasting

import (
	"context"
	"errors"
	"fmt"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
readConversations is the permission every inbox channel requires.
*/
const readConversations = "conversations:read"

/*
WidgetSession is the subject of a messenger widget connection: the
contact a visitor is, in their workspace.
*/
type WidgetSession struct {
	WorkspaceID ids.UUID
	ContactID   ids.UUID
}

/*
Authorizers guard the product's channels. Resolve decides a caller's
membership of a workspace exactly as for API requests.
*/
type Authorizers struct {
	DB      *database.DB
	IDs     *ids.Registry
	Resolve *workspaces.ResolveWorkspace
}

/*
Workspace admits active members of the workspace who may read its
conversations.
*/
func (a *Authorizers) Workspace(
	ctx context.Context, p fbroadcasting.Principal, params map[string]string,
) (bool, error) {
	caller, ok := p.Subject.(*appauth.Principal)
	if !ok {
		return false, nil
	}

	m, ok, err := a.membership(ctx, caller, params["workspace"])
	if err != nil || !ok {
		return false, err
	}

	return m.Can(readConversations), nil
}

/*
WorkspaceConversations admits active members who may read conversations
and see every one of them. Everyone else is refused and hears about
their conversations on their own channel.
*/
func (a *Authorizers) WorkspaceConversations(
	ctx context.Context, p fbroadcasting.Principal, params map[string]string,
) (bool, error) {
	caller, ok := p.Subject.(*appauth.Principal)
	if !ok {
		return false, nil
	}

	m, ok, err := a.membership(ctx, caller, params["workspace"])
	if err != nil || !ok {
		return false, err
	}

	return m.Can(readConversations) && !m.ConversationScope().Restricted(), nil
}

/*
Conversation admits members who may read conversations and may see
this one.
*/
func (a *Authorizers) Conversation(
	ctx context.Context, p fbroadcasting.Principal, params map[string]string,
) (bool, error) {
	caller, ok := p.Subject.(*appauth.Principal)
	if !ok {
		return false, nil
	}

	id, ok := a.decode(models.PrefixConversation, params["conversation"])
	if !ok {
		return false, nil
	}

	conversation, found, err := a.findConversation(ctx, caller, id)
	if err != nil || !found {
		return false, err
	}

	m, ok, err := a.membership(ctx, caller, models.PublicID(models.PrefixWorkspace, conversation.WorkspaceID))
	if err != nil || !ok {
		return false, err
	}

	seen := conversation.DeletedAt == nil && m.ConversationScope().Contains(conversation)

	return m.Can(readConversations) && seen, nil
}

/*
Member admits only the member themselves.
*/
func (a *Authorizers) Member(ctx context.Context, p fbroadcasting.Principal, params map[string]string) (bool, error) {
	caller, ok := p.Subject.(*appauth.Principal)
	if !ok {
		return false, nil
	}

	id, ok := a.decode(models.PrefixMember, params["member"])
	if !ok {
		return false, nil
	}

	if !caller.IsFirstParty() {
		return caller.MemberID == id, nil
	}

	rows, err := models.New(a.DB).ListUserWorkspaces(ctx, caller.User.ID)
	if err != nil {
		return false, fmt.Errorf("list workspaces of realtime user: %w", err)
	}

	for _, row := range rows {
		if row.MemberID == id {
			return true, nil
		}
	}

	return false, nil
}

/*
WidgetContact admits only the widget connection of that contact.
*/
func (a *Authorizers) WidgetContact(
	_ context.Context, p fbroadcasting.Principal, params map[string]string,
) (bool, error) {
	s, ok := p.Subject.(*WidgetSession)

	return ok && models.PublicID(models.PrefixContact, s.ContactID) == params["contact"], nil
}

/*
membership returns the caller's membership of the named workspace, and
false when the caller may not work there.
*/
func (a *Authorizers) membership(
	ctx context.Context, caller *appauth.Principal, named string,
) (authz.Membership, bool, error) {
	m, err := a.Resolve.Handle(ctx, workspaces.ResolveWorkspaceInput{Principal: caller, Named: named})
	if _, refused := errors.AsType[*fault.Error](err); refused {
		return authz.Membership{}, false, nil
	}

	if err != nil {
		return authz.Membership{}, false, fmt.Errorf("resolve realtime workspace: %w", err)
	}

	return m, true, nil
}

/*
decode returns the row a public id names, and false for text that is
no id of the kind.
*/
func (a *Authorizers) decode(prefix models.Prefix, public string) (ids.UUID, bool) {
	id, err := a.IDs.Decode(string(prefix), public)

	return id, err == nil
}

/*
findConversation looks the conversation up in the workspaces the caller
reaches: a token's own, or every workspace the user is active in.
*/
func (a *Authorizers) findConversation(
	ctx context.Context, caller *appauth.Principal, id ids.UUID,
) (models.Conversation, bool, error) {
	candidates := []ids.UUID{caller.WorkspaceID}

	if caller.IsFirstParty() {
		rows, err := models.New(a.DB).ListUserWorkspaces(ctx, caller.User.ID)
		if err != nil {
			return models.Conversation{}, false, fmt.Errorf("list workspaces of realtime user: %w", err)
		}

		candidates = candidates[:0]
		for _, row := range rows {
			candidates = append(candidates, row.WorkspaceID)
		}
	}

	for _, workspaceID := range candidates {
		conversation, err := a.conversationIn(ctx, workspaceID, id)
		if errors.Is(err, database.ErrNotFound) {
			continue
		}

		return conversation, err == nil, err
	}

	return models.Conversation{}, false, nil
}

/*
conversationIn reads the conversation inside the workspace.
*/
func (a *Authorizers) conversationIn(ctx context.Context, workspaceID, id ids.UUID) (models.Conversation, error) {
	var conversation models.Conversation

	err := a.DB.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		conversation, err = models.New(tx).FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: workspaceID, ID: id,
		})

		return err
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return conversation, fmt.Errorf("find realtime conversation: %w", err)
	}

	return conversation, err
}

/*
Locate returns the workspace of a conversation the caller reaches, and
false when they reach none with that id.
*/
func (a *Authorizers) Locate(ctx context.Context, caller *appauth.Principal, id ids.UUID) (ids.UUID, bool, error) {
	conversation, found, err := a.findConversation(ctx, caller, id)

	return conversation.WorkspaceID, found, err
}
