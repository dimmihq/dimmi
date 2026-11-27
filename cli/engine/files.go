package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/dimmihq/dimmi/cli/format"
)

/*
files uploads every file the records name that dimmi doesn't have yet,
each once. A file that can't be read or is too big is reported and left
out; the record still goes, and dimmi shows the attachment as missing.
*/
func (e *Engine) files(ctx context.Context, run kindRun, todo []pending) error {
	st := run.st

	if st.Files == nil {
		st.Files = map[string]string{}
	}

	wanted := wantedFiles(todo, st.Files)
	if len(wanted) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.Workers)
	fr := &fileRun{kindRun: run, cancel: cancel}

	for _, f := range wanted {
		wg.Add(1)
		sem <- struct{}{}

		go func(f *format.File) {
			defer wg.Done()
			defer func() { <-sem }()

			id, err := e.upload(ctx, st.MigrationID, f)
			e.settleFile(fr, f, id, err)
		}(f)
	}

	wg.Wait()

	return fr.fatal
}

/*
fileRun is what the uploads of one page's files share: the kind's run,
the first error that stops them, and the lock that guards both.
*/
type fileRun struct {
	kindRun
	mu     sync.Mutex
	fatal  error
	cancel context.CancelFunc
}

/*
settleFile records one upload's outcome: a skipped file is reported, an
error stops the other uploads, and an uploaded file is remembered.
*/
func (e *Engine) settleFile(fr *fileRun, f *format.File, id string, err error) {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	var skip *fileSkippedError
	switch {
	case errors.As(err, &skip):
		fr.counts["files_skipped"]++
		e.Report.Report(Event{
			Event: "warning", Kind: fr.kind, Code: "file_skipped", SourceID: f.SourceID, Message: skip.reason,
		})
	case err != nil:
		if fr.fatal == nil {
			fr.fatal = err
			fr.cancel()
		}
	default:
		fr.counts["files"]++
		fr.st.Files[f.SourceID] = id
	}
}

/*
wantedFiles lists the files the records name that dimmi doesn't hold
yet, each once.
*/
func wantedFiles(todo []pending, uploaded map[string]string) []*format.File {
	var wanted []*format.File
	queued := map[string]bool{}

	for _, p := range todo {
		for _, f := range p.rec.Files {
			if f == nil || f.SourceID == "" || uploaded[f.SourceID] != "" || queued[f.SourceID] {
				continue
			}

			queued[f.SourceID] = true
			wanted = append(wanted, f)
		}
	}

	return wanted
}

/*
fileSkippedError is a file left out, with why.
*/
type fileSkippedError struct{ reason string }

func (s *fileSkippedError) Error() string { return s.reason }

/*
upload downloads one file to a private temporary file, then uploads it.
*/
func (e *Engine) upload(ctx context.Context, migration string, f *format.File) (string, error) {
	if f.Size > e.MaxFileSize {
		return "", &fileSkippedError{fmt.Sprintf("a %d MB file is over the %d MB limit", f.Size>>20, e.MaxFileSize>>20)}
	}

	body, err := e.Source.Open(ctx, f)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		return "", &fileSkippedError{fmt.Sprintf("a file could not be downloaded: %v", err)}
	}
	defer func() { _ = body.Close() }()

	tmp, err := os.CreateTemp(e.Store.Dir, "file-*")
	if err != nil {
		return "", err
	}

	defer func() { _ = os.Remove(tmp.Name()) }()
	defer func() { _ = tmp.Close() }()

	n, err := io.Copy(tmp, io.LimitReader(body, e.MaxFileSize+1))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		return "", &fileSkippedError{fmt.Sprintf("a file download broke off: %v", err)}
	}

	if n > e.MaxFileSize {
		return "", &fileSkippedError{fmt.Sprintf("a file is over the %d MB limit", e.MaxFileSize>>20)}
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	id, err := e.Dest.Upload(ctx, migration, Upload{
		SourceID: f.SourceID, Name: f.Name, ContentType: f.ContentType, Size: n, Body: tmp,
	})

	var refused interface{ Refused() bool }
	if errors.As(err, &refused) && refused.Refused() {
		return "", &fileSkippedError{fmt.Sprintf("dimmi refused a file: %v", err)}
	}

	return id, err
}

/*
linkFiles points a record at the files dimmi holds: every file source id
the translator wrote becomes the id dimmi gave the upload, inside lists
and inside Markdown such as dimmi-file:<id>. A file that couldn't be
uploaded is dropped from lists, so the record still goes.
*/
func linkFiles(rec *format.Record, uploaded map[string]string) error {
	if len(rec.Files) == 0 {
		return nil
	}

	ids := map[string]string{}
	for _, f := range rec.Files {
		if f != nil && f.SourceID != "" {
			ids[f.SourceID] = uploaded[f.SourceID]
		}
	}

	raw, err := json.Marshal(rec.Data)
	if err != nil {
		return err
	}

	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}

	out, err := json.Marshal(swapFiles(v, ids))
	if err != nil {
		return err
	}

	rec.Data = json.RawMessage(out)

	return nil
}

/*
swapFiles replaces file source ids in a decoded JSON value with the ids
dimmi gave them.
*/
func swapFiles(v any, ids map[string]string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			x[k] = swapFiles(child, ids)
		}

		return x
	case []any:
		return swapList(x, ids)
	case string:
		return swapString(x, ids)
	}

	return v
}

/*
swapList swaps the files inside a list, dropping a file entry dimmi
doesn't hold.
*/
func swapList(x []any, ids map[string]string) []any {
	out := x[:0]

	for _, child := range x {
		if id, known := fileRef(child, ids); known {
			if id != "" {
				out = append(out, id)
			}

			continue
		}

		out = append(out, swapFiles(child, ids))
	}

	return out
}

/*
fileRef reports whether a list entry is a file source id, and the id
dimmi gave it, empty when the file wasn't uploaded.
*/
func fileRef(child any, ids map[string]string) (string, bool) {
	s, ok := child.(string)
	if !ok {
		return "", false
	}

	id, known := ids[s]

	return id, known
}

/*
swapString swaps a string that is a file source id, or the file source
ids inside it, such as Markdown links.
*/
func swapString(x string, ids map[string]string) string {
	if id, known := ids[x]; known {
		return id
	}

	for src, id := range ids {
		if id != "" && strings.Contains(x, src) {
			x = strings.ReplaceAll(x, src, id)
		}
	}

	return x
}
