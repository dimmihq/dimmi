/*
|--------------------------------------------------------------------------
| Saving A Series
|--------------------------------------------------------------------------
|
| The builder saves the whole working copy and the sending settings at
| once, with the lock version it read. A save over someone else's newer
| save is refused, so two teammates never silently undo each other.
| Saving never changes what runs: only publishing does.
|
*/

package series

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
attributeKey is the shape of a custom attribute key a step writes.
*/
var attributeKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

/*
SaveInput is a save of one series over LockVersion: its name and
description, the working copy, and who and how its messages are sent.
*/
type SaveInput struct {
	Membership     authz.Membership
	ID             ids.UUID
	LockVersion    int32
	Name           string
	Description    string
	Spec           *Spec
	TopicID        *ids.UUID
	Service        bool
	EmailAddressID *ids.UUID
	SenderMemberID *ids.UUID
	SlackAs        string
	ReplyTeamID    *ids.UUID
	ReplyMemberID  *ids.UUID
	AIReplies      bool
}

/*
CurrentSettings reads a series' current settings as a save, so a caller can
change some of them and save the rest as they are.
*/
func CurrentSettings(row models.Series) SaveInput {
	return SaveInput{
		ID: row.ID, LockVersion: row.LockVersion, Name: row.Name, Description: row.Description,
		TopicID: row.TopicID, Service: row.Service, EmailAddressID: row.EmailAddressID,
		SenderMemberID: row.SenderMemberID, SlackAs: row.SlackAs, ReplyTeamID: row.ReplyTeamID,
		ReplyMemberID: row.ReplyMemberID, AIReplies: row.AiReplies,
	}
}

/*
Save stores the working copy and settings.
*/
func (s *Series) Save(ctx context.Context, in SaveInput) (models.Series, error) {
	m := in.Membership
	if !m.Can("series:write") {
		return models.Series{}, exceptions.ErrForbidden
	}

	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 120 {
		return models.Series{}, ErrName
	}

	if in.SlackAs != "member" {
		in.SlackAs = "bot"
	}

	var out models.Series

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		row, err := find(ctx, q, m.Workspace.ID, in.ID)
		if err != nil {
			return err
		}

		if row.Status == StatusArchived {
			return ErrArchived
		}

		working, hash := row.Working, row.WorkingHash
		if in.Spec != nil {
			if working, hash, err = encode(*in.Spec); err != nil {
				return err
			}
		}

		out, err = q.SaveSeries(ctx, saveParams(in, working, hash, s.Clock.Now()))
		if errors.Is(err, database.ErrNotFound) {
			return ErrConflict
		}

		return nameTaken(err, "save series")
	})

	return out, err
}

/*
saveParams lays a save out for the query.
*/
func saveParams(in SaveInput, working []byte, hash string, now time.Time) models.SaveSeriesParams {
	return models.SaveSeriesParams{
		WorkspaceID: in.Membership.Workspace.ID, ID: in.ID, LockVersion: in.LockVersion, Name: in.Name,
		Description: strings.TrimSpace(in.Description), Working: working, WorkingHash: hash, TopicID: in.TopicID,
		Service: in.Service, EmailAddressID: in.EmailAddressID, SenderMemberID: in.SenderMemberID,
		SlackAs: in.SlackAs, ReplyTeamID: in.ReplyTeamID, ReplyMemberID: in.ReplyMemberID,
		AiReplies: in.AIReplies, MemberID: &in.Membership.Member.ID, Now: now,
	}
}
