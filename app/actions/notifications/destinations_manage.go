/*
|--------------------------------------------------------------------------
| Managing Destinations
|--------------------------------------------------------------------------
|
| Admins pick the Slack channels the team hears in; any teammate links
| their own Slack direct message. Both go through the workspace's Slack
| installation, which must work when the destination is made: a channel
| is checked with Slack and named after it, and a teammate is found in
| Slack by the address they verified here, never by an id they typed, so
| nobody is ever messaged by mistake. A teammate keeps one direct
| message; linking again replaces it.
|
*/

package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ManagePermission is what changing the team's own destinations takes.
*/
const ManagePermission = "notification_destinations:manage"

/*
maxWorkspaceDestinations is how many Slack channels a workspace may
post to.
*/
const maxWorkspaceDestinations = 25

/*
The refusals of destination changes.
*/
var (
	ErrDestinationLimit = fault.New(fault.Conflict, "destination_limit_reached",
		"A workspace can post to at most 25 Slack channels.")
	ErrSlackUserNotFound = fault.New(fault.Invalid, "slack_user_not_found",
		"Slack has nobody with your verified email address in this Slack workspace.")
	ErrSlackChannelNotFound = fault.New(fault.Invalid, "slack_channel_not_found",
		"Slack has no channel with that id the app can see.").OnField("channel_id")
	ErrEmailUnverified = fault.New(fault.Invalid, "email_unverified",
		"Verify your email address before linking Slack.")
	ErrInvalidTrigger = fault.New(fault.Invalid, "invalid_trigger",
		"A Slack channel can hear about new conversations, replies, assignments, handoffs and snoozes ending.").
		OnField("triggers")
	ErrInvalidDigest = fault.New(fault.Invalid, "invalid_digest",
		"Digests are daily_standup, daily_summary and weekly_themes.").OnField("digests")
	ErrInvalidDigestTime = fault.New(fault.Invalid, "invalid_digest_time",
		"Give the digest time as HH:MM.").OnField("digest_time")
	ErrInvalidDestination = fault.New(fault.Invalid, "invalid_destination",
		"Pick a slack_channel for the workspace or a slack_dm for yourself.").OnField("type")
)

/*
ManageDestinations lists, creates, changes, removes and tests Slack
destinations.

Rules:
  - Everyone who reads notifications sees the workspace's destinations
    and their own.
  - The workspace's need notification_destinations:manage; a teammate
    changes only their own direct message.
  - The installation must work, and the channel or the teammate must be
    found in Slack.
*/
type ManageDestinations struct {
	DB      *database.DB
	Clock   clock.Clock
	Tokens  SlackTokens
	API     func(token string) *slackdriver.API
	Channel *SlackChannel
}

/*
DestinationInput is a new destination: its type, the installation it
posts through and, for a channel, the Slack channel and what it hears.
*/
type DestinationInput struct {
	Type           string
	InstallationID ids.UUID
	ChannelID      string
	Hearing        Hearing
}

/*
Hearing is what a channel hears about: its name, the triggers, the
inboxes, nil for all, whether it hears about conversations the agent is
answering and shows customers' words, and its digests and their time.
*/
type Hearing struct {
	Name             string
	Triggers         []string
	InboxIDs         []ids.UUID
	IncludeAIHandled bool
	ShowPreviews     bool
	Digests          []string
	DigestTime       string
	Enabled          bool
}

/*
DigestKinds are the digests a channel can get.
*/
func DigestKinds() []string {
	return []string{DigestStandup, DigestSummary, DigestThemes}
}

/*
List returns the workspace's destinations and the member's own.
*/
func (a *ManageDestinations) List(ctx context.Context, m authz.Membership) ([]models.NotificationDestination, error) {
	if err := requireRead(m); err != nil {
		return nil, err
	}

	var out []models.NotificationDestination

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListNotificationDestinations(ctx, models.ListNotificationDestinationsParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID,
		})

		return err
	})

	return out, err
}

/*
Show returns one destination the member may see.
*/
func (a *ManageDestinations) Show(ctx context.Context, m authz.Membership, id ids.UUID,
) (models.NotificationDestination, error) {
	if err := requireRead(m); err != nil {
		return models.NotificationDestination{}, err
	}

	var row models.NotificationDestination

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = visible(ctx, models.New(tx), m, id)

		return err
	})

	return row, err
}

