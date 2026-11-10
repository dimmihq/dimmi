/*
|--------------------------------------------------------------------------
| Teammate Access From Slack
|--------------------------------------------------------------------------
|
| A button in Slack is the app by another door, so it allows exactly
| what the app allows: the teammate's role, the custom role narrowing
| it, their access teams and hidden inboxes. A teammate who may not
| change a conversation, or may not see it, is told so and nothing
| changes; a member no longer active acts on nothing at all.
|
*/

package channels

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/slackteam"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Notices for a teammate whose press dimmi refused.
*/
const (
	inactiveNotice  = "Your dimmi account is not active in this workspace, so nothing was changed."
	forbiddenNotice = "Your dimmi role does not let you change this conversation, so nothing was changed."
)

/*
teammateFor returns the linked teammate the Slack user acts as on the
conversation, with their membership, reporting false and telling them
why when they are not linked, no longer active, or may not change or
see the conversation.
*/
func (s *Slack) teammateFor(ctx context.Context, in SlackInteraction, conv models.Conversation,
) (teammate, bool, error) {
	who, linked, err := s.teammateOf(ctx, in)
	if err != nil || !linked {
		return who, false, err
	}

	var (
		m      authz.Membership
		active bool
	)

	err = s.read(ctx, func(q *models.Queries) error {
		var err error
		if m, active, err = slackteam.Membership(ctx, q, conv.WorkspaceID, who.memberID); err != nil || !active {
			return err
		}

		_, err = conversations.Find(ctx, q, m, conversations.Ref{ID: conv.ID})

		return err
	})

	switch {
	case errors.Is(err, exceptions.ErrNotFound) || (err == nil && active && !m.Can("conversations:write")):
		s.tell(ctx, in, forbiddenNotice)

		return who, false, nil
	case err != nil:
		return who, false, err
	case !active:
		s.tell(ctx, in, inactiveNotice)

		return who, false, nil
	}

	who.membership = m

	return who, true, nil
}
