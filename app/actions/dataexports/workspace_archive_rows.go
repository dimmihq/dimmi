/*
|--------------------------------------------------------------------------
| Workspace Archive Rows and Files
|--------------------------------------------------------------------------
|
| A page of a table is one statement: the rows of this workspace after
| the last one written, each as JSON without the columns that never
| leave. The workspace is named in the statement as well as by row
| level security, so a mistake in either still shows no other
| workspace's rows. Table and column names come only from the list of
| tables in the code, never from a request.
|
| A stored file is copied into the archive under attachments/, named by
| its table and id, and the row that owns it says where in
| archive_path. A file the malware scan did not clear, or one missing
| from the disk, is left out and listed in the manifest.
|
*/

package dataexports

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
)

/*
statement gathers a statement's arguments as it is written.
*/
type statement struct {
	args []any
}

/*
arg adds an argument and returns its placeholder cast to the type.
*/
func (s *statement) arg(value any, cast string) string {
	s.args = append(s.args, value)

	return "$" + strconv.Itoa(len(s.args)) + "::" + cast
}

/*
keyColumn splits a key into its column and type, a uuid unless said.
*/
func keyColumn(key string) (string, string) {
	column, kind, found := strings.Cut(key, ":")
	if !found {
		return column, "uuid"
	}

	return column, kind
}

/*
archivePath is the expression of where a row's file sits in the
archive: attachments/<table>/<id> with the file's extension.
*/
func (f archiveFiles) archivePath() string {
	return `'attachments/` + f.Table + `/' || t.id || coalesce(lower(substring(` + f.Ext +
		` from '\.[A-Za-z0-9]{1,8}$')), '')`
}

/*
tableRead is where the reading of one table stands: the columns left
out, the files that were missing and the key of the last row written.
*/
type tableRead struct {
	table   ArchiveTable
	files   *archiveFiles
	omit    []string
	missing []string
	last    []string
}

/*
query writes the statement of the next page.
*/
func (r *tableRead) query(workspace string) (string, []any) {
	var (
		s       statement
		columns = make([]string, 0, len(r.table.Key))
		casts   = make([]string, 0, len(r.table.Key))
	)

	scope := r.table.Scope
	if scope == "" {
		scope = "workspace_id"
	}

	where := "t." + scope + " = " + s.arg(workspace, "uuid")
	row := "to_jsonb(t) - " + s.arg(append([]string{}, r.omit...), "text[]")

	if r.table.Extra != "" {
		row += " || coalesce(" + r.table.Extra + ", '{}'::jsonb)"
	}

	if r.files != nil {
		row += ` || jsonb_build_object('archive_path', CASE WHEN (` + r.files.Path + `) IS NULL OR (` +
			r.files.Blocked + `) IS NOT NULL OR t.id::text = ANY(` + s.arg(append([]string{}, r.missing...), "text[]") +
			`) THEN NULL ELSE ` + r.files.archivePath() + ` END)`
	}

	for i, key := range r.table.Key {
		column, kind := keyColumn(key)
		columns = append(columns, "t."+column)

		if len(r.last) > 0 {
			casts = append(casts, s.arg(r.last[i], kind))
		}
	}

	sql := "SELECT (" + row + ")::text"
	if len(columns) > 0 {
		sql += ", " + strings.Join(columns, "::text, ") + "::text"
	}

	sql += " FROM " + r.table.Name + " t WHERE " + where

	if len(casts) > 0 {
		sql += " AND (" + strings.Join(columns, ", ") + ") > (" + strings.Join(casts, ", ") + ")"
	}

	if len(columns) > 0 {
		sql += " ORDER BY " + strings.Join(columns, ", ") + " LIMIT " + strconv.Itoa(archivePage)
	}

	return sql, s.args
}

/*
page writes the next page of rows as JSON lines and returns how many it
wrote, remembering the last row's key.
*/
func (r *tableRead) page(ctx context.Context, tx *database.Tx, workspace string, out io.Writer) (int, error) {
	sql, args := r.query(workspace)

	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("read rows: %w", err)
	}

	defer rows.Close()

	var (
		line []byte
		key  = make([]string, len(r.table.Key))
		dest = []any{&line}
		n    int
	)

	for i := range key {
		dest = append(dest, &key[i])
	}

	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return n, fmt.Errorf("read row: %w", err)
		}

		if _, err := out.Write(append(line, '\n')); err != nil {
			return n, fmt.Errorf("write row: %w", err)
		}

		n++
	}

	if n > 0 {
		r.last = key
	}

	return n, rows.Err()
}

