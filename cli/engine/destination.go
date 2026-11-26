package engine

import (
	"context"
	"errors"
	"io"

	"github.com/dimmihq/dimmi/cli/format"
)

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

/*
Destination is where records go: dimmi's migrations API.
*/
type Destination interface {
	Migrations
	Records
}

/*
Migrations starts, reads and undoes migrations.
*/
type Migrations interface {
	/* Start starts the migration, or returns the one already running. */
	Start(ctx context.Context, start Start) (Migration, error)
	/* Get reads a migration and its counts. */
	Get(ctx context.Context, id string) (Migration, error)
	/* Rollback undoes everything the migration created. */
	Rollback(ctx context.Context, id string) (Migration, error)
}

/*
Records sends a migration's records and files, and looks records up.
*/
type Records interface {
	/* Upsert sends a batch of one kind's records. */
	Upsert(ctx context.Context, id string, kind format.Kind, records []format.Record) ([]Result, error)
	/* Upload uploads a file and returns the id dimmi gave it. */
	Upload(ctx context.Context, id string, file Upload) (string, error)
	/* Lookup reports which of the source ids dimmi holds. */
	Lookup(ctx context.Context, id string, kind format.Kind, sourceIDs []string) (map[string]bool, error)
}
