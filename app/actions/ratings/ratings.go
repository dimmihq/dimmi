/*
|--------------------------------------------------------------------------
| Conversation Ratings
|--------------------------------------------------------------------------
|
| When a conversation closes, its customer can be asked how it went: a
| card with five faces in the messenger, or an email whose five faces
| link to a page that needs no account. The workspace chooses whether
| to ask, how long after the close, whether conversations the agent
| closed ask too, and on which channels; a workflow's close step can ask
| whatever those settings say. The customer's score lands on the
| timeline, goes to webhooks and starts workflows, and stays open to a
| change or a comment for a week.
|
*/

package ratings

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EditWindow is how long after the ask a customer may still change their
rating; MaxComment is the longest comment kept.
*/
const (
	EditWindow = 7 * 24 * time.Hour
	MaxComment = 2000
)

/*
Channels a rating is asked on.
*/
const (
	ChannelWidget = "widget"
	ChannelEmail  = "email"
)

/*
Why a rating or a setting is refused.
*/
var (
	ErrRatingClosed = fault.New(fault.Gone, "rating_closed",
		"This rating can no longer be changed.")
	ErrRatingScore = fault.New(fault.Invalid, "rating_score_invalid",
		"Pick a score from 1 to 5.").OnField("score")
	ErrRatingDelay = fault.New(fault.Invalid, "rating_delay_invalid",
		"Ask straight away, after 10 minutes or after an hour.").OnField("delay_minutes")
)

/*
Ratings asks customers to rate closed conversations and keeps their
answers. Email sends the email asks from the conversation's address,
Links signs the links in them, and Queue times the asks.
*/
type Ratings struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
	Queue  *queue.Queue
	Email  *email.SendRatingEmail
	Links  Links
}

/*
Listen turns every close into a job that may ask for a rating.
*/
func (r *Ratings) Listen(d *events.Dispatcher) {
	events.ListenQueued(d, func(ev appevents.ConversationClosed) queue.Args {
		return jobs.AskConversationRatingArgs{
			WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, ClosedAt: ev.OccurredAt,
			Actor: ev.Actor.Type, Forced: ev.AskRating,
		}
	})
}

/*
Settings is how a workspace asks for ratings: whether at all, how many
minutes after the close, whether conversations the agent closed ask
too, and on which channels.
*/
type Settings struct {
	Enabled            bool
	DelayMinutes       int
	IncludeAgentClosed bool
	Messenger          bool
	Email              bool
}

/*
defaults is what a workspace that never saved its settings gets: no
asks, but both channels ready once turned on.
*/
func defaults() Settings {
	return Settings{Messenger: true, Email: true}
}

/*
SettingsOf returns the workspace's rating settings.
*/
func (r *Ratings) SettingsOf(ctx context.Context, m authz.Membership) (Settings, error) {
	if !m.Can("settings:read") {
		return Settings{}, exceptions.ErrForbidden
	}

	var out Settings
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = load(ctx, models.New(tx), m.Workspace.ID)
		return err
	})

	return out, err
}

/*
SaveSettings keeps the workspace's rating settings. The delay is 0, 10
or 60 minutes.
*/
func (r *Ratings) SaveSettings(ctx context.Context, m authz.Membership, s Settings) (Settings, error) {
	if !m.Can("settings:write") {
		return Settings{}, exceptions.ErrForbidden
	}

	if s.DelayMinutes != 0 && s.DelayMinutes != 10 && s.DelayMinutes != 60 {
		return Settings{}, ErrRatingDelay
	}

	var out Settings
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).SaveConversationRatingSettings(ctx, models.SaveConversationRatingSettingsParams{
			WorkspaceID: m.Workspace.ID, Enabled: s.Enabled, DelayMinutes: delayColumn(s.DelayMinutes),
			IncludeAgentClosed: s.IncludeAgentClosed, Messenger: s.Messenger, Email: s.Email, Now: r.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("save rating settings: %w", err)
		}
		out = fromRow(row)
		return nil
	})

	return out, err
}

/*
load reads the workspace's settings, or the defaults when it saved none.
*/
func load(ctx context.Context, q *models.Queries, workspace ids.UUID) (Settings, error) {
	row, err := q.FindConversationRatingSettings(ctx, workspace)
	if errors.Is(err, database.ErrNotFound) {
		return defaults(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("find rating settings: %w", err)
	}

	return fromRow(row), nil
}

/*
delayColumn narrows a delay in minutes to the column's range, holding it
at the largest or smallest value the column stores.
*/
func delayColumn(minutes int) int32 {
	if minutes > math.MaxInt32 {
		return math.MaxInt32
	}

	if minutes < math.MinInt32 {
		return math.MinInt32
	}

	return int32(minutes)
}

func fromRow(row models.ConversationRatingSetting) Settings {
	return Settings{
		Enabled: row.Enabled, DelayMinutes: int(row.DelayMinutes), IncludeAgentClosed: row.IncludeAgentClosed,
		Messenger: row.Messenger, Email: row.Email,
	}
}

/*
txn is the transaction a step writes in and the queries bound to it.
*/
type txn struct {
	tx *database.Tx
	q  *models.Queries
}

/*
inWorkspace runs fn in a transaction scoped to the workspace, for the
jobs and the customer's own requests, which carry no member.
*/
func (r *Ratings) inWorkspace(ctx context.Context, workspace ids.UUID,
	fn func(context.Context, *database.Tx, *models.Queries) error,
) error {
	scoped := tenancy.With(ctx, workspace.String())
	return r.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, tx, models.New(tx))
	})
}
