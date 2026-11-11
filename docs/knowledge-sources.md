# Adding a knowledge source

A knowledge source is anything dimmi reads documents from: a website, a repository, a help center. Each kind is one
driver package under `app/integrations/knowledge/sources/<name>`, one line in `app/providers/knowledge_sources.go`,
and one recorded fixture with a short test.

The driver only talks to the source. The engine in `app/actions/knowledgesources` does everything else: discovery,
include and exclude patterns, change detection, retries, scheduling, indexing, the dashboard and turning documents
into help center articles. A driver keeps no state about a workspace; its settings, secrets included, arrive on
every call.

## The interface

`sources.Driver` (`driver.go`) has four methods.

| Method | Does |
|---|---|
| `Describe() Description` | Says what the driver is: `Type`, `Name`, `Summary`, `Logo`, `Auth`, `Schema`, `Modes`, `Targets`, `Schedules` |
| `Check(ctx, cfg) (CheckResult, error)` | Connects with the settings and returns the `Account` it reached and, when cheap, an `Estimate` of documents |
| `List(ctx, cfg, cursor) (Page, error)` | Returns one page of `Ref`s from the cursor, and the `Next` cursor, empty at the end |
| `Fetch(ctx, cfg, ref) (Document, error)` | Reads one listed document as Markdown |

- `Type` is lowercase with underscores (`^[a-z][a-z0-9_]{1,39}$`) and never changes once shipped.
- `Modes` is `ModeSync` (a read-only copy kept fresh) and/or `ModeImport` (editable articles, once).
- `Targets` is any of `TargetAgent`, `TargetOperator`, `TargetHelpCenter`.
- `Schedules` lists the sync schedules that make sense, the default first, such as `daily`, `hourly`, `weekly`,
  `manual`.
- `Logo` is a key in `resources/js/features/knowledge/looks.ts`. A key with no entry gets a plain mark, so a new
  driver needs no React.

Read the settings with `cfg.Decode(&settings)`. It already returns a `ConfigError` on bad JSON. Check required
fields yourself and return `&sources.ConfigError{Field: "repository", Message: "Enter the repository as owner/name."}`;
the dashboard shows the message under that field.

### Refs and documents

A `Ref` (`listing.go`) names one document:

- `ExternalID` names it for the driver and never changes.
- `Path` is what include and exclude patterns match, such as `/docs/billing` or `guides/setup.md`.
- `Version` changes whenever the document does: an update time, a blob hash, an ETag. Unchanged versions are not
  fetched again. An empty version always fetches.
- `Document`, when set, is stored as is and `Fetch` is skipped. Use it when the listing already holds the full
  content, as a crawl service or a bulk export does.

A `Document` (`document.go`) holds `Title`, `Markdown`, `URL`, `Path`, `Locale`, `Collections` (outermost first),
`Assets` (pictures by address), `Visibility` (`VisibilityPublic` or `VisibilityInternal`), `Draft`,
`TranslationOf`, and `Links` for drivers that crawl. A `Version` set here replaces the listing's.

Helpers in the package do the common work:

- `HTMLToMarkdown`, `Images`, `Tidy` and `MarkdownFile` (front matter for title and draft) in `markdown.go`.
- `Folders` and `Humanize` turn a path into collections and a readable title.
- `Match` and `Allowed` in `glob.go` apply `*` and `**` patterns, if the driver can filter early.

### Optional: Capped

A driver whose settings bound how many documents a source holds, such as a crawl's page limit, also implements
`sources.Capped`:

```go
func (d *Driver) MaxDocuments(cfg sources.Config) int
```

The engine stops discovering past that number.

## Settings as JSON Schema

`Describe().Schema` is a JSON Schema object. The dashboard draws the connect form from it and marks required
fields; the driver's `Check` is what rejects bad values. Every property needs a `title` and a `type`; every
`required` name must be described.

| Keyword | Effect |
|---|---|
| `title`, `description` | Field label and hint |
| `default` | Initial value |
| `type: "boolean"` | A switch |
| `type: "integer"` | A number field |
| `type: "array"` of strings | A textarea, one entry per line |
| `enum` | A picker |
| `x-enum-labels` | Labels for the `enum` values, in the same order |
| `x-placeholder` | Placeholder text of a text field |
| `x-secret: true` | Sealed at rest, never sent back. Never for a connected service's key: connect it with OAuth |
| `format: "x-upload"` | A file picker; the field holds the stored file's key |
| `format: "x-integration"` | The connected account; set `Description.Integration` to the OAuth provider. The engine hands the driver `access_token` and `account` |
| `format: "x-choice"`, `x-depends` | Picked from a list the driver returns from `Options` (implement `sources.Chooser`), read again when a setting in `x-depends` changes |
| `format: "x-tree"` | Ticked items stored as `selected`; implement `sources.Browser` and filter `List` with `sources.Chosen` |

A connected service never asks for a pasted key or a typed id: the account is an OAuth installation, and
everything else (a repository, a branch, pages) is picked from what the service returns.

