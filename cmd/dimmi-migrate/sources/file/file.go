/*
Package file reads records already in dimmi's import format from a JSON
Lines file: one record a line, as format.Line describes. It's how a
tool dimmi-migrate has no translator for comes across: export it, write
the lines, and run `dimmi-migrate import file.jsonl`.
*/
package file

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/engine"
	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/format"
)

/*
Source reads one import file. Tool is the tool it came from, and
AccountID the account in it; both default from the file.
*/
type Source struct {
	Path      string
	Tool      string
	AccountID string
	HTTP      *http.Client
}

func (s *Source) Name() string {
	if s.Tool != "" {
		return s.Tool
	}

	return "file"
}

func (s *Source) Kinds() []format.Kind { return format.Kinds() }

/*
Account names the file's account: given, or the file's own name and a
hash of its full path, so the same file resumes the same migration.
*/
func (s *Source) Account(context.Context) (engine.Account, error) {
	abs, err := filepath.Abs(s.Path)
	if err != nil {
		return engine.Account{}, err
	}

	if s.AccountID != "" {
		return engine.Account{ID: s.AccountID, Name: s.AccountID}, nil
	}

	sum := sha256.Sum256([]byte(abs))

	return engine.Account{ID: hex.EncodeToString(sum[:8]), Name: filepath.Base(abs)}, nil
}

/*
scan calls fn for every line from the given one, numbered from 1.
*/
func (s *Source) scan(from int, fn func(n int, rec format.Record) error) error {
	f, err := os.Open(s.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)

	for n := 1; sc.Scan(); n++ {
		if n < from {
			continue
		}

		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		rec, err := format.ParseLine([]byte(line))
		if err != nil {
			return fmt.Errorf("%s line %d: %w", filepath.Base(s.Path), n, err)
		}

		if err := fn(n, rec); err != nil {
			return err
		}
	}

	return sc.Err()
}

func (s *Source) Discover(context.Context) (engine.Inventory, error) {
	acct, err := s.Account(context.Background())
	if err != nil {
		return engine.Inventory{}, err
	}

	counts := map[format.Kind]int{}

	if err := s.scan(1, func(_ int, rec format.Record) error {
		counts[rec.Kind]++

		return nil
	}); err != nil {
		return engine.Inventory{}, err
	}

	inv := engine.Inventory{Account: acct}
	for _, k := range format.Kinds() {
		if counts[k] > 0 {
			inv.Kinds = append(inv.Kinds, engine.KindCount{Kind: k, Count: counts[k]})
		}
	}

	return inv, nil
}

/*
Read sends the file's records of the kind, 100 lines a page, the cursor
being the next line to read.
*/
func (s *Source) Read(_ context.Context, kind format.Kind, opts engine.ReadOptions, emit func(engine.Page) error) error {
	from, _ := strconv.Atoi(opts.Cursor)
	if from < 1 {
		from = 1
	}

	var page []format.Record

	err := s.scan(from, func(n int, rec format.Record) error {
		if rec.Kind != kind {
			return nil
		}

		page = append(page, rec)
		if len(page) < 100 {
			return nil
		}

		err := emit(engine.Page{Records: page, Cursor: strconv.Itoa(n + 1)})
		page = nil

		return err
	})
	if err != nil {
		return err
	}

	return emit(engine.Page{Records: page})
}

/*
Open reads a file a line names: a path, relative to the import file, or
a URL.
*/
func (s *Source) Open(ctx context.Context, f *format.File) (io.ReadCloser, error) {
	if path, ok := strings.CutPrefix(f.Source, "file://"); ok {
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(s.Path), path)
		}

		return os.Open(path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.Source, nil)
	if err != nil {
		return nil, err
	}

	hc := s.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}

	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()

		return nil, fmt.Errorf("download answered %d", res.StatusCode)
	}

	return res.Body, nil
}
