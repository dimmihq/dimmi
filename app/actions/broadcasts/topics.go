/*
|--------------------------------------------------------------------------
| Topics
|--------------------------------------------------------------------------
|
| Topics are the lists people subscribe to, such as product updates or
| webinars, so leaving one never means leaving all. An opt-in topic
| reaches only those who said yes; an opt-out topic reaches everyone who
| has not said no. A topic's consent never changes once people chose
| under it; archiving keeps every choice made.
|
*/

package broadcasts

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Errors topics return.
*/
var (
	ErrTopicName = fault.New(fault.Invalid, "broadcast_topic_name_invalid",
		"Give the topic a name of up to 80 characters, different from your other topics.")
	ErrTopicConsent = fault.New(fault.Invalid, "broadcast_topic_consent_invalid",
		"Choose whether people opt in to this topic or are in until they opt out.")
)

/*
TopicInput is a topic's editable fields.
*/
type TopicInput struct {
	Membership  authz.Membership
	ID          ids.UUID
	Name        string
	Description string
	Consent     string
	Archive     bool
}

/*
Topics lists the workspace's topics.
*/
func (b *Broadcasts) Topics(ctx context.Context, m authz.Membership) ([]models.BroadcastTopic, error) {
	if !m.Can("broadcasts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.BroadcastTopic

	err := b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error

		out, err = q.ListBroadcastTopics(ctx, m.Workspace.ID)

		return wrap("list topics", err)
	})

	return out, err
}

/*
CreateTopic adds a topic.
*/
func (b *Broadcasts) CreateTopic(ctx context.Context, in TopicInput) (models.BroadcastTopic, error) {
	if !in.Membership.Can("broadcasts:write") {
		return models.BroadcastTopic{}, exceptions.ErrForbidden
	}

	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 80 {
		return models.BroadcastTopic{}, ErrTopicName
	}

	if in.Consent != "opt_in" && in.Consent != "opt_out" {
		return models.BroadcastTopic{}, ErrTopicConsent
	}

	var out models.BroadcastTopic

	err := b.inWorkspace(ctx, in.Membership.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error

		out, err = q.CreateBroadcastTopic(ctx, models.CreateBroadcastTopicParams{
			WorkspaceID: in.Membership.Workspace.ID, Name: name, Description: strings.TrimSpace(in.Description),
			Consent: in.Consent, Now: b.Clock.Now(),
		})

		return topicError(err)
	})

	return out, err
}

/*
UpdateTopic renames, describes or archives a topic.
*/
func (b *Broadcasts) UpdateTopic(ctx context.Context, in TopicInput) (models.BroadcastTopic, error) {
	if !in.Membership.Can("broadcasts:write") {
		return models.BroadcastTopic{}, exceptions.ErrForbidden
	}

	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 80 {
		return models.BroadcastTopic{}, ErrTopicName
	}

	var out models.BroadcastTopic

	err := b.inWorkspace(ctx, in.Membership.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		params := models.UpdateBroadcastTopicParams{
			WorkspaceID: in.Membership.Workspace.ID, ID: in.ID, Name: name,
			Description: strings.TrimSpace(in.Description), Now: b.Clock.Now(),
		}
		if in.Archive {
			now := b.Clock.Now()
			params.ArchivedAt = &now
		}

		var err error

		out, err = q.UpdateBroadcastTopic(ctx, params)

		return topicError(notFound(err, "update topic"))
	})

	return out, err
}

/*
topicError turns a name another topic holds into the name error.
*/
func topicError(err error) error {
	if database.IsUniqueViolation(err, "broadcast_topics_workspace_id_name_key") {
		return ErrTopicName
	}

	return wrap("save topic", err)
}
