package make_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/console/make"
	"github.com/dimmihq/dimmi/framework/console/stubs"
	"github.com/dimmihq/dimmi/framework/database/migrate"
)

/*
generator returns a generator writing into a fresh project directory.
*/
func generator(t *testing.T) make.Generator {
	t.Helper()

	return make.Generator{
		Stubs:  stubs.FS(),
		Module: "example.com/shop",
		Root:   t.TempDir(),
		Clock:  clock.NewFake(time.Date(2026, 11, 2, 9, 30, 15, 0, time.UTC)),
	}
}

/*
generate runs a make command and returns its exit code, standard output
and standard error.
*/
func generate(t *testing.T, g make.Generator, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	app := console.New("app", "test")
	app.Register(g.Commands()...)

	code := app.Run(t.Context(), args, console.IO{In: strings.NewReader(""), Out: &stdout, Err: &stderr})

	return code, stdout.String(), stderr.String()
}

/*
read returns a generated file's contents.
*/
func read(t *testing.T, g make.Generator, path string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(g.Root, filepath.FromSlash(path)))
	require.NoError(t, err)

	return string(contents)
}

func TestGenerator(t *testing.T) {
	t.Parallel()

	files := map[string]struct {
		args     []string
		path     string
		contains []string
	}{
		"an action in its domain folder": {
			args: []string{"make:action", "tags/create_tag"},
			path: "app/actions/tags/create_tag.go",
			contains: []string{
				"| Create Tag\n", "package tags\n", "type CreateTagInput struct{}",
				"func (a *CreateTag) Handle(_ context.Context, _ CreateTagInput) error {",
			},
		},
		"a command named in kebab case": {
			args: []string{"make:command", "PruneTagsCommand"},
			path: "app/console/commands/prune_tags_command.go",
			contains: []string{
				"package commands\n", `"example.com/shop/framework/console"`,
				"type PruneTagsCommand struct{}", `return "prune-tags"`,
			},
		},
		"a command with its own signature": {
			args:     []string{"make:command", "PruneTags", "--signature", "tags:prune"},
			path:     "app/console/commands/prune_tags_command.go",
			contains: []string{`return "tags:prune"`},
		},
		"an event": {
			args:     []string{"make:event", "TagCreated"},
			path:     "app/events/tag_created.go",
			contains: []string{"package events\n", "type TagCreated struct{}"},
		},
		"a job with its kind": {
			args: []string{"make:job", "DeliverWebhook"},
			path: "app/jobs/deliver_webhook.go",
			contains: []string{
				"type DeliverWebhookArgs struct{}", `return "deliver_webhook"`,
				"Handle(_ context.Context, _ *queue.Job[DeliverWebhookArgs]) error",
			},
		},
		"a listener for any event": {
			args:     []string{"make:listener", "AuditEverything"},
			path:     "app/listeners/audit_everything.go",
			contains: []string{`"example.com/shop/framework/events"`, "_ events.Event) error"},
		},
		"a listener for one event": {
			args:     []string{"make:listener", "SendWelcomeMail", "--event", "UserRegistered"},
			path:     "app/listeners/send_welcome_mail.go",
			contains: []string{`appevents "example.com/shop/app/events"`, "_ appevents.UserRegistered) error"},
		},
		"a queued listener": {
			args:     []string{"make:listener", "IndexTag", "--event", "TagCreated", "--queued"},
			path:     "app/listeners/index_tag.go",
			contains: []string{`"example.com/shop/framework/queue"`, "Job(_ appevents.TagCreated) queue.Args"},
		},
		"a mailable": {
			args:     []string{"make:mail", "InvitationMail"},
			path:     "app/mail/invitation_mail.go",
			contains: []string{"package mail\n", `Subject: "Invitation Mail"`},
		},
		"a migration creating a table": {
			args:     []string{"make:migration", "create_tags_table"},
			path:     "database/migrations/20261102093015_create_tags_table.sql",
			contains: []string{"CREATE TABLE tags (", "DROP TABLE tags;"},
		},
		"an empty migration": {
			args:     []string{"make:migration", "add_color_to_tags"},
			path:     "database/migrations/20261102093015_add_color_to_tags.sql",
			contains: []string{"-- migrate: phase=expand", "-- +goose Up"},
		},
		"a controller in its folder": {
			args: []string{"make:controller", "api/v1/TagController"},
			path: "app/http/controllers/api/v1/tag_controller.go",
			contains: []string{
				"package v1\n", `"example.com/shop/framework/routing"`, "type TagController struct{}",
				"routing.Endpoint[TagInput, TagOutput] = (*TagController)(nil).Show",
			},
		},
		"a request with tag rules only": {
			args: []string{"make:request", "StoreTag"},
			path: "app/http/requests/store_tag_request.go",
			contains: []string{
				"package requests\n\n/*", "type StoreTagRequest struct {", `validate:"required,max=255"`,
			},
		},
		"a request with its hooks": {
			args: []string{"make:request", "StoreTagRequest", "--authorize", "--validate"},
			path: "app/http/requests/store_tag_request.go",
			contains: []string{
				`"example.com/shop/framework/validation"`, "Authorize(_ context.Context) error",
				"Validate(_ context.Context, _ *validation.Factory) error",
			},
		},
		"a policy for any resource": {
			args:     []string{"make:policy", "LineItemPolicy"},
			path:     "app/policies/line_item_policy.go",
			contains: []string{`"line-items.update", p.Update`, "_ auth.Principal, _ any) (bool, error)"},
		},
		"a policy for a model": {
			args:     []string{"make:policy", "Tag", "--model", "Tag"},
			path:     "app/policies/tag_policy.go",
			contains: []string{`"example.com/shop/app/models"`, "_ auth.Principal, _ *models.Tag) (bool, error)"},
		},
		"a notification": {
			args: []string{"make:notification", "InvoicePaidNotification"},
			path: "app/notifications/invoice_paid_notification.go",
			contains: []string{
				"type InvoicePaidNotification struct{}", `return "invoice_paid"`,
				"ToMail(_ context.Context, _ notifications.Delivery) (mail.Mailable, error)",
			},
		},
		"a feature test": {
			args:     []string{"make:test", "tags/TagCreationTest"},
			path:     "tests/feature/tags/tag_creation_test.go",
			contains: []string{"package tags\n", "func TestTagCreation(t *testing.T) {"},
		},
	}

	for name, tc := range files {
		t.Run("generates "+name, func(t *testing.T) {
			t.Parallel()
			g := generator(t)

			code, stdout, stderr := generate(t, g, tc.args...)

			require.Equal(t, console.ExitSuccess, code, stderr)
			assert.True(t, strings.HasPrefix(stdout, "Created "+tc.path+"\n"), stdout)

			contents := read(t, g, tc.path)
			for _, fragment := range tc.contains {
				assert.Contains(t, contents, fragment)
			}
		})
	}

	t.Run("starts a new package with its doc comment once", func(t *testing.T) {
		t.Parallel()
		g := generator(t)

		_, first, _ := generate(t, g, "make:job", "DeliverWebhook")
		_, second, _ := generate(t, g, "make:job", "SendDigest")

		assert.Equal(t, "Created app/jobs/deliver_webhook.go\nCreated app/jobs/doc.go\n", first)
		assert.Equal(t, "Created app/jobs/send_digest.go\n", second)
		assert.Equal(t, "/*\nPackage jobs holds queued jobs.\n*/\npackage jobs\n", read(t, g, "app/jobs/doc.go"))
	})

	t.Run("generates migrations that pass the migration lint", func(t *testing.T) {
		t.Parallel()
		g := generator(t)
		fake := clock.NewFake(time.Date(2026, 11, 2, 9, 30, 15, 0, time.UTC))
		g.Clock = fake
		generate(t, g, "make:migration", "create_tags_table")
		fake.Travel(time.Second)
		generate(t, g, "make:migration", "add_color_to_tags")

		assert.Empty(t, migrate.Lint(os.DirFS(filepath.Join(g.Root, "database", "migrations"))))
	})

	t.Run("refuses to overwrite a file unless forced", func(t *testing.T) {
		t.Parallel()
		g := generator(t)
		generate(t, g, "make:event", "TagCreated")
		path := filepath.Join(g.Root, "app", "events", "tag_created.go")
		require.NoError(t, os.WriteFile(path, []byte("kept"), 0o600))

		refused, _, stderr := generate(t, g, "make:event", "TagCreated")
		kept := read(t, g, "app/events/tag_created.go")
		forced, _, _ := generate(t, g, "make:event", "TagCreated", "--force")

		assert.Equal(t, console.ExitFailure, refused)
		assert.Contains(t, stderr, "pass --force to replace it")
		assert.Equal(t, "kept", kept)
		assert.Equal(t, console.ExitSuccess, forced)
		assert.Contains(t, read(t, g, "app/events/tag_created.go"), "type TagCreated struct{}")
	})

	t.Run("renders the application's override of a stub", func(t *testing.T) {
		t.Parallel()
		g := generator(t)
		g.Overrides = fstest.MapFS{"event.go.tmpl": {Data: []byte("package {{.Package}}\n\n// {{.Name}}\n")}}

		generate(t, g, "make:event", "TagCreated")

		assert.Equal(t, "package events\n\n// TagCreated\n", read(t, g, "app/events/tag_created.go"))
	})

	t.Run("uses the framework import path it is given", func(t *testing.T) {
		t.Parallel()
		g := generator(t)
		g.Framework = "example.com/framework"

		generate(t, g, "make:job", "DeliverWebhook")

		assert.Contains(t, read(t, g, "app/jobs/deliver_webhook.go"), `"example.com/framework/queue"`)
	})
}

