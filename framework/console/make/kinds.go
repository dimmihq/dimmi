/*
|--------------------------------------------------------------------------
| Kinds of Files
|--------------------------------------------------------------------------
|
| Each kind knows where its files live and how they are named, following
| the application layout: actions grouped by domain under app/actions,
| one directory each for commands, events, jobs, listeners and mail,
| migrations under database/migrations and feature tests under
| tests/feature.
|
*/

package make

import (
	"fmt"
	"path"
	"regexp"

	"github.com/iancoleman/strcase"

	"github.com/dimmihq/dimmi/framework/console"
)

/*
kind is one make command: its name and description, how its name
argument reads, any extra flags, and how a parsed name becomes a file.
Holds says what a package of these files holds; kinds that create no Go
package leave it empty.
*/
type kind struct {
	name        string
	description string
	argument    string
	holds       string
	flags       func(flags *console.Flags)
	plan        func(n name, in *console.Input, g Generator) (file, error)
}

var (
	createTableRe = regexp.MustCompile(`^create_([a-z][a-z0-9_]*)_table$`)
	migrationRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

func kinds() []kind {
	return append(appKinds(), httpKinds()...)
}

/*
appKinds are the kinds of files outside the HTTP layer.
*/
func appKinds() []kind {
	return []kind{
		{
			name: "action", description: "Create an action", holds: "business actions",
			argument: "the action, in its domain folder: tags/CreateTag",
			plan:     nested("app/actions", "", ".go", "action.go.tmpl"),
		},
		{
			name: "command", description: "Create a console command", holds: "console commands",
			argument: "the command type: PruneTags", flags: commandFlags, plan: planCommand,
		},
		{
			name: "event", description: "Create an event", holds: "the events the application dispatches",
			argument: "the event, in the past tense: TagCreated", plan: flat("app/events", ".go", "event.go.tmpl"),
		},
		{
			name: "job", description: "Create a queued job", holds: "queued jobs",
			argument: "the job, in the imperative: DeliverWebhook", plan: flat("app/jobs", ".go", "job.go.tmpl"),
		},
		{
			name: "listener", description: "Create an event listener", holds: "event listeners",
			argument: "what the listener does: SendWelcomeMail", flags: listenerFlags, plan: planListener,
		},
		{
			name: "mail", description: "Create a mailable", holds: "the mail the application sends",
			argument: "the mailable: InvitationMail", plan: flat("app/mail", ".go", "mail.go.tmpl"),
		},
		{
			name: "migration", description: "Create a database migration",
			argument: "what it does, in snake case: create_tags_table", plan: planMigration,
		},
		{
			name: "test", description: "Create a feature test",
			argument: "the behavior under test, in an optional folder: tags/TagCreation",
			plan:     nested("tests/feature", "Test", "_test.go", "test.go.tmpl"),
		},
	}
}

func commandFlags(flags *console.Flags) {
	flags.String("signature", "", "what the command is called with (default: its name in kebab case)")
}

func listenerFlags(flags *console.Flags) {
	flags.String("event", "", "the event type it handles, from app/events")
	flags.Bool("queued", "push a job instead of running in the request")
}

/*
nested plans a file in the directory or a folder under it, named after
the type with the suffix removed and the file suffix added.
*/
func nested(dir, suffix, fileSuffix, stub string) func(name, *console.Input, Generator) (file, error) {
	return func(n name, _ *console.Input, _ Generator) (file, error) {
		n = n.without(suffix)
		if !n.valid() {
			return file{}, fmt.Errorf("%w: %q is not a Go type name", ErrInvalidName, n.base)
		}

		folder := path.Join(dir, n.folder)

		return file{path: path.Join(folder, n.snake()+fileSuffix), stub: stub, data: n.data(path.Base(folder))}, nil
	}
}

/*
flat plans a file directly in the directory, refusing folders.
*/
func flat(dir, fileSuffix, stub string) func(name, *console.Input, Generator) (file, error) {
	return func(n name, _ *console.Input, _ Generator) (file, error) {
		if n.folder != "" {
			return file{}, fmt.Errorf("%w: %s takes no folder", ErrInvalidName, n.raw)
		}

		if !n.valid() {
			return file{}, fmt.Errorf("%w: %q is not a Go type name", ErrInvalidName, n.base)
		}

		return file{path: path.Join(dir, n.snake()+fileSuffix), stub: stub, data: n.data(path.Base(dir))}, nil
	}
}

func planCommand(n name, in *console.Input, g Generator) (file, error) {
	n = n.without("Command")

	f, err := flat("app/console/commands", "_command.go", "command.go.tmpl")(n, in, g)
	if err != nil {
		return file{}, err
	}

	f.data.Signature = in.String("signature")
	if f.data.Signature == "" {
		f.data.Signature = strcase.ToKebab(n.base)
	}

	return f, nil
}

func planListener(n name, in *console.Input, g Generator) (file, error) {
	stub := "listener.go.tmpl"
	if in.Bool("queued") {
		stub = "listener.queued.go.tmpl"
	}

	f, err := flat("app/listeners", ".go", stub)(n, in, g)
	if err != nil {
		return file{}, err
	}

	if event := in.String("event"); event != "" {
		if !typeRe.MatchString(event) {
			return file{}, fmt.Errorf("%w: event %q", ErrInvalidName, event)
		}

		f.data.Event = event
	}

	return f, nil
}

/*
planMigration names the file after the migration with a version from
the clock, using the create-table stub for names like
create_tags_table.
*/
func planMigration(n name, _ *console.Input, g Generator) (file, error) {
	if !migrationRe.MatchString(n.raw) {
		return file{}, fmt.Errorf("%w: %q is not snake case", ErrInvalidName, n.raw)
	}

	version := g.Clock.Now().UTC().Format("20060102150405")
	f := file{path: path.Join("database/migrations", version+"_"+n.raw+".sql"), stub: "migration.sql.tmpl"}

	if match := createTableRe.FindStringSubmatch(n.raw); match != nil {
		f.stub = "migration.create.sql.tmpl"
		f.data.Table = match[1]
	}

	return f, nil
}
