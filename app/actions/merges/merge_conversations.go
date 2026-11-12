/*
|--------------------------------------------------------------------------
| Merge Conversations
|--------------------------------------------------------------------------
|
| A customer who writes about one thing twice, by mail and then in chat,
| leaves two conversations the team would rather answer as one. Merging
| moves every entry of the sources into the target, where they keep their
| times and so read in the order they happened, and folds the sources'
| counts into it. Each source is then closed, pointed at the target and
| moved to the trash, so it leaves every list and a link to it can still
| lead on. Only conversations of the same contact, or of contacts at the
| same company, merge: anything wider would put one customer's words in
| front of another.
|
*/

package merges

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/drafts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxMergeSources is the most conversations one merge folds into a target.
*/
const MaxMergeSources = 20

/*
MergeConversationsInput is who merges which conversations into which.
*/
type MergeConversationsInput struct {
	Membership authz.Membership
	Target     conversations.Ref
	Sources    []ids.UUID
	MutationID string
}

/*
MergeConversations merges conversations into a target.

Rules:
  - The caller needs conversations:write and must see every conversation.
  - One to MaxMergeSources sources, none of them the target.
  - Each source belongs to the target's contact, or to a contact at the
    target contact's company.
  - Entries keep their times; the target's counts take the sources'.
  - Each source is closed, points at the target and goes to the trash.

Events: [appevents.ConversationMerged] per source on the target, and
[appevents.ConversationDeleted] per source.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMergeIntoItself], [exceptions.ErrMergeTooMany],
[exceptions.ErrMergeOtherCustomer], or a wrapped database error.
*/
type MergeConversations struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle merges the sources and returns the target as it now is.
*/
func (a *MergeConversations) Handle(ctx context.Context, in MergeConversationsInput) (conversations.Loaded, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return conversations.Loaded{}, err
	}

	sources := uniqueIDs(in.Sources)
	if len(sources) == 0 || len(sources) > MaxMergeSources {
		return conversations.Loaded{}, exceptions.ErrMergeTooMany
	}

	var out conversations.Loaded

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := conversations.NewWriter(ctx, tx, conversations.Services{Events: a.Events, Clock: a.Clock},
			in.Membership)
		if err != nil {
			return err
		}

		w.MutationID = in.MutationID

		in.Sources = sources

		target, locked, err := lockAll(ctx, w.Q, in)
		if err != nil {
			return err
		}

		if err := sameCustomer(ctx, w.Q, target, locked); err != nil {
			return err
		}

		for _, source := range locked {
			if target, err = fold(ctx, w, pair{target: target, source: source}); err != nil {
				return err
			}
		}

		out, err = conversations.LoadOne(ctx, w.Q, in.Membership.Member.ID, target)

		return err
	})

	return out, err
}

/*
uniqueIDs returns the ids once each, in the order first named.
*/
func uniqueIDs(in []ids.UUID) []ids.UUID {
	out := make([]ids.UUID, 0, len(in))

	for _, id := range in {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}

	return out
}

/*
lockAll finds the target and the sources the caller may see, then locks
them in id order, so two merges over the same rows wait for each other
instead of deadlocking. It returns the target and the sources, oldest
first.
*/
func lockAll(
	ctx context.Context, q *models.Queries, in MergeConversationsInput,
) (models.Conversation, []models.Conversation, error) {
	m, sources := in.Membership, in.Sources

	found, err := conversations.Find(ctx, q, m, in.Target)
	if err != nil {
		return found, nil, err
	}

	if slices.Contains(sources, found.ID) {
		return found, nil, exceptions.ErrMergeIntoItself
	}

	order := append([]ids.UUID{found.ID}, sources...)
	slices.SortFunc(order, func(x, y ids.UUID) int { return bytes.Compare(x[:], y[:]) })

	var (
		locked  []models.Conversation
		lockedT models.Conversation
	)

	for _, id := range order {
		c, err := conversations.Lock(ctx, q, m, conversations.Ref{ID: id})
		if err != nil {
			return found, nil, err
		}

		if id == found.ID {
			lockedT = c
		} else {
			locked = append(locked, c)
		}
	}

	slices.SortFunc(locked, func(x, y models.Conversation) int { return x.CreatedAt.Compare(y.CreatedAt) })

	return lockedT, locked, nil
}

/*
pair is the target of a merge and one of its sources.
*/
type pair struct {
	target models.Conversation
	source models.Conversation
}

/*
sameCustomer refuses a source of a contact that is neither the target's
nor at the target contact's company.
*/
func sameCustomer(
	ctx context.Context, q *models.Queries, target models.Conversation, sources []models.Conversation,
) error {
	var company *ids.UUID

	for _, s := range sources {
		if s.ContactID == target.ContactID {
			continue
		}

		if company == nil {
			c, err := targetCompany(ctx, q, target)
			if err != nil {
				return err
			}

			company = c
		}

		if err := atCompany(ctx, q, s, *company); err != nil {
			return err
		}
	}

	return nil
}