func TestGeneratorFailures(t *testing.T) {
	t.Parallel()

	invalid := map[string][]string{
		"a folder in upper case":            {"make:action", "Tags/CreateTag"},
		"a folder for a flat kind":          {"make:event", "tags/TagCreated"},
		"a type name that is not Go":        {"make:job", "9lives"},
		"a test name that is only a suffix": {"make:test", "tags/9Test"},
		"a migration not in snake case":     {"make:migration", "CreateTags"},
		"an event type that is not Go":      {"make:listener", "Audit", "--event", "tag-created"},
		"a model type that is not Go":       {"make:policy", "Tag", "--model", "tags"},
		"a folder for a request":            {"make:request", "tags/StoreTag"},
	}

	for name, args := range invalid {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()

			code, _, stderr := generate(t, generator(t), args...)

			assert.Equal(t, console.ExitFailure, code)
			assert.Contains(t, stderr, "make: invalid name")
		})
	}

	t.Run("fails when a stub is missing", func(t *testing.T) {
		t.Parallel()
		g := generator(t)
		g.Stubs = fstest.MapFS{}

		code, _, stderr := generate(t, g, "make:event", "TagCreated")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "read stub event.go.tmpl")
	})

	t.Run("fails on a stub that does not parse or render", func(t *testing.T) {
		t.Parallel()
		broken := make.Generator{Root: t.TempDir(), Stubs: fstest.MapFS{
			"event.go.tmpl": {Data: []byte("{{.Name")},
			"job.go.tmpl":   {Data: []byte("{{.Missing}}")},
		}}

		_, _, parse := generate(t, broken, "make:event", "TagCreated")
		_, _, render := generate(t, broken, "make:job", "DeliverWebhook")

		assert.Contains(t, parse, "parse stub event.go.tmpl")
		assert.Contains(t, render, "render stub job.go.tmpl")
	})

	t.Run("fails on a stub it cannot read", func(t *testing.T) {
		t.Parallel()
		g := generator(t)
		g.Overrides = fstest.MapFS{"event.go.tmpl": {Mode: os.ModeDir}}

		code, _, stderr := generate(t, g, "make:event", "TagCreated")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "read stub event.go.tmpl")
	})

	t.Run("fails when the project directory cannot be written", func(t *testing.T) {
		t.Parallel()
		g := generator(t)
		require.NoError(t, os.WriteFile(filepath.Join(g.Root, "app"), []byte("a file"), 0o600))

		code, _, stderr := generate(t, g, "make:event", "TagCreated")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "create directory for app/events/tag_created.go")
	})
}
