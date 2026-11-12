/*
|--------------------------------------------------------------------------
| Issue Tool Controller
|--------------------------------------------------------------------------
|
| A connected engineering tool, seen from tickets: the teams and other
| lists its pickers offer, a search of its issues, its sync settings,
| connecting a tool that takes an API key, and the workspace's ticket
| switches those settings sit beside.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/actions/issues"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IssueToolController serves connected tools' lists, search and
settings.
*/
type IssueToolController struct {
	IDs          *ids.Registry
	Issues       *issues.Issues
	Integrations *integrations.CreateManualInstallation
	List         *integrations.ListInstallations
}

/*
IssueOptionsOutput is a list to pick from.
*/
type IssueOptionsOutput struct {
	Body fhttp.Collection[resources.IssueOption]
}

/*
Teams lists the tool's teams: Linear teams, Jira projects, GitHub
repositories.
*/
func (c *IssueToolController) Teams(ctx context.Context, in *requests.IssueTeamsRequest,
) (*IssueOptionsOutput, error) {
	return c.readOptions(ctx, in.Installation, trackers.ListTeams, "")
}

/*
Options lists any of the tool's lists within a parent.
*/
func (c *IssueToolController) Options(ctx context.Context, in *requests.IssueOptionsRequest,
) (*IssueOptionsOutput, error) {
	return c.readOptions(ctx, in.Installation, in.List, in.Parent)
}

/*
readOptions reads one list.
*/
func (c *IssueToolController) readOptions(ctx context.Context, installation, list, parent string,
) (*IssueOptionsOutput, error) {
	id, err := installationID(c.IDs, installation)
	if err != nil {
		return nil, err
	}

	query := issues.OptionsQuery{Installation: id, List: list, Parent: parent}

	found, err := c.Issues.Options(ctx, membership(ctx), query)
	if err != nil {
		return nil, err
	}

	items := resources.NewIssueOptions(found)

	return &IssueOptionsOutput{Body: fhttp.NewCollection(pagination.Page[resources.IssueOption]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
IssuesFoundOutput is a search's issues.
*/
type IssuesFoundOutput struct {
	Body fhttp.Collection[resources.IssueFound]
}

/*
Search finds the tool's issues to link.
*/
func (c *IssueToolController) Search(ctx context.Context, in *requests.IssueSearchRequest,
) (*IssuesFoundOutput, error) {
	id, err := installationID(c.IDs, in.Installation)
	if err != nil {
		return nil, err
	}

	found, err := c.Issues.Search(ctx, membership(ctx), id, in.Q)
	if err != nil {
		return nil, err
	}

	items := resources.NewIssuesFound(found)

	return &IssuesFoundOutput{Body: fhttp.NewCollection(pagination.Page[resources.IssueFound]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
IssueSettingsOutput is a tool's sync settings.
*/
type IssueSettingsOutput struct {
	Body fhttp.Resource[resources.IssueSettings]
}

/*
Settings reads a tool's sync settings.
*/
func (c *IssueToolController) Settings(ctx context.Context, in *requests.IssueSettingsRequest,
) (*IssueSettingsOutput, error) {
	id, err := installationID(c.IDs, in.Installation)
	if err != nil {
		return nil, err
	}

	v, err := c.Issues.Settings(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &IssueSettingsOutput{Body: fhttp.NewResource(resources.NewIssueSettings(v))}, nil
}

/*
UpdateSettings changes a tool's sync settings.
*/
func (c *IssueToolController) UpdateSettings(ctx context.Context, in *requests.UpdateIssueSettingsRequest,
) (*IssueSettingsOutput, error) {
	id, err := installationID(c.IDs, in.Installation)
	if err != nil {
		return nil, err
	}

	m := membership(ctx)

	current, err := c.Issues.Settings(ctx, m, id)
	if err != nil {
		return nil, err
	}

	set, err := c.merged(current.Settings, in)
	if err != nil {
		return nil, err
	}

	change := issues.SettingsInput{Settings: &set, SigningSecret: in.Body.SigningSecret}

	v, err := c.Issues.UpdateSettings(ctx, m, id, change)
	if err != nil {
		return nil, err
	}

	return &IssueSettingsOutput{Body: fhttp.NewResource(resources.NewIssueSettings(v))}, nil
}

/*
merged applies what the request changes to the stored settings, its
state ids read from their public form.
*/
func (c *IssueToolController) merged(set issues.SyncSettings, in *requests.UpdateIssueSettingsRequest,
) (issues.SyncSettings, error) {
	b := in.Body

	if b.States != nil {
		set.States = map[string]string{}

		for status, state := range b.States {
			if state == "" {
				continue
			}

			id, err := c.IDs.Decode(string(models.PrefixTicketState), state)
			if err != nil {
				return set, issues.ErrStateUnknown
			}

			set.States[status] = id.String()
		}
	}

	for _, f := range []struct {
		from *string
		to   *string
	}{
		{b.TeamID, &set.Team},
		{b.ProjectID, &set.Project},
		{b.ServiceID, &set.Service},
		{b.SeverityID, &set.Severity},
		{b.MirrorSeverityID, &set.MirrorSeverity},
		{b.FromEmail, &set.FromEmail},
		{b.ImpactFieldID, &set.ImpactField},
		{b.DeployRepository, &set.DeployRepository},
		{b.DeployEnvironment, &set.DeployEnvironment},
	} {
		if f.from != nil {
			*f.to = *f.from
		}
	}

	return set, nil
}

/*
StoreKey connects a tool that takes an API key, checked against the
tool first.
*/
func (c *IssueToolController) StoreKey(ctx context.Context, in *requests.StoreKeyInstallationRequest,
) (*IntegrationOutput, error) {
	m := membership(ctx)

	row, err := c.Integrations.Handle(ctx, integrations.ManualInstallationInput{
		Membership: m, Provider: in.Provider, Token: in.Body.APIKey, SigningSecret: in.Body.SigningSecret,
	})
	if err != nil {
		return nil, err
	}

	view, err := c.List.Show(ctx, m, row.ID)
	if err != nil {
		return nil, err
	}

	return &IntegrationOutput{Body: fhttp.NewResource(resources.NewIntegration(view))}, nil
}

/*
TicketSettingsOutput is the workspace's ticket switches.
*/
type TicketSettingsOutput struct {
	Body fhttp.Resource[resources.TicketSettings]
}

/*
TicketSettings reads the workspace's ticket switches.
*/
func (c *IssueToolController) TicketSettings(ctx context.Context, _ *requests.TicketSettingsRequest,
) (*TicketSettingsOutput, error) {
	s, err := c.Issues.WorkspaceSettings(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &TicketSettingsOutput{Body: fhttp.NewResource(resources.NewTicketSettings(s))}, nil
}

/*
UpdateTicketSettings changes the workspace's ticket switches.
*/
func (c *IssueToolController) UpdateTicketSettings(ctx context.Context, in *requests.UpdateTicketSettingsRequest,
) (*TicketSettingsOutput, error) {
	b := in.Body

	s, err := c.Issues.UpdateWorkspaceSettings(ctx, membership(ctx), issues.TicketSettingsChange{
		IncidentMirror:         b.IncidentMirror,
		IncidentMirrorSeverity: b.IncidentMirrorSeverity, AutoLinkDuplicates: b.AutoLinkDuplicates,
		SpikeThreshold: b.SpikeThreshold,
	})
	if err != nil {
		return nil, err
	}

	return &TicketSettingsOutput{Body: fhttp.NewResource(resources.NewTicketSettings(s))}, nil
}
