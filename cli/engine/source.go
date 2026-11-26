/*
Package engine moves records from a source tool into dimmi: it reads
each kind in dependency order, downloads the files records point at,
uploads them, sends records in batches and keeps a checkpoint after
every page, so a run that stops resumes where it left off and a sync
reads only what changed.

The engine knows nothing about any one tool. A Source reads a tool and
translates its objects into the import format; a Destination is dimmi's
migrations API.
*/
package engine

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dimmihq/dimmi/cli/format"
)

/*
Account is the source workspace being migrated.
*/
type Account struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region,omitempty"`
}

/*
KindCount is how many records of a kind the source holds. Estimate is
set when the source can only guess.
*/
type KindCount struct {
	Kind     format.Kind `json:"kind"`
	Count    int         `json:"count"`
	Estimate bool        `json:"estimate,omitempty"`
	Notes    []string    `json:"notes,omitempty"`
}

/*
Inventory is what a source holds and what will not come across.
Missing lists permissions the source's key lacks.
*/
type Inventory struct {
	Account     Account     `json:"account"`
	Kinds       []KindCount `json:"kinds"`
	Missing     []string    `json:"missing_permissions,omitempty"`
	NotImported []string    `json:"not_imported,omitempty"`
	Warnings    []string    `json:"warnings,omitempty"`
}

/*
Count returns the inventory's count for the kind.
*/
func (inv Inventory) Count(kind format.Kind) (KindCount, bool) {
	for _, k := range inv.Kinds {
		if k.Kind == kind {
			return k, true
		}
	}

	return KindCount{}, false
}

/*
ReadOptions narrows a read. Since asks only for what changed after it;
a source that can't filter reads everything and the engine skips what
didn't change. Cursor resumes a read where a checkpoint left it.
*/
type ReadOptions struct {
	Since  time.Time
	Cursor string
}

/*
Page is a batch of records the source read, with the cursor that
resumes after it. An empty cursor on the last page ends the read.
*/
type Page struct {
	Records []format.Record
	Cursor  string
}

/*
Source reads one tool and translates it into the import format.
Kinds lists what it reads, in import order. Read calls emit for each
page; when emit returns an error, Read stops and returns it. Open
downloads a file a record points at.
*/
type Source interface {
	Name() string
	Account(ctx context.Context) (Account, error)
	Discover(ctx context.Context) (Inventory, error)
	Kinds() []format.Kind
	Read(ctx context.Context, kind format.Kind, opts ReadOptions, emit func(Page) error) error
	Open(ctx context.Context, file *format.File) (io.ReadCloser, error)
}

/*
Counts is how many records of a kind dimmi took.
*/
type Counts struct {
	Deferred  int `json:"deferred"`
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Failed    int `json:"failed"`
}

/*
Total is every record dimmi holds for the kind.
*/
func (c Counts) Total() int { return c.Created + c.Updated + c.Unchanged }

/*
Migration is a migration as dimmi reports it.
*/
type Migration struct {
	ID     string                 `json:"id"`
	Status string                 `json:"status"`
	Counts map[format.Kind]Counts `json:"counts"`
}

/*
RecordError is why dimmi refused a record. Messages never hold the
record's content.
*/
type RecordError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

/*
The outcomes of one record.
*/
const (
	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
	StatusDeferred  = "deferred"
	StatusFailed    = "failed"
)

/*
Result is one record's outcome.
*/
type Result struct {
	SourceID string       `json:"source_id"`
	Status   string       `json:"status"`
	ID       string       `json:"id,omitempty"`
	Error    *RecordError `json:"error,omitempty"`
}

/*
Start names the migration to start or resume.
*/
type Start struct {
	Source        string `json:"source"`
	SourceAccount string `json:"source_account"`
	Name          string `json:"name"`
}

/*
Upload is a file to upload.
*/
type Upload struct {
	SourceID    string
	Name        string
	ContentType string
	Size        int64
	Body        io.Reader
}

/*
ErrNoLookup is what a Destination returns when it can't look records
up; verify then compares counts only.
*/
var ErrNoLookup = errors.New("lookups not supported")

type Destination interface {
	Start(ctx context.Context, start Start) (Migration, error)
	Get(ctx context.Context, id string) (Migration, error)
	Upsert(ctx context.Context, id string, kind format.Kind, records []format.Record) ([]Result, error)
	Upload(ctx context.Context, id string, file Upload) (string, error)
	Lookup(ctx context.Context, id string, kind format.Kind, sourceIDs []string) (map[string]bool, error)
	Rollback(ctx context.Context, id string) (Migration, error)
}
