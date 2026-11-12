/*
|--------------------------------------------------------------------------
| Ticket Issue Resource
|--------------------------------------------------------------------------
|
| The work behind a ticket as the API shows it: linked issues and
| engineering incidents, and the service level clocks running on it.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketIssue is an issue or engineering incident behind a ticket.
*/
type TicketIssue struct {
	Object         string     `json:"object"`
	ID             string     `json:"id"`
	TicketID       string     `json:"ticket_id"`
	Provider       string     `json:"provider"`
	InstallationID *string    `json:"installation_id"`
	ExternalID     string     `json:"external_id"`
	Key            string     `json:"key"`
	URL            string     `json:"url"`
	Title          string     `json:"title"`
	Status         string     `json:"status"`
	StatusGroup    string     `json:"status_group"`
	TeamName       string     `json:"team_name"`
	Warning        *string    `json:"warning"`
	SyncError      *string    `json:"sync_error"`
	SyncedAt       *time.Time `json:"synced_at"`
	CreatedAt      time.Time  `json:"created_at"`
	TeamID         string     `json:"team_id"`
	Severity       string     `json:"severity"`
	DoneAt         *time.Time `json:"done_at"`
	DeployedAt     *time.Time `json:"deployed_at"`
}

/*
NewTicketIssues shapes a ticket's issues.
*/
func NewTicketIssues(list []models.TicketIssue) []TicketIssue {
	out := make([]TicketIssue, len(list))
	for i, is := range list {
		out[i] = NewTicketIssue(is)
	}

	return out
}

/*
NewTicketIssue shapes one issue link.
*/
func NewTicketIssue(is models.TicketIssue) TicketIssue {
	return TicketIssue{
		Object: "ticket_issue", ID: models.PublicID(models.PrefixTicketIssue, is.ID),
		TicketID: models.PublicID(models.PrefixTicket, is.TicketID), Provider: is.Provider,
		InstallationID: optionalID(models.PrefixIntegrationInstallation, is.InstallationID), ExternalID: is.ExternalID,
		Key: is.Key, URL: is.Url, Title: is.Title, Status: is.Status, StatusGroup: is.StatusGroup,
		TeamName: is.TeamName, Warning: is.Warning, SyncError: is.SyncError, SyncedAt: is.SyncedAt,
		CreatedAt: is.CreatedAt, TeamID: is.TeamID, Severity: is.Severity, DoneAt: is.DoneAt, DeployedAt: is.DeployedAt,
	}
}

/*
SLAClock is one service level target running on a ticket or a
conversation.
*/
type SLAClock struct {
	Object         string     `json:"object"`
	ID             string     `json:"id"`
	PolicyID       string     `json:"policy_id"`
	TicketID       *string    `json:"ticket_id"`
	ConversationID *string    `json:"conversation_id"`
	Target         string     `json:"target"`
	State          string     `json:"state"`
	StartedAt      time.Time  `json:"started_at"`
	DueAt          time.Time  `json:"due_at"`
	WarnAt         *time.Time `json:"warn_at"`
	PausedAt       *time.Time `json:"paused_at"`
	BreachedAt     *time.Time `json:"breached_at"`
	StoppedAt      *time.Time `json:"stopped_at"`
}

/*
NewSLAClocks shapes clocks.
*/
func NewSLAClocks(list []models.SlaClock) []SLAClock {
	out := make([]SLAClock, len(list))
	for i, c := range list {
		out[i] = NewSLAClock(c)
	}

	return out
}

/*
NewSLAClock shapes one clock.
*/
func NewSLAClock(c models.SlaClock) SLAClock {
	return SLAClock{
		Object: "sla_clock", ID: models.PublicID(models.PrefixSLAClock, c.ID),
		PolicyID:       models.PublicID(models.PrefixSLAPolicy, c.PolicyID),
		TicketID:       optionalID(models.PrefixTicket, c.TicketID),
		ConversationID: optionalID(models.PrefixConversation, c.ConversationID), Target: c.Target, State: c.State,
		StartedAt: c.StartedAt, DueAt: c.DueAt, WarnAt: c.WarnAt, PausedAt: c.PausedAt, BreachedAt: c.BreachedAt,
		StoppedAt: c.StoppedAt,
	}
}
