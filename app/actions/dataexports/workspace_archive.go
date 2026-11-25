/*
|--------------------------------------------------------------------------
| Workspace Archive
|--------------------------------------------------------------------------
|
| A workspace export is one zip: a file of JSON lines per table, the
| stored files under attachments/, and a manifest saying what each file
| holds. Nothing is held in memory: rows are read a page at a time, each
| page in its own short transaction inside the workspace, and written
| straight into the zip, which streams to the disk as it grows. A page
| starts after the last row of the one before, so a table of any size
| costs the same per page.
|
*/

package dataexports

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/models"
)

/*
archiveName is the file a workspace export downloads as, archiveVersion
the version of its layout and archivePage how many rows one page reads.
*/
const (
	archiveName    = "workspace-export.zip"
	archiveVersion = "1"
	archivePage    = 1000
)

/*
archiveEntry describes one file of JSON lines in the manifest.
*/
type archiveEntry struct {
	Name        string `json:"name"`
	Records     int    `json:"records"`
	Description string `json:"description"`
}

/*
skippedFile is a stored file the archive could not carry, and why.
*/
type skippedFile struct {
	Table  string `json:"table"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

/*
archiveManifest is manifest.json: whose data the archive holds, when it
was written and what is in it.
*/
type archiveManifest struct {
	FormatVersion string `json:"format_version"`
	Workspace     struct {
		ID       string `json:"id"`
		PublicID string `json:"public_id"`
		Name     string `json:"name"`
	} `json:"workspace"`
	GeneratedAt  time.Time      `json:"generated_at"`
	Files        []archiveEntry `json:"files"`
	Attachments  int            `json:"attachments"`
	SkippedFiles []skippedFile  `json:"skipped_files"`
}

/*
archiver writes one workspace's archive. Dropped holds, per table, the
columns whose type never leaves.
*/
type archiver struct {
	db       *database.DB
	storage  *filesystem.Manager
	disk     string
	ws       models.Workspace
	zip      *zip.Writer
	dropped  map[string][]string
	manifest archiveManifest
	records  int
}

/*
write writes every table, its files first, then the manifest.
*/
func (w *archiver) write(ctx context.Context, now time.Time) error {
	if err := w.loadDropped(ctx); err != nil {
		return err
	}

	w.manifest.FormatVersion = archiveVersion
	w.manifest.Workspace.ID = w.ws.ID.String()
	w.manifest.Workspace.PublicID = models.PublicID(models.PrefixWorkspace, w.ws.ID)
	w.manifest.Workspace.Name = w.ws.Name
	w.manifest.GeneratedAt = now.UTC()
	w.manifest.SkippedFiles = []skippedFile{}

	files := archiveFileTables()

	for _, table := range ArchiveTables() {
		read := tableRead{table: table}

		if i := slices.IndexFunc(files, func(f archiveFiles) bool { return f.Table == table.Name }); i >= 0 {
			read.files = &files[i]

			missing, err := w.writeFiles(ctx, files[i])
			if err != nil {
				return fmt.Errorf("write files of %s: %w", table.Name, err)
			}

			read.missing = missing
		}

		if err := w.writeTable(ctx, read); err != nil {
			return fmt.Errorf("write %s: %w", table.Name, err)
		}
	}

	return w.writeManifest()
}

/*
loadDropped finds the columns of bytes, search vectors and embeddings,
which no table writes.
*/
func (w *archiver) loadDropped(ctx context.Context) error {
	w.dropped = map[string][]string{}

	return w.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT table_name::text, column_name::text FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND udt_name IN ('bytea', 'tsvector', 'vector', 'halfvec', 'sparsevec')`)
		if err != nil {
			return fmt.Errorf("list dropped columns: %w", err)
		}

		defer rows.Close()

		for rows.Next() {
			var table, column string
			if err := rows.Scan(&table, &column); err != nil {
				return fmt.Errorf("read dropped column: %w", err)
			}

			w.dropped[table] = append(w.dropped[table], column)
		}

		return rows.Err()
	})
}

/*
writeTable writes the table's rows as one file of JSON lines, page
after page, and notes it in the manifest.
*/
func (w *archiver) writeTable(ctx context.Context, read tableRead) error {
	name := read.table.Name + ".ndjson"

	out, err := w.zip.Create(name)
	if err != nil {
		return fmt.Errorf("add %s: %w", name, err)
	}

	read.omit = slices.Concat(read.table.Omit, w.dropped[read.table.Name])
	total := 0

	for {
		var n int

		err := w.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			var err error

			n, err = read.page(ctx, tx, w.ws.ID.String(), out)

			return err
		})
		if err != nil {
			return err
		}

		total += n

		if n < archivePage || len(read.table.Key) == 0 {
			break
		}
	}

	w.records += total
	w.manifest.Files = append(w.manifest.Files, archiveEntry{
		Name: name, Records: total, Description: read.table.About,
	})

	return nil
}

/*
writeManifest writes manifest.json, last, once every count is known.
*/
func (w *archiver) writeManifest() error {
	out, err := w.zip.Create("manifest.json")
	if err != nil {
		return fmt.Errorf("add manifest: %w", err)
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")

	if err := enc.Encode(w.manifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

/*
countingWriter counts the bytes written through it.
*/
type countingWriter struct {
	to io.Writer
	n  int64
}

/*
Write writes the bytes and counts them.
*/
func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.to.Write(p)
	c.n += int64(n)

	return n, err
}