/*
storedFile is one file a row names: where it is kept, where it goes in
the archive and, when it may not leave, why.
*/
type storedFile struct {
	id      string
	disk    string
	path    string
	archive string
	blocked *string
}

/*
writeFiles copies the table's stored files into the archive and returns
the ids of rows whose file was missing from its disk.
*/
func (w *archiver) writeFiles(ctx context.Context, files archiveFiles) ([]string, error) {
	var (
		missing []string
		last    string
	)

	for {
		var page []storedFile

		err := w.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			var err error

			page, err = listFiles(ctx, tx, fileRead{files: files, workspace: w.ws.ID.String(), last: last})

			return err
		})
		if err != nil {
			return nil, err
		}

		for _, file := range page {
			reason, err := w.copyFile(ctx, file)
			if err != nil {
				return nil, err
			}

			last = file.id

			if w.noteFile(files.Table, file, reason) {
				missing = append(missing, file.id)
			}
		}

		if len(page) < archivePage {
			return missing, nil
		}
	}
}

/*
noteFile counts a copied file, or lists one left out in the manifest.
It reports whether the file was missing from its disk.
*/
func (w *archiver) noteFile(table string, file storedFile, reason string) bool {
	if reason == "" {
		w.manifest.Attachments++

		return false
	}

	w.manifest.SkippedFiles = append(w.manifest.SkippedFiles, skippedFile{Table: table, ID: file.id, Reason: reason})

	return file.blocked == nil
}

/*
fileRead names the page of a table's files to read: the workspace and
the id of the last file read.
*/
type fileRead struct {
	files     archiveFiles
	workspace string
	last      string
}

/*
listFiles reads the next page of the table's rows that name a file.
*/
func listFiles(ctx context.Context, tx *database.Tx, read fileRead) ([]storedFile, error) {
	var s statement

	files, workspace, last := read.files, read.workspace, read.last

	sql := "SELECT t.id::text, coalesce(" + files.Disk + ", ''), " + files.Path + ", " + files.archivePath() +
		", " + files.Blocked + " FROM " + files.Table + " t WHERE t.workspace_id = " + s.arg(workspace, "uuid") +
		" AND (" + files.Path + ") IS NOT NULL"

	if last != "" {
		sql += " AND t.id > " + s.arg(last, "uuid")
	}

	sql += " ORDER BY t.id LIMIT " + strconv.Itoa(archivePage)

	rows, err := tx.Query(ctx, sql, s.args...)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	defer rows.Close()

	var out []storedFile

	for rows.Next() {
		var f storedFile
		if err := rows.Scan(&f.id, &f.disk, &f.path, &f.archive, &f.blocked); err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}

		out = append(out, f)
	}

	return out, rows.Err()
}

/*
copyFile streams one stored file into the archive. It returns why the
file was left out, empty when it was copied.
*/
func (w *archiver) copyFile(ctx context.Context, file storedFile) (string, error) {
	if file.blocked != nil {
		return *file.blocked, nil
	}

	name := file.disk
	if name == "" {
		name = w.disk
	}

	disk, err := w.storage.Disk(name)
	if err != nil {
		return "", fmt.Errorf("open disk %s: %w", name, err)
	}

	in, err := disk.Open(ctx, file.path)
	if errors.Is(err, filesystem.ErrNotFound) {
		return "missing from storage", nil
	}

	if err != nil {
		return "", fmt.Errorf("open %s: %w", file.archive, err)
	}

	out, err := w.zip.CreateHeader(&zip.FileHeader{Name: file.archive, Method: zip.Store})
	if err != nil {
		return "", errors.Join(fmt.Errorf("add %s: %w", file.archive, err), in.Close())
	}

	if _, err := io.Copy(out, in); err != nil {
		return "", errors.Join(fmt.Errorf("copy %s: %w", file.archive, err), in.Close())
	}

	if err := in.Close(); err != nil {
		return "", fmt.Errorf("close %s: %w", file.archive, err)
	}

	return "", nil
}
