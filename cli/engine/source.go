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
	Describer
	Reader
}

/*
Describer tells which tool and workspace a source reads, and what the
workspace holds.
*/
type Describer interface {
	/* Name is the tool's name, such as "intercom". */
	Name() string
	/* Account reads the workspace the source's key belongs to. */
	Account(ctx context.Context) (Account, error)
	/* Discover counts what the workspace holds. */
	Discover(ctx context.Context) (Inventory, error)
}

/*
Reader reads a tool's records and the files they point at.
*/
type Reader interface {
	/* Kinds lists the kinds the source reads, in import order. */
	Kinds() []format.Kind
	/* Read emits the kind's records page by page. */
	Read(ctx context.Context, kind format.Kind, opts ReadOptions, emit func(Page) error) error
	/* Open downloads a file a record points at. */
	Open(ctx context.Context, file *format.File) (io.ReadCloser, error)
}
