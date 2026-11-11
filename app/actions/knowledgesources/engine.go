/*
|--------------------------------------------------------------------------
| Knowledge Source Engine
|--------------------------------------------------------------------------
|
| Everything a source does besides reading its service happens here, the
| same for every driver: a source is created with its settings checked
| and its secrets sealed, a run discovers documents page by page, fetches
| only the ones whose version moved, indexes the ones whose text changed,
| and when complete removes the ones the source no longer has. Runs keep
| their counters, the last fifty are kept, a failing source tells the
| workspace's admins, and an import turns documents into articles. A
| driver never sees the database and the engine never sees an API.
|
*/

package knowledgesources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of the engine: runs kept per source, documents one source may
hold, the largest document kept, and how long a run may go without
progress before it is called stalled.
*/
const (
	keptRuns       = 50
	maxDocuments   = 5000
	maxMarkdown    = 512 << 10
	stalledAfter   = 30 * time.Minute
	importBatch    = 25
	documentsLimit = 100
)

/*
Failures of managing sources.
*/
var (
	ErrSourceNotFound = fault.New(fault.NotFound, "knowledge_source_not_found",
		"No knowledge source of this workspace has this id.")
	ErrDocumentNotFound = fault.New(fault.NotFound, "knowledge_document_not_found",
		"No document of this source has this id.")
	ErrRunNotFound = fault.New(fault.NotFound, "knowledge_sync_run_not_found",
		"No sync of this source has this id.")
	ErrUnknownType = fault.New(fault.Invalid, "knowledge_source_type_invalid",
		"No source of this type is available.").OnField("type")
	ErrModeUnsupported = fault.New(fault.Invalid, "knowledge_source_mode_invalid",
		"This source cannot be used that way.").OnField("mode")
	ErrTargetUnsupported = fault.New(fault.Invalid, "knowledge_source_targets_invalid",
		"This source cannot feed that.").OnField("targets")
	ErrHelpCenterRequired = fault.New(fault.Invalid, "help_center_required",
		"Choose the help center the articles go to.").OnField("help_center_id")
	ErrScheduleUnsupported = fault.New(fault.Invalid, "knowledge_source_schedule_invalid",
		"This source cannot sync on that schedule.").OnField("schedule")
	ErrNothingToImport = fault.New(fault.Conflict, "knowledge_source_nothing_to_import",
		"Sync this source before turning it into articles.")
)

/*
Engine runs knowledge sources. DB holds them, Clock dates them, Queue
runs their jobs and Events tells the rest of the product. Keys seal
credentials, Drivers reads sources and Vectors indexes documents.
Articles and Collections write imports, Images keeps their pictures and
HTTP downloads them. Mail and Links tell admins about failing syncs,
and Uploads holds uploaded archives. Connections hands out tokens of
connected services and IDs reads their public ids. BrowserHourMicros
prices an hour of a hosted crawling browser.
*/
type Engine struct {
	DB          *database.DB
	Clock       clock.Clock
	Queue       *queue.Queue
	Events      *events.Dispatcher
	Keys        encryption.KeyManager
	Drivers     *sources.Registry
	Vectors     *knowledge.Vectors
	Articles    *helpcenters.Articles
	Collections *helpcenters.Collections
	Images      *helpcenters.Images
	HTTP        *httpclient.Client
	Mail        *mail.Manager
	Links       auth.Links
	Uploads     filesystem.Filesystem
	Connections Connections
	IDs         *ids.Registry

	BrowserHourMicros int64
}

/*
inWorkspace runs fn in a transaction scoped to the workspace.
*/
func (e *Engine) inWorkspace(ctx context.Context, ws ids.UUID,
	fn func(context.Context, *database.Tx, *models.Queries) error,
) error {
	return e.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, tx, models.New(tx))
	})
}

/*
driverOf returns the driver of a source's type.
*/
func (e *Engine) driverOf(kind string) (sources.Driver, error) {
	d, ok := e.Drivers.Driver(kind)
	if !ok {
		return nil, ErrUnknownType
	}

	return d, nil
}

/*
settingsOf returns a source's settings with its secrets, for its driver.
*/
func (e *Engine) settingsOf(ctx context.Context, s models.KnowledgeSource) (sources.Config, error) {
	secrets, err := e.openSecrets(ctx, s)
	if err != nil {
		return sources.Config{}, err
	}

	merged := map[string]any{}
	if len(s.Config) > 0 {
		if err := json.Unmarshal(s.Config, &merged); err != nil {
			return sources.Config{}, fmt.Errorf("read source settings: %w", err)
		}
	}

	for k, v := range secrets {
		merged[k] = v
	}

	if d, ok := e.Drivers.Driver(s.Type); ok {
		if err := e.connect(ctx, s.WorkspaceID, d, merged); err != nil {
			return sources.Config{}, err
		}
	}

	raw, err := json.Marshal(merged)
	if err != nil {
		return sources.Config{}, fmt.Errorf("write source settings: %w", err)
	}

	return sources.Config{JSON: raw, Include: s.Include, Exclude: s.Exclude}, nil
}

/*
nextSync is when a schedule runs next after the moment; manual never.
*/
func nextSync(schedule string, after time.Time) *time.Time {
	var next time.Time

	switch schedule {
	case "hourly":
		next = after.Add(time.Hour)
	case "daily":
		next = after.Add(24 * time.Hour)
	case "weekly":
		next = after.Add(7 * 24 * time.Hour)
	default:
		return nil
	}

	return &next
}

/*
wrap wraps an error with what was attempted, nil staying nil.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}
