/*
|--------------------------------------------------------------------------
| Steps That Change The Traveler
|--------------------------------------------------------------------------
|
| Tags, attributes and topics change in the same transaction that moves
| the journey on, so they happen exactly once. A contact series changes
| the contact; an account series changes the company. Every change is
| checked against the workspace's other series afterwards, so tagging
| someone in one journey can start another.
|
*/

package series

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
action runs a step that changes the traveler.
*/
func (r *run) action(ctx context.Context, n Node) (outcome, error) {
	s := settingsOf[ActionSettings](n)

	var err error

	switch n.Kind {
	case KindTag:
		err = r.tag(ctx, r.decodeAll(models.PrefixTag, s.TagIDs))
	case KindUntag:
		err = r.untag(ctx, r.decodeAll(models.PrefixTag, s.TagIDs))
	case KindSetAttribute:
		err = r.attribute(ctx, s.Key, s.Value)
	case KindTopic:
		err = r.topic(ctx, s.TopicID, s.Subscribed)
	}

	if err != nil {
		return outcome{}, err
	}

	return outcome{path: PathNext, result: n.Kind}, r.recheck(ctx)
}

/*
tag puts tags on the traveler.
*/
func (r *run) tag(ctx context.Context, tags []ids.UUID) error {
	ws := r.e.WorkspaceID
	if r.e.Subject == SubjectCompany && r.e.CompanyID != nil {
		_, err := r.q.TagCompany(ctx, models.TagCompanyParams{
			WorkspaceID: ws, CompanyID: *r.e.CompanyID, CreatedByType: "series", CreatedByID: &r.e.SeriesID,
			Now: r.now, TagIds: tags,
		})

		return wrap("tag company", err)
	}

	if r.e.ContactID == nil {
		return nil
	}

	_, err := r.q.TagContact(ctx, models.TagContactParams{
		WorkspaceID: ws, ContactID: *r.e.ContactID, CreatedByType: "series", CreatedByID: &r.e.SeriesID,
		Now: r.now, TagIds: tags,
	})

	return wrap("tag contact", err)
}

/*
untag takes tags off the traveler.
*/
func (r *run) untag(ctx context.Context, tags []ids.UUID) error {
	ws := r.e.WorkspaceID
	if r.e.Subject == SubjectCompany && r.e.CompanyID != nil {
		_, err := r.q.UntagCompany(ctx, models.UntagCompanyParams{
			WorkspaceID: ws, CompanyID: *r.e.CompanyID, TagIds: tags,
		})

		return wrap("untag company", err)
	}

	if r.e.ContactID == nil {
		return nil
	}

	_, err := r.q.UntagContact(ctx, models.UntagContactParams{
		WorkspaceID: ws, ContactID: *r.e.ContactID, TagIds: tags,
	})

	return wrap("untag contact", err)
}

/*
attribute writes one custom attribute of the traveler.
*/
func (r *run) attribute(ctx context.Context, key string, value json.RawMessage) error {
	ws := r.e.WorkspaceID
	if r.e.Subject == SubjectCompany && r.e.CompanyID != nil {
		return wrap("set company attribute", r.q.SetSeriesCompanyAttribute(ctx, models.SetSeriesCompanyAttributeParams{
			WorkspaceID: ws, ID: *r.e.CompanyID, Key: key, Value: value, Now: r.now,
		}))
	}

	if r.e.ContactID == nil {
		return nil
	}

	return wrap("set contact attribute", r.q.SetSeriesContactAttribute(ctx, models.SetSeriesContactAttributeParams{
		WorkspaceID: ws, ID: *r.e.ContactID, Key: key, Value: value, Now: r.now,
	}))
}

/*
topic subscribes the contact to a topic or takes them off it.
*/
func (r *run) topic(ctx context.Context, public string, subscribed bool) error {
	topics := r.decodeAll(models.PrefixBroadcastTopic, []string{public})
	if len(topics) == 0 || r.e.ContactID == nil {
		return nil
	}

	return wrap("choose topic", r.q.SetSeriesTopicChoice(ctx, models.SetSeriesTopicChoiceParams{
		WorkspaceID: r.e.WorkspaceID, ContactID: *r.e.ContactID, TopicID: topics[0], Subscribed: subscribed,
		Now: r.now,
	}))
}

/*
recheck checks the traveler against every series again once the
change is committed.
*/
func (r *run) recheck(ctx context.Context) error {
	for _, c := range r.people {
		if err := r.s.Queue.PushTx(ctx, r.tx, jobs.CheckSeriesSubjectArgs{
			WorkspaceID: r.e.WorkspaceID, ContactID: c.ID,
		}); err != nil {
			return wrap("recheck traveler", err)
		}
	}

	return nil
}

/*
decodeAll reads public ids of one kind, skipping any that do not read.
*/
func (r *run) decodeAll(prefix models.Prefix, public []string) []ids.UUID {
	out := make([]ids.UUID, 0, len(public))

	for _, p := range public {
		if id, err := r.s.IDs.Decode(string(prefix), p); err == nil {
			out = append(out, id)
		}
	}

	return out
}
