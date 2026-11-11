/*
|--------------------------------------------------------------------------
| Knowledge Source Drivers
|--------------------------------------------------------------------------
|
| A source is anything dimmi can read documents from. Each kind is one
| small driver behind this interface, so a contributor adds a source in
| one file: it says what it is and which settings it takes, checks that
| it can connect, lists the documents it sees page by page, and reads
| one document as Markdown. Discovery, change detection, retries,
| scheduling, indexing and turning documents into articles all belong to
| the engine, so every driver gets them for free and behaves the same.
|
| A driver holds no state about a workspace: everything it needs arrives
| in the config, secrets included, on every call.
|
*/

package sources

import (
	"context"
	"encoding/json"
	"fmt"
)

/*
Mode is what a source's documents become: sync keeps a read-only copy
that is refreshed, import turns them once into editable articles.
*/
type Mode string

/*
Target is who a source's documents feed: the agent answering customers,
the Operator helping teammates, or the help center as articles.
*/
type Target string

/*
AuthKind is how a driver connects: not at all, or through an OAuth
installation of the service. Keys are never pasted into a form.
*/
type AuthKind string

/*
The modes, targets and ways to connect.
*/
const (
	ModeSync   Mode = "sync"
	ModeImport Mode = "import"

	TargetAgent      Target = "agent"
	TargetOperator   Target = "operator"
	TargetHelpCenter Target = "help_center"

	AuthNone  AuthKind = "none"
	AuthOAuth AuthKind = "oauth"
)

/*
Description is what a driver says about itself. Type is its registry
value, lowercase with underscores; Name and Summary how people see it,
and Logo the key of its picture in the dashboard. Schema is the JSON
Schema of its settings: the dashboard draws the connect form from it,
and properties marked "x-secret" are sealed and never shown again.
Modes and Targets say what it supports, Schedules which sync schedules
make sense for it, the first being the default. Integration names the
connected service, such as notion, a driver reads through: the engine
then adds a working "access_token" and the connected "account" to its
settings on every call, and the settings name the connection in an
"installation" property of format x-integration.
*/
type Description struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Summary     string          `json:"summary"`
	Logo        string          `json:"logo"`
	Auth        AuthKind        `json:"auth"`
	Schema      json.RawMessage `json:"schema"`
	Modes       []Mode          `json:"modes"`
	Targets     []Target        `json:"targets"`
	Schedules   []string        `json:"schedules"`
	Integration string          `json:"integration,omitempty"`
}

/*
Config is a source's settings, secrets included, as the JSON object its
schema describes. Include and Exclude are the member's path patterns;
the engine applies them anyway, and a driver that can filter at the
source, such as a crawl service, may pass them on to save work.
*/
type Config struct {
	JSON    json.RawMessage
	Include []string
	Exclude []string
}

/*
Decode reads the settings into v.
*/
func (c Config) Decode(v any) error {
	if len(c.JSON) == 0 {
		return nil
	}

	if err := json.Unmarshal(c.JSON, v); err != nil {
		return &ConfigError{Message: fmt.Sprintf("The settings could not be read: %v.", err)}
	}

	return nil
}

/*
Driver reads one kind of source.
*/
type Driver interface {
	/*
		Describe says what the driver is and which settings it takes.
	*/
	Describe() Description

	/*
		Check connects with the settings and reports the account,
		failing with a ConfigError, ErrUnauthorized or another error.
	*/
	Check(ctx context.Context, cfg Config) (CheckResult, error)

	/*
		List returns one page of the documents the source has, from
		the cursor; an empty cursor is the first page.
	*/
	List(ctx context.Context, cfg Config, cursor string) (Page, error)

	/*
		Fetch reads one listed document. ErrNotFound says it is gone,
		an ExcludedError that it should not be indexed.
	*/
	Fetch(ctx context.Context, cfg Config, ref Ref) (Document, error)
}

/*
Capped is a driver whose settings bound how many documents one source
may hold, such as a crawl's page limit.
*/
type Capped interface {
	/*
		MaxDocuments is the most documents the settings allow.
	*/
	MaxDocuments(cfg Config) int
}
