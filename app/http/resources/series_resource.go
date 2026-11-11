/*
|--------------------------------------------------------------------------
| Series Resources
|--------------------------------------------------------------------------
|
| How series, their versions, their journeys and the steps of a journey
| read over the API. Ids are public ids. The definition reads as the
| same JSON the builder saves.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Series is one series with its working copy, its sending settings and
its totals.
*/
type Series struct {
	ID             string           `json:"id"`
	Object         string           `json:"object"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	Subject        string           `json:"subject"`
	Status         string           `json:"status"`
	Spec           json.RawMessage  `json:"spec"`
	Problems       []series.Problem `json:"problems"`
	LiveVersionID  *string          `json:"live_version_id"`
	HasChanges     bool             `json:"has_unpublished_changes"`
	LockVersion    int32            `json:"lock_version"`
	TopicID        *string          `json:"topic_id"`
	Service        bool             `json:"service"`
	EmailAddressID *string          `json:"email_address_id"`
	SenderID       *string          `json:"sender_id"`
	SlackAs        string           `json:"slack_as"`
	ReplyTeamID    *string          `json:"reply_team_id"`
	ReplyMemberID  *string          `json:"reply_member_id"`
	AIReplies      bool             `json:"ai_replies"`
	Counts         json.RawMessage  `json:"counts"`
	Totals         *SeriesTotals    `json:"totals,omitempty"`
	PublishedAt    *time.Time       `json:"published_at"`
	PausedAt       *time.Time       `json:"paused_at"`
	ArchivedAt     *time.Time       `json:"archived_at"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

/*
SeriesTotals is how many entered a series, are on it now, finished,
left and reached the goal.
*/
type SeriesTotals struct {
	Entered  int64 `json:"entered"`
	Active   int64 `json:"active"`
	Finished int64 `json:"finished"`
	Exited   int64 `json:"exited"`
	Goal     int64 `json:"goal"`
}

/*
NewSeries shapes a series. LiveHash is the hash of the version that
runs, so the builder knows whether the working copy changed since.
*/
func NewSeries(s models.Series, liveHash string) Series {
	problems := []series.Problem{}
	if spec, err := series.SpecOf(s); err == nil {
		if found := series.Check(spec, s.Subject); found != nil {
			problems = found
		}
	}

	return Series{
		ID: models.PublicID(models.PrefixSeries, s.ID), Object: "series", Name: s.Name, Description: s.Description,
		Subject: s.Subject, Status: s.Status, Spec: s.Working, Problems: problems,
		LiveVersionID: broadcastRef(models.PrefixSeriesVersion, s.LiveVersionID),
		HasChanges:    s.LiveVersionID == nil || liveHash != s.WorkingHash, LockVersion: s.LockVersion,
		TopicID: broadcastRef(models.PrefixBroadcastTopic, s.TopicID), Service: s.Service,
		EmailAddressID: broadcastRef(models.PrefixEmailAddress, s.EmailAddressID),
		SenderID:       broadcastRef(models.PrefixMember, s.SenderMemberID), SlackAs: s.SlackAs,
		ReplyTeamID: broadcastRef(models.PrefixTeam, s.ReplyTeamID), ReplyMemberID: broadcastRef(models.PrefixMember,
			s.ReplyMemberID),
		AIReplies: s.AiReplies, Counts: s.Counts, PublishedAt: s.PublishedAt, PausedAt: s.PausedAt,
		ArchivedAt: s.ArchivedAt, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

/*
SeriesVersion is one published version of a series.
*/
type SeriesVersion struct {
	ID          string          `json:"id"`
	Object      string          `json:"object"`
	Number      int32           `json:"number"`
	Note        string          `json:"note"`
	Spec        json.RawMessage `json:"spec"`
	PublishedBy *string         `json:"published_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

/*
NewSeriesVersion shapes a version.
*/
func NewSeriesVersion(v models.SeriesVersion) SeriesVersion {
	return SeriesVersion{
		ID: models.PublicID(models.PrefixSeriesVersion, v.ID), Object: "series_version", Number: v.Number,
		Note: v.Note, Spec: v.Spec, PublishedBy: broadcastRef(models.PrefixMember, v.PublishedByMemberID),
		CreatedAt: v.CreatedAt,
	}
}

/*
SeriesEnrollment is one journey: who travels, where they stand, and how
it ended.
*/
type SeriesEnrollment struct {
	ID           string     `json:"id"`
	Object       string     `json:"object"`
	SeriesID     string     `json:"series_id"`
	SeriesName   string     `json:"series_name,omitempty"`
	ContactID    *string    `json:"contact_id"`
	ContactName  string     `json:"contact_name,omitempty"`
	ContactEmail string     `json:"contact_email,omitempty"`
	CompanyID    *string    `json:"company_id"`
	CompanyName  string     `json:"company_name,omitempty"`
	Status       string     `json:"status"`
	NodeID       string     `json:"node_id"`
	WakeAt       *time.Time `json:"wake_at"`
	EnteredAt    time.Time  `json:"entered_at"`
	EnteredBy    string     `json:"entered_by"`
	GoalAt       *time.Time `json:"goal_at"`
	LeftAt       *time.Time `json:"left_at"`
	EndReason    *string    `json:"end_reason"`
	Test         bool       `json:"test"`
}

/*
NewSeriesEnrollment shapes a journey.
*/
func NewSeriesEnrollment(e models.SeriesEnrollment) SeriesEnrollment {
	return SeriesEnrollment{
		ID: models.PublicID(models.PrefixSeriesEnrollment, e.ID), Object: "series_enrollment",
		SeriesID: models.PublicID(models.PrefixSeries, e.SeriesID), ContactID: broadcastRef(models.PrefixContact,
			e.ContactID),
		CompanyID: broadcastRef(models.PrefixCompany, e.CompanyID), Status: e.Status, NodeID: e.NodeID,
		WakeAt:    e.WakeAt,
		EnteredAt: e.EnteredAt, EnteredBy: e.EnteredBy, GoalAt: e.GoalAt, LeftAt: e.LeftAt, EndReason: e.EndReason,
		Test: e.Test,
	}
}

/*
NewSeriesJourney shapes a journey listed with who travels it.
*/
func NewSeriesJourney(j series.Journey) SeriesEnrollment {
	out := NewSeriesEnrollment(models.SeriesEnrollment{
		ID: j.ID, SeriesID: j.SeriesID, ContactID: j.ContactID, CompanyID: j.CompanyID, Status: j.Status,
		NodeID: j.NodeID, WakeAt: j.WakeAt, EnteredAt: j.EnteredAt, EnteredBy: j.EnteredBy, GoalAt: j.GoalAt,
		LeftAt: j.LeftAt, EndReason: j.EndReason, Test: j.Test,
	})
	out.ContactName, out.ContactEmail, out.CompanyName = j.ContactName, j.ContactEmail, j.CompanyName

	return out
}