/*
targetCompany returns the company of the target's contact, refusing a
target whose contact has none.
*/
func targetCompany(ctx context.Context, q *models.Queries, target models.Conversation) (*ids.UUID, error) {
	c, err := companyOf(ctx, q, target)
	if err != nil {
		return nil, err
	}

	if c == nil {
		return nil, exceptions.ErrMergeOtherCustomer
	}

	return c, nil
}

/*
atCompany refuses a source whose contact is not at the company.
*/
func atCompany(ctx context.Context, q *models.Queries, source models.Conversation, company ids.UUID) error {
	other, err := companyOf(ctx, q, source)
	if err != nil {
		return err
	}

	if other == nil || *other != company {
		return exceptions.ErrMergeOtherCustomer
	}

	return nil
}

/*
companyOf returns the company of the conversation's contact, nil for
none.
*/
func companyOf(ctx context.Context, q *models.Queries, c models.Conversation) (*ids.UUID, error) {
	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: c.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return nil, fmt.Errorf("find the conversation's contact: %w", err)
	}

	return contact.CompanyID, nil
}

/*
fold moves one source into the target: its entries and what hangs on
them, then its counts. The source is retired and the merge recorded on
the target's timeline.
*/
func fold(ctx context.Context, w *conversations.Writer, p pair) (models.Conversation, error) {
	moved, err := moveEntries(ctx, w, p)
	if err != nil {
		return p.target, err
	}

	ws := p.target.WorkspaceID

	p.target, err = w.Q.FoldConversationActivity(ctx, models.FoldConversationActivityParams{
		WorkspaceID: ws, SourceID: p.source.ID, TargetID: p.target.ID, Now: w.Now,
	})
	if err != nil {
		return p.target, fmt.Errorf("fold activity: %w", err)
	}

	p.source, err = w.Q.RetireMergedConversation(ctx, models.RetireMergedConversationParams{
		WorkspaceID: ws, SourceID: p.source.ID, TargetID: p.target.ID, Now: w.Now,
	})
	if err != nil {
		return p.target, fmt.Errorf("retire merged conversation: %w", err)
	}

	return p.target, announce(ctx, w, p, int(moved))
}

/*
moveEntries moves the source's entries to the target, with their
mentions and mail ids, its people and its drafts, and returns how many
entries moved.
*/
func moveEntries(ctx context.Context, w *conversations.Writer, p pair) (int64, error) {
	ws, q, from, to := p.target.WorkspaceID, w.Q, p.source.ID, p.target.ID

	moved, err := q.MoveConversationMessages(ctx, models.MoveConversationMessagesParams{
		WorkspaceID: ws, SourceID: from, TargetID: to,
	})
	if err != nil {
		return 0, fmt.Errorf("move messages: %w", err)
	}

	steps := []func() error{
		func() error {
			return q.MoveConversationMentions(ctx, models.MoveConversationMentionsParams{
				WorkspaceID: ws, SourceID: from, TargetID: to,
			})
		},
		func() error {
			return q.MoveConversationMessageRefs(ctx, models.MoveConversationMessageRefsParams{
				WorkspaceID: ws, SourceID: from, TargetID: to,
			})
		},
		func() error {
			return q.CopyConversationParticipants(ctx, models.CopyConversationParticipantsParams{
				WorkspaceID: ws, SourceID: from, TargetID: to, Now: w.Now,
				SourceContactID: p.source.ContactID, TargetContactID: p.target.ContactID,
			})
		},
		func() error {
			return drafts.MoveOnMerge(ctx, q, drafts.Merge{Workspace: ws, Source: from, Target: to, Now: w.Now})
		},
	}

	for _, step := range steps {
		if err := step(); err != nil {
			return 0, fmt.Errorf("merge conversation: %w", err)
		}
	}

	return moved, nil
}

/*
announce records the merge on the target's timeline and sends its
events: the merge on the target, and the source leaving the lists.
*/
func announce(ctx context.Context, w *conversations.Writer, p pair, moved int) error {
	target, source := p.target, p.source

	err := w.Record(ctx, target, "conversation.merged", map[string]any{
		"source": map[string]any{
			"id": models.PublicID(models.PrefixConversation, source.ID), "number": source.Number,
		},
		"moved_messages": moved,
	})
	if err != nil {
		return err
	}

	err = w.Dispatch(ctx, appevents.ConversationMerged{
		Base: w.Base(target.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: target.ID},
		SourceID: source.ID, MovedMessages: moved,
	})
	if err != nil {
		return err
	}

	return w.Dispatch(ctx, appevents.ConversationDeleted{
		Base: w.Base(source.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: source.ID},
	})
}
