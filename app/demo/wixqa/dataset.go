/*
|--------------------------------------------------------------------------
| The WixQA Dataset
|--------------------------------------------------------------------------
|
| WixQA (Wix.com AI Research, MIT license) holds 6,221 real Wix help
| articles and questions with expert-written answers that point at the
| articles holding the answer. The files are read where they were
| downloaded, at run time; nothing of the dataset is kept in the
| repository. A missing folder says how to download it.
|
*/

package wixqa

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/filesystem"
)

/*
DatasetURL is where the dataset is downloaded from.
*/
const DatasetURL = "https://huggingface.co/datasets/Wix/WixQA"

/*
DefaultDir is where the dataset is looked for, inside the repository's
ignored tmp folder.
*/
const DefaultDir = "tmp/datasets/wixqa"

/*
ErrMissing marks a dataset folder without the files.
*/
var ErrMissing = errors.New("the WixQA dataset is missing")

/*
The question sets.
*/
const (
	SetExpert    = "expert"
	SetSimulated = "simulated"
)

/*
Doc is one help article of the corpus.
*/
type Doc struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Contents string `json:"contents"`
	HTML     string `json:"html_content"`
	Type     string `json:"article_type"`
}

/*
Question is one question with its expert answer and the articles that
hold it.
*/
type Question struct {
	Question   string   `json:"question"`
	Answer     string   `json:"answer"`
	ArticleIDs []string `json:"-"`
}

/*
Dataset is the downloaded dataset, on a disk rooted at its folder.
*/
type Dataset struct {
	Disk filesystem.Filesystem
	Dir  string
}

/*
Corpus reads the help articles.
*/
func (ds Dataset) Corpus(ctx context.Context) ([]Doc, error) {
	var out []Doc
	err := ds.readLines(ctx, path.Join("wix_kb_corpus", "wix_kb_corpus.jsonl"), func(line []byte) error {
		var d Doc
		if err := json.Unmarshal(line, &d); err != nil {
			return err
		}
		out = append(out, d)
		return nil
	})
	return out, err
}

/*
Questions reads one question set.
*/
func (ds Dataset) Questions(ctx context.Context, set string) ([]Question, error) {
	folder := map[string]string{SetExpert: "wixqa_expertwritten", SetSimulated: "wixqa_simulated"}[set]
	if folder == "" {
		return nil, fmt.Errorf("unknown question set %q: use %s or %s", set, SetExpert, SetSimulated)
	}
	var out []Question
	err := ds.readLines(ctx, path.Join(folder, "test.jsonl"), func(line []byte) error {
		var raw struct {
			Question
			IDs json.RawMessage `json:"article_ids"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			return err
		}
		raw.ArticleIDs = articleIDs(raw.IDs)
		out = append(out, raw.Question)
		return nil
	})
	return out, err
}

/*
articleIDs reads the article ids, a list or a string holding one.
*/
func articleIDs(raw json.RawMessage) []string {
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return nil
	}
	if json.Unmarshal([]byte(strings.ReplaceAll(text, "'", `"`)), &list) == nil {
		return list
	}
	return nil
}

/*
readLines reads a JSON-lines file, saying how to download it when it is
missing.
*/
func (ds Dataset) readLines(ctx context.Context, file string, each func([]byte) error) error {
	found, err := ds.Disk.Exists(ctx, file)
	if err != nil || !found {
		return fmt.Errorf("%w: %s not found in %s. Download it from %s (for example with "+
			"`huggingface-cli download Wix/WixQA --repo-type dataset --local-dir %s`)", errors.Join(ErrMissing, err),
			file, ds.Dir, DatasetURL, DefaultDir)
	}
	f, err := ds.Disk.Open(ctx, file)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		if err := each(sc.Bytes()); err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
	}
	return sc.Err()
}

/*
nonSlug matches what a slug may not hold: anything but lowercase
letters and digits, runs of dashes included.
*/
var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

/*
Slug is an article's slug in the help center: the last part of its Wix
address, which names the article.
*/
func Slug(d Doc) string {
	s := strings.ToLower(d.URL[strings.LastIndex(d.URL, "/")+1:])
	s = strings.Trim(nonSlug.ReplaceAllString(s, "-"), "-")
	if s == "" || len(s) > 100 {
		return "wix-" + d.ID[:12]
	}
	return s
}