/*
Create makes a destination.
*/
func (a *ManageDestinations) Create(ctx context.Context, m authz.Membership, in DestinationInput,
) (models.NotificationDestination, error) {
	if err := requireWrite(m); err != nil {
		return models.NotificationDestination{}, err
	}

	switch in.Type {
	case DestinationSlackChannel:
		return a.createChannel(ctx, m, in)
	case DestinationSlackDM:
		return a.createDirect(ctx, m, in.InstallationID)
	}

	return models.NotificationDestination{}, ErrInvalidDestination
}

/*
createChannel makes a Slack channel destination of the workspace.
*/
func (a *ManageDestinations) createChannel(ctx context.Context, m authz.Membership, in DestinationInput,
) (models.NotificationDestination, error) {
	if !m.Can(ManagePermission) {
		return models.NotificationDestination{}, exceptions.ErrForbidden
	}

	if err := checkHearing(in.Hearing); err != nil {
		return models.NotificationDestination{}, err
	}

	channel, err := a.slackChannel(ctx, m.Workspace.ID, in)
	if err != nil {
		return models.NotificationDestination{}, err
	}

	config, err := json.Marshal(DestinationConfig{ChannelID: channel.ID, ChannelName: channel.Name})
	if err != nil {
		return models.NotificationDestination{}, fmt.Errorf("encode destination config: %w", err)
	}

	h := in.Hearing
	if h.Name == "" {
		h.Name = "#" + channel.Name
	}

	return a.insert(ctx, m, models.CreateNotificationDestinationParams{
		Scope: ScopeWorkspace, Type: DestinationSlackChannel, IntegrationInstallationID: in.InstallationID,
		Name: h.Name, Config: config, Triggers: orEmpty(h.Triggers), InboxIds: h.InboxIDs,
		IncludeAiHandled: h.IncludeAIHandled, ShowPreviews: h.ShowPreviews, Digests: orEmpty(h.Digests),
		DigestTime: h.DigestTime, NextDigestAt: nextDigest(a.Clock.Now(), m.Workspace.Timezone, h),
	})
}

/*
slackChannel finds the channel in Slack through the installation.
*/
func (a *ManageDestinations) slackChannel(ctx context.Context, ws ids.UUID, in DestinationInput,
) (slackdriver.Channel, error) {
	api, err := a.api(ctx, ws, in.InstallationID)
	if err != nil {
		return slackdriver.Channel{}, err
	}

	channel, err := api.Channel(ctx, in.ChannelID)
	if slackdriver.Code(err) == slackdriver.CodeChannelGone || (err == nil && channel.IsArchived) {
		return channel, ErrSlackChannelNotFound
	}

	if err != nil {
		return channel, fmt.Errorf("find slack channel: %w", err)
	}

	return channel, nil
}

/*
createDirect links the member's own Slack direct message, replacing the
one they had.
*/
func (a *ManageDestinations) createDirect(ctx context.Context, m authz.Membership, installation ids.UUID,
) (models.NotificationDestination, error) {
	user, err := a.slackUser(ctx, m, installation)
	if err != nil {
		return models.NotificationDestination{}, err
	}

	config, err := json.Marshal(DestinationConfig{UserID: user.ID})
	if err != nil {
		return models.NotificationDestination{}, fmt.Errorf("encode destination config: %w", err)
	}

	return a.insert(ctx, m, models.CreateNotificationDestinationParams{
		Scope: ScopeMember, WorkspaceMemberID: &m.Member.ID, Type: DestinationSlackDM,
		IntegrationInstallationID: installation, Name: "Slack direct message", Config: config,
		Triggers: []string{}, Digests: []string{}, DigestTime: "09:00", ShowPreviews: true,
	})
}

