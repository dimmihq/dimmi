/*
|--------------------------------------------------------------------------
| Setting Options
|--------------------------------------------------------------------------
|
| Settings that name something at the service are picked from lists the
| service gives, read through the connected account with the settings
| chosen so far, so a repository's branches follow the repository.
|
*/

package knowledgesources

import (
	"context"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
ErrNoOptions is a setting its driver does not list values for.
*/
var ErrNoOptions = fault.New(fault.Invalid, "knowledge_source_options_unsupported",
	"This setting has no values to choose from.").OnField("field")

/*
Options lists the values a setting of a source being added can take.
*/
func (e *Engine) Options(ctx context.Context, in SourceInput, field string) ([]sources.Option, error) {
	if !in.Membership.Can("knowledge:manage") {
		return nil, exceptions.ErrForbidden
	}

	d, err := e.driverOf(in.Type)
	if err != nil {
		return nil, err
	}

	chooser, ok := d.(sources.Chooser)
	if !ok {
		return nil, ErrNoOptions
	}

	cfg, err := e.connected(ctx, in.Membership.Workspace.ID, d, configOf(splitSecrets(d.Describe(), in.Settings)))
	if err != nil {
		return nil, checkError(err)
	}

	options, err := chooser.Options(ctx, cfg, field)

	return options, checkError(err)
}
