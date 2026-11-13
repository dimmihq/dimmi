/*
|--------------------------------------------------------------------------
| Linked Issues
|--------------------------------------------------------------------------
|
| A ticket stays dimmi's object; the engineering work behind it lives in
| Linear, Jira, GitHub, incident.io, Rootly or PagerDuty and is linked
| to it. Linking creates the issue (team and project picked from lists
| pulled from the tool) or links one pasted or found by search. The tool
| tells dimmi when the issue moves, by webhook and by a re-check every
| fifteen minutes, and each status maps to a dimmi state, so customers
| only ever see dimmi's states. Comments arrive as internal notes; a
| comment starting with @customers drafts a message to everyone
| linked. The impact on customers is kept
| current on the issue so engineering ranks work by it.
|
*/

package issues

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
Failures of linking issues.
*/
var (
	ErrIssueNotFound = fault.New(fault.NotFound, "ticket_issue_not_found",
		"No linked issue of this workspace has this id.")
	ErrInstallation = fault.New(fault.Invalid, "issue_installation_invalid",
		"Pick a connected issue tracker or incident tool.").OnField("installation_id")
	ErrNotFoundInTool = fault.New(fault.Invalid, "issue_not_found_in_tool",
		"The tool has no issue at this link or key, or dimmi cannot see it.").OnField("url")
	ErrToolRefused = fault.New(fault.FailedDependency, "issue_tool_unavailable",
		"The tool did not answer. Try again in a moment.")
	ErrCreateInvalid = fault.New(fault.Invalid, "issue_create_invalid",
		"Creating an issue needs a team and a title; linking one needs its URL or key.").OnField("team_id")
	ErrListUnknown = fault.New(fault.Invalid, "issue_list_unknown",
		"This tool offers no such list.").OnField("list")
)

/*
Issues links tickets to engineering work. AppURL is where links back
to dimmi point; Services holds the apps' webhook secrets.
*/
type Issues struct {
	DB       *database.DB
	Clock    clock.Clock
	Queue    *queue.Queue
	Tickets  *tickets.Tickets
	Tokens   *integrations.Tokens
	Registry *oauth.Registry
	Trackers trackers.Set
	Services appconfig.ServicesConfig
	AppURL   string
}

/*
Label returns the tool's name as people read it.
*/
func Label(provider string) string {
	switch provider {
	case "linear":
		return "Linear"
	case "jira":
		return "Jira"
	case "github":
		return "GitHub"
	case "incident_io":
		return "incident.io"
	case "rootly":
		return "Rootly"
	case "pagerduty":
		return "PagerDuty"
	}

	return provider
}

/*
tool is an installation with its tracker and the call to reach it.
*/
type tool struct {
	row      models.IntegrationInstallation
	tracker  trackers.Tracker
	call     trackers.Call
	settings SyncSettings
}

/*
toolFor reads the workspace's working installation of an engineering
tool and readies a call with its token. The context must carry the
workspace.
*/
func (s *Issues) toolFor(ctx context.Context, ws, id ids.UUID) (tool, error) {
	var row models.IntegrationInstallation

	err := s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		found, err := q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
			WorkspaceID: ws, ID: id,
		})
		row = found

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return tool{}, ErrInstallation
	}

	if err != nil {
		return tool{}, fmt.Errorf("find installation: %w", err)
	}

	tracker, ok := s.Trackers.Get(row.Provider)
	if !ok || row.Status != integrations.StatusActive {
		return tool{}, ErrInstallation
	}

	token, err := s.Tokens.Bot(tenancy.With(ctx, ws.String()), row.ID)
	if err != nil {
		return tool{}, ErrInstallation
	}

	t := tool{row: row, tracker: tracker, settings: settingsOf(row.Metadata)}
	t.call = trackers.Call{Token: token, Meta: t.meta()}

	return t, nil
}

/*
meta is what the tracker reads of the installation: its metadata, the
Jira site it reaches and the settings picked for it.
*/
func (t tool) meta() map[string]any {
	meta := decodeObject(t.row.Metadata)
	if _, ok := meta["cloud_id"]; !ok && t.row.ExternalAccountID != nil {
		meta["cloud_id"] = *t.row.ExternalAccountID
	}

	if t.settings.FromEmail != "" {
		meta["from_email"] = t.settings.FromEmail
	}

	if t.settings.ImpactField != "" {
		meta["impact_field"] = t.settings.ImpactField
	}

	return meta
}

/*
inWorkspace runs the work in one transaction of the workspace.
*/
func (s *Issues) inWorkspace(ctx context.Context, ws ids.UUID, fn func(ctx context.Context, q *models.Queries) error,
) error {
	ctx = tenancy.With(ctx, ws.String())

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
}

/*
toolError turns a tool's failure into what the API answers.
*/
func toolError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, trackers.ErrNotFound), errors.Is(err, trackers.ErrHidden):
		return ErrNotFoundInTool
	case errors.Is(err, trackers.ErrUnsupported):
		return ErrListUnknown
	}

	if _, ok := errors.AsType[*fault.Error](err); ok {
		return err
	}

	return ErrToolRefused
}

/*
providerOf guesses the tool a pasted URL belongs to, empty when it
names none.
*/
func providerOf(link string) string {
	lower := strings.ToLower(link)

	for _, p := range []struct{ host, provider string }{
		{"linear.app", "linear"},
		{"atlassian.net", "jira"},
		{"github.com", "github"},
		{"incident.io", "incident_io"},
		{"rootly.com", "rootly"},
		{"pagerduty.com", "pagerduty"},
	} {
		if strings.Contains(lower, p.host) {
			return p.provider
		}
	}

	return ""
}

/*
ticketLink is the address of a ticket in its workspace in dimmi.
*/
func (s *Issues) ticketLink(ctx context.Context, q *models.Queries, t models.Ticket) string {
	slug := "tickets"
	if ws, err := q.FindWorkspace(ctx, t.WorkspaceID); err == nil {
		slug = ws.Slug + "/tickets"
	}

	return strings.TrimRight(s.AppURL, "/") + "/d/" + slug + "/" + models.PublicID(models.PrefixTicket, t.ID)
}