```json
{
  "type": "object",
  "required": ["installation", "project"],
  "properties": {
    "installation": {"type": "string", "format": "x-integration", "title": "Acme account"},
    "project": {"type": "string", "format": "x-choice", "title": "Project", "x-depends": ["installation"]},
    "selected": {"type": "array", "items": {"type": "string"}, "format": "x-tree", "title": "Pages"}
  }
}
```

## Errors

Return the shared errors from `errors.go` so every source behaves the same.

| Error | Engine does |
|---|---|
| `ErrUnauthorized` | Stops the sync, marks the source failing and emails the admins |
| `ErrNotFound` | From `Fetch`: removes the document. From `Check`: the source is gone |
| `ErrUnavailable` | Retries later |
| `*RateLimitError{RetryAfter}` | Waits that long, without using up an attempt |
| `*WaitError{After}` | The source is still working, such as a crawl job; asks again after the wait, not a failure |
| `*ExcludedError` via `sources.Excluded(reason)` | Keeps the document out, showing the reason |
| `*ConfigError{Field, Message}` | Shown on the connect form |

Exclusion reasons: `robots`, `pattern`, `js_rendered`, `duplicate`, `too_large`, `unsupported_type`, `empty`,
`draft`.

For an HTTP API, `sources.FromResponse(resp, "what was asked")` maps the status: 401 and 403 to `ErrUnauthorized`,
404 and 410 to `ErrNotFound`, 429 to a `RateLimitError` from `Retry-After`, 5xx to `ErrUnavailable`.

## Dependencies

Drivers live in the app zone of the depguard rules in `.golangci.yml`: the standard library and
`github.com/dimmihq/dimmi/...` only. No vendor SDKs.
Reach the service through `framework/httpclient` (the `*httpclient.Client`
the provider passes in), which also refuses private addresses, caps bodies and follows the redirect rules. If a
library is truly needed, wrap it in `framework/` first.

## A driver in one file

Copy `app/integrations/knowledge/sources/github/github.go`. The shape is:

```go
type Driver struct {
	HTTP *httpclient.Client
}

func New(client *httpclient.Client) *Driver {
	return &Driver{HTTP: client}
}

func (d *Driver) Describe() sources.Description { ... }
func (d *Driver) Check(ctx context.Context, cfg sources.Config) (sources.CheckResult, error) { ... }
func (d *Driver) List(ctx context.Context, cfg sources.Config, cursor string) (sources.Page, error) { ... }
func (d *Driver) Fetch(ctx context.Context, cfg sources.Config, ref sources.Ref) (sources.Document, error) { ... }
```

Add a `doc.go` with the package comment, and the file banner and docblocks the repository's comment rules ask for.

## Register it

Add one line to `sourceDrivers` in `app/providers/knowledge_sources.go`. The order there is the order of the
dashboard's gallery.

```go
drivers := []sources.Driver{
	siteDriver(a),
	intercom.New(a.HTTP),
	...
	acme.New(a.HTTP),
}
```

Registration fails at boot on a bad type, a duplicate type, or a description missing a name, schema, modes or
targets.

## Fixtures and the conformance test

Driver tests replay recorded answers of the real API; they never reach the network. Add
`tests/unit/knowledgesources/<name>_test.go`:

```go
func acmeCase() sourcetest.Case {
	return sourcetest.Case{
		New:      func(c *httpclient.Client) sources.Driver { return acme.New(c) },
		Config:   `{"site":"demo"}`,
		Fixture:  "acme.json",
		MaxFetch: 3,
		Scrub:    map[string]string{"Jane Realname": "Ada Lovelace"},
	}
}

func TestAcme(t *testing.T) {
	t.Parallel()

	sourcetest.Run(t, acmeCase())
}

func TestRecordAcme(t *testing.T) {
	sourcetest.Record(t, acmeCase(), "ACME_SOURCE_CONFIG", "../../fixtures/sources")
}
```

`sourcetest.Run` checks the contract:

- the description is complete and its schema well formed, every field with a title and a type;
- `Check` connects and names an account;
- the listing pages to the end, every `ExternalID` listed once, the cursor always moving;
- up to `MaxFetch` documents read with a title, a body, no `<script`, and a known visibility;
- empty settings fail with a `ConfigError`.

### Recording

Record once against an account filled only with dummy content:

```bash
DIMMI_RECORD_SOURCES=1 ACME_SOURCE_CONFIG='{"project":"demo","access_token":"..."}' \
  go test ./tests/unit/knowledgesources -run TestRecordAcme
```

- Without `DIMMI_RECORD_SOURCES=1` the record test skips, so CI never reaches the service.
- Settings come from the environment variable, never a file. Requests that carried them are not recorded; only
  answers are.
- Every email address in the answers becomes `person@example.test`. Add each real name or other literal to
  `Scrub`.
- The file lands in `tests/fixtures/sources/acme.json`, embedded by `tests/fixtures/sources`. Read it before
  committing: the repository is public.

Edge cases that the contract does not cover, such as a 429 or an excluded draft, go in the same test file as extra
subtests with `httpclienttest` answers.
