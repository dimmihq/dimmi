/*
|--------------------------------------------------------------------------
| Changelog Resources
|--------------------------------------------------------------------------
|
| How a changelog's entries, labels and subscribers read over the API.
| An entry carries its working copy with where it stands, the labels it
| is filed under, its authors and the teammates who edited it. Lists
| leave the body out; one entry carries it.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ChangelogEntry is an entry's working copy and where it stands.
*/
type ChangelogEntry struct {
	ID                    string            `json:"id"`
	Object                string            `json:"object"`
	HelpCenterID          string            `json:"help_center_id"`
	Title                 string            `json:"title"`
	Slug                  string            `json:"slug"`
	Summary               *string           `json:"summary"`
	BodyMarkdown          *string           `json:"body_markdown,omitempty"`
	CoverURL              *string           `json:"cover_url"`
	Status                string            `json:"status"`
	ScheduledFor          *time.Time        `json:"scheduled_for"`
	PublishedAt           *time.Time        `json:"published_at"`
	HasUnpublishedChanges bool              `json:"has_unpublished_changes"`
	NotifySubscribers     bool              `json:"notify_subscribers"`
	NotifiedAt            *time.Time        `json:"notified_at"`
	AuthorsChosen         bool              `json:"authors_chosen"`
	Labels                []ChangelogTag    `json:"labels"`
	Authors               []ChangelogPerson `json:"authors"`
	Editors               []ChangelogPerson `json:"editors"`
	LockVersion           int32             `json:"lock_version"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

/*
ChangelogTag is a label as an entry shows it.
*/
type ChangelogTag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Color string `json:"color"`
}

/*
ChangelogPerson is a teammate named on an entry.
*/
type ChangelogPerson struct {
	MemberID  string  `json:"member_id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
NewChangelogEntry shapes one entry with its body.
*/
func NewChangelogEntry(e changelog.Entry) ChangelogEntry {
	out := changelogSummary(e)
	out.BodyMarkdown = &e.BodyMarkdown
	return out
}

/*
NewChangelogEntries shapes entries as lists show them.
*/
func NewChangelogEntries(rows []changelog.Entry) []ChangelogEntry {
	out := make([]ChangelogEntry, len(rows))
	for i, r := range rows {
		out[i] = changelogSummary(r)
	}
	return out
}

func changelogSummary(e changelog.Entry) ChangelogEntry {
	out := ChangelogEntry{
		ID: models.PublicID(models.PrefixChangelogEntry, e.ID), Object: "changelog_entry",
		HelpCenterID: models.PublicID(models.PrefixHelpCenter, e.HelpCenterID), Title: e.Title, Slug: e.Slug,
		Summary: e.Summary, CoverURL: e.CoverUrl, Status: e.Status, ScheduledFor: e.ScheduledFor,
		PublishedAt: e.PublishedAt, HasUnpublishedChanges: e.HasUnpublishedChanges && e.Status == "published",
		NotifySubscribers: e.NotifySubscribers, NotifiedAt: e.NotifiedAt, AuthorsChosen: e.AuthorsChosen,
		Labels: make([]ChangelogTag, len(e.Labels)), Authors: people(e.Authors), Editors: people(e.Editors),
		LockVersion: e.LockVersion, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
	for i, l := range e.Labels {
		out.Labels[i] = ChangelogTag{
			ID: models.PublicID(models.PrefixChangelogLabel, l.ID), Name: l.Name, Slug: l.Slug, Color: l.Color,
		}
	}
	return out
}

func people(in []changelog.Person) []ChangelogPerson {
	out := make([]ChangelogPerson, len(in))
	for i, p := range in {
		out[i] = ChangelogPerson{
			MemberID: models.PublicID(models.PrefixMember, p.MemberID), Name: p.Name,
			AvatarURL: models.User{AvatarPath: p.Avatar}.AvatarURL(),
		}
	}
	return out
}

/*
ChangelogLabel is one of a changelog's labels.
*/
type ChangelogLabel struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Color    string `json:"color"`
	Position int32  `json:"position"`
}

/*
NewChangelogLabels shapes a changelog's labels.
*/
func NewChangelogLabels(rows []models.ChangelogLabel) []ChangelogLabel {
	out := make([]ChangelogLabel, len(rows))
	for i, r := range rows {
		out[i] = NewChangelogLabel(r)
	}
	return out
}

/*
NewChangelogLabel shapes one label.
*/
func NewChangelogLabel(l models.ChangelogLabel) ChangelogLabel {
	return ChangelogLabel{
		ID: models.PublicID(models.PrefixChangelogLabel, l.ID), Object: "changelog_label", Name: l.Name,
		Slug: l.Slug, Color: l.Color, Position: l.Position,
	}
}

/*
ChangelogSubscriber is a person who asked for a changelog's updates.
*/
type ChangelogSubscriber struct {
	ID             string     `json:"id"`
	Object         string     `json:"object"`
	Email          string     `json:"email"`
	ContactID      *string    `json:"contact_id"`
	Status         string     `json:"status"`
	ConfirmedAt    *time.Time `json:"confirmed_at"`
	UnsubscribedAt *time.Time `json:"unsubscribed_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

/*
NewChangelogSubscribers shapes subscribers.
*/
func NewChangelogSubscribers(rows []models.ChangelogSubscriber) []ChangelogSubscriber {
	out := make([]ChangelogSubscriber, len(rows))
	for i, r := range rows {
		out[i] = ChangelogSubscriber{
			ID: models.PublicID(models.PrefixChangelogSubscriber, r.ID), Object: "changelog_subscriber",
			Email: r.Email, Status: r.Status, ConfirmedAt: r.ConfirmedAt, UnsubscribedAt: r.UnsubscribedAt,
			CreatedAt: r.CreatedAt,
		}
		if r.ContactID != nil {
			out[i].ContactID = new(models.PublicID(models.PrefixContact, *r.ContactID))
		}
	}
	return out
}
