/*
|--------------------------------------------------------------------------
| Sync Settings
|--------------------------------------------------------------------------
|
| Each connected tool keeps its own settings in its installation's
| metadata, every value picked from lists pulled from the tool: which of
| its statuses moves a ticket to which dimmi state, the team and project
| new issues go to, the PagerDuty service and user to page as, the
| severity new incidents are declared with, the incident.io field the
| customer count is written to, and the GitHub repository and
| environment whose deployments prove a fix shipped. A tool connected
| with an API key also has the secret its deliveries are signed with.
|
| A status nobody mapped follows the defaults: started is "Being worked
| on", done closes the loop and canceled is "Won't fix".
|
*/

package issues

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
settingsKey is where an installation's metadata keeps the settings.
*/
const settingsKey = "ticket_sync"

/*
ErrStateUnknown refuses a mapping to a state the workspace does not
have.
*/
var ErrStateUnknown = fault.New(fault.Invalid, "ticket_state_not_found",
	"A status is mapped to a state this workspace does not have.").OnField("states")

/*
SyncSettings are a tool's settings. States maps the tool's status ids
to dimmi state ids.
*/
type SyncSettings struct {
	States            map[string]string `json:"states,omitempty"`
	Team              string            `json:"team,omitempty"`
	Project           string            `json:"project,omitempty"`
	Service           string            `json:"service,omitempty"`
	Severity          string            `json:"severity,omitempty"`
	FromEmail         string            `json:"from_email,omitempty"`
	ImpactField       string            `json:"impact_field,omitempty"`
	DeployRepository  string            `json:"deploy_repository,omitempty"`
	DeployEnvironment string            `json:"deploy_environment,omitempty"`
}

/*
SettingsView is a tool's settings with where its deliveries go and
whether their signing secret is set.
*/
type SettingsView struct {
	Installation     models.IntegrationInstallation
	Settings         SyncSettings
	WebhookURL       string
	HasSigningSecret bool
	NeedsSecret      bool
}

/*
settingsOf reads the settings in an installation's metadata.
*/
func settingsOf(metadata json.RawMessage) SyncSettings {
	var holder struct {
		Settings SyncSettings `json:"ticket_sync"`
	}

	_ = json.Unmarshal(metadata, &holder)

	return holder.Settings
}

/*
decodeObject reads a JSON object, empty when it is none.
*/
func decodeObject(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)

	return out
}

/*
installation reads an installation of the member's workspace.
*/
func (s *Issues) installation(ctx context.Context, m authz.Membership, id ids.UUID,
) (models.IntegrationInstallation, error) {
	var row models.IntegrationInstallation

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		found, err := q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
			WorkspaceID: m.Workspace.ID, ID: id,
		})
		row = found

		return err
	})
	if err != nil {
		return row, exceptions.ErrNotFound
	}

	if _, ok := s.Trackers.Get(row.Provider); !ok {
		return row, exceptions.ErrNotFound
	}

	return row, nil
}

/*
Settings returns a tool's settings.
*/
func (s *Issues) Settings(ctx context.Context, m authz.Membership, id ids.UUID) (SettingsView, error) {
	row, err := s.installation(ctx, m, id)
	if err != nil {
		return SettingsView{}, err
	}

	return s.view(ctx, row), nil
}

/*
view shapes the settings of the installation.
*/
func (s *Issues) view(ctx context.Context, row models.IntegrationInstallation) SettingsView {
	p, _ := s.Registry.Provider(row.Provider)

	return SettingsView{
		Installation: row, Settings: settingsOf(row.Metadata), WebhookURL: s.WebhookURL(row),
		NeedsSecret:      p.Key,
		HasSigningSecret: p.Key && s.Tokens.HasSigningSecret(tenancy.With(ctx, row.WorkspaceID.String()), row.ID),
	}
}

/*
WebhookURL is where the tool delivers: the app-wide address of Linear
and Jira, a GitHub address naming the installation, and each key-
connected tool's own address.
*/
func (s *Issues) WebhookURL(row models.IntegrationInstallation) string {
	base := s.AppURL + "/webhooks/"
	id := models.PublicID(models.PrefixIntegrationInstallation, row.ID)

	switch row.Provider {
	case "linear", "jira":
		return base + row.Provider
	case "github":
		return base + "github-issues?installation=" + id
	}

	return base + strings.ReplaceAll(row.Provider, "_", "-") + "/" + id
}

/*
SettingsInput changes a tool's settings; nil leaves them, and a
signing secret is only set when given.
*/
type SettingsInput struct {
	Settings      *SyncSettings
	SigningSecret string
}

/*
UpdateSettings stores a tool's settings. Teammates who manage
integrations may.
*/
func (s *Issues) UpdateSettings(ctx context.Context, m authz.Membership, id ids.UUID, in SettingsInput,
) (SettingsView, error) {
	if !m.Can(integrations.ManagePermission) {
		return SettingsView{}, exceptions.ErrForbidden
	}

	row, err := s.installation(ctx, m, id)
	if err != nil {
		return SettingsView{}, err
	}

	if in.SigningSecret != "" {
		err := s.Tokens.SetSigningSecret(tenancy.With(ctx, m.Workspace.ID.String()), m, row.ID, in.SigningSecret)
		if err != nil {
			return SettingsView{}, err
		}
	}

	if in.Settings != nil {
		if row, err = s.saveSettings(ctx, m.Workspace.ID, row, *in.Settings); err != nil {
			return SettingsView{}, err
		}
	}

	return s.view(ctx, row), nil
}

/*
saveSettings checks every mapped state is the workspace's and stores
the settings.
*/
func (s *Issues) saveSettings(ctx context.Context, ws ids.UUID, row models.IntegrationInstallation, set SyncSettings,
) (models.IntegrationInstallation, error) {
	raw, err := json.Marshal(map[string]any{settingsKey: set})
	if err != nil {
		return row, fmt.Errorf("encode settings: %w", err)
	}

	err = s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		for _, state := range set.States {
			id, err := ids.Parse(state)
			if err != nil {
				return ErrStateUnknown
			}

			if _, err := q.FindTicketState(ctx, models.FindTicketStateParams{WorkspaceID: ws, ID: id}); err != nil {
				return ErrStateUnknown
			}
		}

		row, err = q.MergeIntegrationMetadata(ctx, models.MergeIntegrationMetadataParams{
			Metadata: raw, Now: s.Clock.Now(), WorkspaceID: ws, ID: row.ID,
		})

		return err
	})

	return row, err
}
