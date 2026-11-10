/*
|--------------------------------------------------------------------------
| Broadcast Drafts
|--------------------------------------------------------------------------
|
| A PATCH names only what changed. Each field it names replaces the
| stored one; public ids are decoded, and an empty id clears the field,
| so the composer can unset an audience or a topic.
|
*/

package v1

import (
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
merge lays the request's fields over the stored draft.
*/
func (c *BroadcastController) merge(d broadcasts.Draft, in requests.BroadcastBody) (broadcasts.Draft, error) {
	setText(&d.Name, in.Name)
	setText(&d.Subject, in.Subject)
	setText(&d.Preview, in.Preview)
	setText(&d.Body, in.Body)
	setText(&d.SlackAs, in.SlackAs)
	setFlag(&d.Service, in.Service)
	setFlag(&d.AIReplies, in.AIReplies)
	setFlag(&d.TrackClicks, in.TrackClicks)
	setFlag(&d.SkipFrequencyCap, in.SkipFrequencyCap)

	if in.Rules != nil {
		d.Rules = in.Rules
	}

	if in.Goal != nil {
		d.Goal = in.Goal
	}

	if in.Schedule != nil {
		d.ScheduleKind, d.ScheduledAt = in.Schedule.Kind, in.Schedule.At
	}

	if err := c.mergeRefs(&d, in); err != nil {
		return d, err
	}

	return d, nil
}

/*
mergeRefs decodes the ids the request names over the draft's.
*/
func (c *BroadcastController) mergeRefs(d *broadcasts.Draft, in requests.BroadcastBody) error {
	refs := []struct {
		to     **ids.UUID
		from   *string
		prefix models.Prefix
	}{
		{&d.AudienceID, in.AudienceID, models.PrefixAudience},
		{&d.TopicID, in.TopicID, models.PrefixBroadcastTopic},
		{&d.SenderMemberID, in.SenderID, models.PrefixMember},
		{&d.EmailAddressID, in.EmailAddressID, models.PrefixEmailAddress},
		{&d.ReplyTeamID, in.ReplyTeamID, models.PrefixTeam},
		{&d.ReplyMemberID, in.ReplyMemberID, models.PrefixMember},
	}

	for _, r := range refs {
		if r.from == nil {
			continue
		}

		id, err := c.optional(r.prefix, *r.from)
		if err != nil {
			return err
		}

		*r.to = id
	}

	if in.RoomIDs == nil {
		return nil
	}

	d.RoomIDs = []ids.UUID{}

	for _, public := range in.RoomIDs {
		id, err := c.optional(models.PrefixChannelRoom, public)
		if err != nil || id == nil {
			return err
		}

		d.RoomIDs = append(d.RoomIDs, *id)
	}

	return nil
}

/*
setText replaces the text when the request names it.
*/
func setText(to, from *string) {
	if from != nil {
		*to = *from
	}
}

/*
setFlag replaces the flag when the request names it.
*/
func setFlag(to, from *bool) {
	if from != nil {
		*to = *from
	}
}
