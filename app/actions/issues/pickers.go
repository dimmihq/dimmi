/*
|--------------------------------------------------------------------------
| Pickers
|--------------------------------------------------------------------------
|
| Nothing about a tool is typed: teams, projects, workflow states,
| services, severities, users, fields and environments are pulled from
| the tool, and issues to link are found by search. A list pulled is
| kept for a day so pickers open at once; asking for it fresh pulls it
| again, as after adding a team in the tool. Reading them takes only
| tickets:read; what is picked is then checked again by the tool itself
| when used.
|
*/

package issues

import (
	"context"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
)

/*
optionsKept is how long a list pulled from a tool is kept.
*/
const optionsKept = 24 * time.Hour

/*
OptionsQuery names a list of an installation's tool and the parent it
is within, such as a team. Fresh pulls it from the tool again instead
of the copy kept.
*/
type OptionsQuery struct {
	Installation ids.UUID
	List         string
	Parent       string
	Fresh        bool
}

/*
Options returns one of a tool's lists, within the parent named, such
as the states of a team.

A fresh read pulls from the tool itself and never goes through the
cache: the copy kept in this process outlives Forget until the cache
server says the key changed.
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

	var failed error

	pull := func(ctx context.Context) ([]trackers.Option, error) {
		options, err := tl.tracker.Options(ctx, tl.call, in.List, in.Parent)
		if err != nil {
			failed = toolError(err)

			return nil, failed
		}

		if options == nil {
			options = []trackers.Option{}
		}

		return options, nil
	}

	key := "issue-options:" + m.Workspace.ID.String() + ":" + in.Installation.String() + ":" + in.List + ":" + in.Parent
	if s.Cache != nil && in.Fresh {
		_ = s.Cache.Forget(ctx, key)
	}

	if s.Cache == nil || in.Fresh {
		return pull(ctx)
	}

	options, err := cache.For[[]trackers.Option](s.Cache, optionsKept).Remember(ctx, key, pull)
	if failed != nil {
		return nil, failed
	}

	if err != nil {
		return pull(ctx)
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
