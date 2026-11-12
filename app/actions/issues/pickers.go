/*
|--------------------------------------------------------------------------
| Pickers
|--------------------------------------------------------------------------
|
| Nothing about a tool is typed: teams, projects, workflow states,
| services, severities, users, fields and environments are pulled from
| the tool when a picker opens, and issues to link are found by search.
| Reading them takes only tickets:read; what is picked is then checked
| again by the tool itself when used.
|
*/

package issues

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
)

/*
OptionsQuery names a list of an installation's tool and the parent it
is within, such as a team.
*/
type OptionsQuery struct {
	Installation ids.UUID
	List         string
	Parent       string
}

/*
Options returns one of a tool's lists, within the parent named, such
as the states of a team.
*/
func (s *Issues) Options(ctx context.Context, m authz.Membership, in OptionsQuery) ([]trackers.Option, error) {
	if err := tickets.MayRead(m); err != nil {
		return nil, err
	}

	if in.List == "" {
		in.List = trackers.ListTeams
	}

	tl, err := s.toolFor(ctx, m.Workspace.ID, in.Installation)
	if err != nil {
		return nil, err
	}

	options, err := tl.tracker.Options(ctx, tl.call, in.List, in.Parent)
	if err != nil {
		return nil, toolError(err)
	}

	if options == nil {
		options = []trackers.Option{}
	}

	return options, nil
}

/*
Search finds issues in a tool to link.
*/
func (s *Issues) Search(ctx context.Context, m authz.Membership, id ids.UUID, q string) ([]trackers.Issue, error) {
	if err := tickets.MayRead(m); err != nil {
		return nil, err
	}

	tl, err := s.toolFor(ctx, m.Workspace.ID, id)
	if err != nil {
		return nil, err
	}

	found, err := tl.tracker.Search(ctx, tl.call, strings.TrimSpace(q))
	if err != nil {
		return nil, toolError(err)
	}

	if found == nil {
		found = []trackers.Issue{}
	}

	return found, nil
}
