/*
|--------------------------------------------------------------------------
| Markdown Upload Source
|--------------------------------------------------------------------------
|
| A zip of Markdown files, or one Markdown file, uploaded from the
| dashboard. The archive is read from the workspace's storage, never
| unpacked to disk, with limits on its size, its number of files and
| each file's size, and paths that climb out of the archive are skipped.
| A file's checksum is its version, so uploading a new archive re-reads
| only what changed, and a file missing from it is removed by the sync.
|
*/

package upload

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
Archive limits: the largest archive, the most files read from it, and
the largest file.
*/
const (
	maxArchive = 50 << 20
	maxFiles   = 2000
	maxFile    = 2 << 20
)

/*
Prefix is the storage folder uploads are kept in.
*/
const Prefix = "knowledge-uploads/"

/*
schema is the settings form of an upload source.
*/
const schema = `{
  "type": "object",
  "required": ["upload"],
  "properties": {
    "upload": {"type": "string", "title": "Archive", "format": "x-upload",
      "description": "A .zip of Markdown files, or a single .md file."}
  }
}`

/*
settings are an upload source's settings: the key of the stored file.
*/
type settings struct {
	Upload string `json:"upload"`
}

/*
Driver reads uploaded archives from the storage.
*/
type Driver struct {
	Files filesystem.Filesystem
}

/*
New returns the upload driver reading from the storage.
*/
func New(files filesystem.Filesystem) *Driver {
	return &Driver{Files: files}
}

/*
Describe says what the driver is.
*/
func (d *Driver) Describe() sources.Description {
	return sources.Description{
		Type: "markdown_upload", Name: "Markdown files", Logo: "markdown", Auth: sources.AuthNone,
		Summary: "Upload a zip of Markdown files. Upload again to replace them.",
		Schema:  json.RawMessage(schema), Modes: []sources.Mode{sources.ModeSync, sources.ModeImport},
		Targets:   []sources.Target{sources.TargetAgent, sources.TargetOperator, sources.TargetHelpCenter},
		Schedules: []string{"manual"},
	}
}

/*
read decodes and checks the settings.
*/
func read(cfg sources.Config) (settings, error) {
	var s settings
	if err := cfg.Decode(&s); err != nil {
		return s, err
	}

	if !strings.HasPrefix(s.Upload, Prefix) || strings.Contains(s.Upload, "..") {
		return s, &sources.ConfigError{Field: "upload", Message: "Upload a .zip or .md file."}
	}

	return s, nil
}

/*
Check opens the uploaded file.
*/
func (d *Driver) Check(ctx context.Context, cfg sources.Config) (sources.CheckResult, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.CheckResult{}, err
	}

	files, err := d.open(ctx, s)
	if err != nil {
		return sources.CheckResult{}, err
	}

	return sources.CheckResult{Account: path.Base(s.Upload), Estimate: len(files)}, nil
}

/*
List returns every text file of the archive in one page.
*/
func (d *Driver) List(ctx context.Context, cfg sources.Config, _ string) (sources.Page, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Page{}, err
	}

	files, err := d.open(ctx, s)
	if err != nil {
		return sources.Page{}, err
	}

	out := sources.Page{}
	for name, f := range files {
		out.Refs = append(out.Refs, sources.Ref{
			ExternalID: name, Path: name, Title: sources.Humanize(name), Version: strconv.FormatUint(f.sum, 16),
		})
	}

	return out, nil
}

/*
Fetch reads one file of the archive.
*/
func (d *Driver) Fetch(ctx context.Context, cfg sources.Config, ref sources.Ref) (sources.Document, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Document{}, err
	}

	files, err := d.open(ctx, s)
	if err != nil {
		return sources.Document{}, err
	}

	f, ok := files[ref.ExternalID]
	if !ok {
		return sources.Document{}, sources.ErrNotFound
	}

	doc := sources.MarkdownFile(ref.ExternalID, f.body)
	doc.Collections = sources.Folders(ref.ExternalID)
	doc.Visibility = sources.VisibilityInternal

	return doc, nil
}

/*
file is one text file of an archive and its checksum.
*/
type file struct {
	body []byte
	sum  uint64
}

/*
open reads the upload's text files by path.
*/
func (d *Driver) open(ctx context.Context, s settings) (map[string]file, error) {
	r, err := d.Files.Open(ctx, s.Upload)
	if err != nil {
		return nil, fmt.Errorf("open upload: %w", sources.ErrNotFound)
	}

	defer func() { _ = r.Close() }()

	raw, err := io.ReadAll(io.LimitReader(r, maxArchive+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}

	if len(raw) > maxArchive {
		return nil, &sources.ConfigError{Field: "upload", Message: "The file is larger than 50 MB."}
	}

	if !bytes.HasPrefix(raw, []byte("PK")) {
		name := strings.TrimSuffix(path.Base(s.Upload), path.Ext(s.Upload))
		return map[string]file{name + ".md": {body: raw, sum: checksum(raw)}}, nil
	}

	return unzip(raw)
}

/*
unzip reads the archive's text files, skipping folders, other kinds of
file, paths that climb out of the archive and files over the limit.
*/
func unzip(raw []byte) (map[string]file, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, &sources.ConfigError{Field: "upload", Message: "The archive could not be opened."}
	}

	out := map[string]file{}

	for _, entry := range zr.File {
		name := path.Clean(strings.TrimPrefix(entry.Name, "/"))
		if len(out) >= maxFiles || entry.FileInfo().IsDir() || strings.HasPrefix(name, "..") ||
			strings.HasPrefix(path.Base(name), ".") || strings.HasPrefix(name, "__MACOSX") || !isText(name) ||
			entry.UncompressedSize64 > maxFile {
			continue
		}

		rc, err := entry.Open()
		if err != nil {
			continue
		}

		body, err := io.ReadAll(io.LimitReader(rc, maxFile))
		_ = rc.Close()

		if err == nil {
			out[name] = file{body: body, sum: uint64(entry.CRC32)}
		}
	}

	return out, nil
}

/*
isText reports whether a file is read as a document: Markdown, MDX or
plain text.
*/
func isText(name string) bool {
	return slices.Contains([]string{".md", ".mdx", ".markdown", ".txt"}, strings.ToLower(path.Ext(name)))
}

/*
checksum is a cheap version for a single uploaded file: its length and
a rolling sum of its bytes.
*/
func checksum(b []byte) uint64 {
	sum := uint64(len(b))
	for _, c := range b {
		sum = sum*31 + uint64(c)
	}

	return sum
}