/*
slackUser finds the member in Slack by their verified address.
*/
func (a *ManageDestinations) slackUser(ctx context.Context, m authz.Membership, installation ids.UUID,
) (slackdriver.User, error) {
	var user models.User

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		user, err = models.New(tx).FindUser(ctx, m.Member.UserID)

		return err
	})
	if err != nil {
		return slackdriver.User{}, fmt.Errorf("find user: %w", err)
	}

	if user.EmailVerifiedAt == nil {
		return slackdriver.User{}, ErrEmailUnverified
	}

	api, err := a.api(ctx, m.Workspace.ID, installation)
	if err != nil {
		return slackdriver.User{}, err
	}

	found, err := api.UserByEmail(ctx, user.Email)
	if slackdriver.Unreachable(err) || (err == nil && (found.IsBot || found.Deleted)) {
		return found, ErrSlackUserNotFound
	}

	if err != nil {
		return found, fmt.Errorf("find slack user: %w", err)
	}

	return found, nil
}

/*
insert stores the destination, within the workspace's limit, replacing
the member's own of the same type.
*/
func (a *ManageDestinations) insert(ctx context.Context, m authz.Membership,
	p models.CreateNotificationDestinationParams,
) (models.NotificationDestination, error) {
	p.WorkspaceID, p.CreatedByMemberID, p.Now = m.Workspace.ID, &m.Member.ID, a.Clock.Now()

	var row models.NotificationDestination

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if err := a.makeRoom(ctx, q, m, p); err != nil {
			return err
		}

		var err error
		row, err = q.CreateNotificationDestination(ctx, p)
		if err != nil {
			return fmt.Errorf("create notification destination: %w", err)
		}

		return nil
	})

	return row, err
}

/*
makeRoom refuses a channel past the limit and removes the member's
earlier direct message.
*/
func (a *ManageDestinations) makeRoom(ctx context.Context, q *models.Queries, m authz.Membership,
	p models.CreateNotificationDestinationParams,
) error {
	if p.Scope == ScopeMember {
		existing, found, err := memberSlack(ctx, q, m.Workspace.ID, m.Member.ID)
		if err != nil || !found {
			return err
		}

		_, err = q.DeleteNotificationDestination(ctx, models.DeleteNotificationDestinationParams{
			WorkspaceID: m.Workspace.ID, ID: existing.DestinationID,
		})

		return err
	}

	n, err := q.CountWorkspaceNotificationDestinations(ctx, m.Workspace.ID)
	if err != nil {
		return fmt.Errorf("count notification destinations: %w", err)
	}

	if n >= maxWorkspaceDestinations {
		return ErrDestinationLimit
	}

	return nil
}

/*
api opens Slack through the workspace's working installation.
*/
func (a *ManageDestinations) api(ctx context.Context, ws, installation ids.UUID) (*slackdriver.API, error) {
	var row models.IntegrationInstallation

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
			WorkspaceID: ws, ID: installation,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && row.Provider != slackdriver.Type) {
		return nil, exceptions.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find installation: %w", err)
	}

	token, err := a.Tokens.Bot(ctx, installation)
	if err != nil {
		return nil, ErrSlackUnavailable
	}

	return a.API(token), nil
}

/*
ErrSlackUnavailable refuses a destination on an installation that does
not work.
*/
var ErrSlackUnavailable = fault.New(fault.Conflict, "slack_unavailable",
	"The Slack installation must be reconnected first.")

/*
checkHearing checks the triggers, digests and digest time of a channel.
*/
func checkHearing(h Hearing) error {
	for _, t := range h.Triggers {
		if !slices.Contains(DestinationTriggers(), t) {
			return ErrInvalidTrigger
		}
	}

	for _, d := range h.Digests {
		if !slices.Contains(DigestKinds(), d) {
			return ErrInvalidDigest
		}
	}

	if _, ok := minutes(h.DigestTime); !ok {
		return ErrInvalidDigestTime
	}

	return nil
}

/*
nextDigest is when the channel's next digest is due: the next time its
digest time comes round in the workspace's zone, nil without digests.
*/
func nextDigest(now time.Time, zone string, h Hearing) *time.Time {
	at, ok := minutes(h.DigestTime)
	if len(h.Digests) == 0 || !ok {
		return nil
	}

	loc, err := time.LoadLocation(zone)
	if err != nil {
		loc = time.UTC
	}

	local := now.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), at/60, at%60, 0, 0, loc)

	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}

	next = next.UTC()

	return &next
}

/*
orEmpty returns the list, an empty one for nil.
*/
func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}
