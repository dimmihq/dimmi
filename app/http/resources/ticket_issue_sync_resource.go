/*
|--------------------------------------------------------------------------
| Ticket Issue Sync Resources
|--------------------------------------------------------------------------
|
| What the API shows of a connected engineering tool: one value of a
| list to pick from, an issue found by search, the tool's sync settings
| with the address it delivers to, and a draft closing the loop with who
| it would reach and who would be held.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/issues"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IssueOption is one value of a tool's list: a team, project, workflow
state with its status group, severity with its rank, service, user,
field or environment.
*/
type IssueOption struct {
	Object string `json:"object"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
	Group  string `json:"group"`
	Rank   int    `json:"rank"`
}

/*
NewIssueOptions shapes a list.
*/
func NewIssueOptions(list []trackers.Option) []IssueOption {
	out := make([]IssueOption, len(list))
	for i, o := range list {
		out[i] = IssueOption{
			Object: "issue_option", ID: o.ID, Name: o.Name, Detail: o.Detail, Group: o.Group, Rank: o.Rank,
		}
	}

	return out
}

/*
IssueFound is an issue or incident found in a tool, ready to link.
*/
type IssueFound struct {
	Object      string `json:"object"`
	ExternalID  string `json:"external_id"`
	Key         string `json:"key"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	StatusGroup string `json:"status_group"`
	TeamName    string `json:"team_name"`
	Severity    string `json:"severity"`
}

/*
NewIssuesFound shapes search results.
*/
func NewIssuesFound(list []trackers.Issue) []IssueFound {
	out := make([]IssueFound, len(list))
	for i, is := range list {
		out[i] = IssueFound{
			Object: "issue_found", ExternalID: is.ExternalID, Key: is.Key, URL: is.URL, Title: is.Title,
			Status: is.Status, StatusGroup: is.StatusGroup, TeamName: is.TeamName, Severity: is.Severity,
		}
	}

	return out
}

/*
IssueSettings are a tool's sync settings: its statuses mapped to dimmi
state ids, the defaults new issues use, the deployment that proves a
fix shipped, where the tool delivers and whether the secret its
deliveries are signed with is set.
*/
type IssueSettings struct {
	Object            string            `json:"object"`
	InstallationID    string            `json:"installation_id"`
	Provider          string            `json:"provider"`
	States            map[string]string `json:"states"`
	TeamID            string            `json:"team_id"`
	ProjectID         string            `json:"project_id"`
	ServiceID         string            `json:"service_id"`
	SeverityID        string            `json:"severity_id"`
	FromEmail         string            `json:"from_email"`
	ImpactFieldID     string            `json:"impact_field_id"`
	DeployRepository  string            `json:"deploy_repository"`
	DeployEnvironment string            `json:"deploy_environment"`
	WebhookURL        string            `json:"webhook_url"`
	NeedsSigning      bool              `json:"needs_signing_secret"`
	HasSigningSecret  bool              `json:"has_signing_secret"`
}

/*
NewIssueSettings shapes a tool's settings, its state ids made public.
*/
func NewIssueSettings(v issues.SettingsView) IssueSettings {
	s := v.Settings
	states := map[string]string{}

	for status, state := range s.States {
		states[status] = publicStateID(state)
	}

	return IssueSettings{
		Object:         "issue_settings",
		InstallationID: models.PublicID(models.PrefixIntegrationInstallation, v.Installation.ID),
		Provider:       v.Installation.Provider, States: states, TeamID: s.Team, ProjectID: s.Project,
		ServiceID: s.Service, SeverityID: s.Severity, FromEmail: s.FromEmail,
		ImpactFieldID:    s.ImpactField,
		DeployRepository: s.DeployRepository, DeployEnvironment: s.DeployEnvironment, WebhookURL: v.WebhookURL,
		NeedsSigning: v.NeedsSecret, HasSigningSecret: v.HasSigningSecret,
	}
}

/*
publicStateID writes a stored state id as the API names it.
*/
func publicStateID(raw string) string {
	id, err := ids.Parse(raw)
	if err != nil {
		return raw
	}

	return models.PublicID(models.PrefixTicketState, id)
}

/*
TicketUpdateDraft is a drafted message to everyone a tracker links:
why it was drafted, its text, whether it waits for the
fix to deploy, and who it would reach now with those held by reason.
*/
type TicketUpdateDraft struct {
	Object     string         `json:"object"`
	ID         string         `json:"id"`
	TicketID   string         `json:"ticket_id"`
	IssueID    *string        `json:"issue_id"`
	Source     string         `json:"source"`
	Reason     string         `json:"reason"`
	Body       string         `json:"body"`
	AuthorName string         `json:"author_name"`
	Status     string         `json:"status"`
	Resolve    bool           `json:"resolve"`
	UpdateID   *string        `json:"update_id"`
	Recipients int            `json:"recipients"`
	Held       map[string]int `json:"held"`
	DecidedAt  *time.Time     `json:"decided_at"`
	CreatedAt  time.Time      `json:"created_at"`
}

/*
NewTicketUpdateDraft shapes a draft with its counts.
*/
func NewTicketUpdateDraft(v issues.DraftView) TicketUpdateDraft {
	d := v.Draft

	held := v.Held
	if held == nil {
		held = map[string]int{}
	}

	return TicketUpdateDraft{
		Object: "ticket_update_draft", ID: models.PublicID(models.PrefixTicketUpdateDraft, d.ID),
		TicketID: models.PublicID(models.PrefixTicket, d.TicketID),
		IssueID:  optionalID(models.PrefixTicketIssue, d.IssueID), Source: d.Source, Reason: d.Reason, Body: d.Body,
		AuthorName: d.AuthorName, Status: d.Status, Resolve: d.Resolve,
		UpdateID: optionalID(models.PrefixTicketUpdate, d.UpdateID), Recipients: v.Recipients, Held: held,
		DecidedAt: d.DecidedAt, CreatedAt: d.CreatedAt,
	}
}

/*
TicketSettings are the workspace's ticket switches.
*/
type TicketSettings struct {
	Object             string `json:"object"`
	AutoLinkDuplicates bool   `json:"auto_link_duplicates"`
}

/*
NewTicketSettings shapes the switches.
*/
func NewTicketSettings(s issues.TicketSettings) TicketSettings {
	return TicketSettings{
		Object: "ticket_settings", AutoLinkDuplicates: s.AutoLinkDuplicates,
	}
}
