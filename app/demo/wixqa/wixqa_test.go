package wixqa_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"

	"github.com/dimmihq/dimmi/app/demo/wixqa"
)

func dataset(t *testing.T) wixqa.Dataset {
	t.Helper()
	disk, err := local.New(local.Options{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return wixqa.Dataset{Disk: disk, Dir: "test"}
}

func write(t *testing.T, ds wixqa.Dataset, rel, body string) {
	t.Helper()
	if err := ds.Disk.Put(t.Context(), rel, []byte(body), filesystem.PutOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestReadsTheDatasetWhereItWasDownloaded(t *testing.T) {
	dir := dataset(t)
	write(t, dir, "wix_kb_corpus/wix_kb_corpus.jsonl",
		`{"id":"abc123abc123abc1","url":"https://support.wix.com/en/article/Adding--a_Store!","title":"Adding a store",`+
			`"contents":"Adding a store\nOpen the editor.","html_content":"<p>Open the editor.</p>",`+
			`"article_type":"article"}`+"\n")
	write(t, dir, "wixqa_expertwritten/test.jsonl",
		`{"question":"How do I add a store?","answer":"Open the editor.","article_ids":"['abc123abc123abc1']"}`+"\n")
	write(t, dir, "wixqa_simulated/test.jsonl",
		`{"question":"store?","answer":"Open the editor.","article_ids":["abc123abc123abc1"]}`+"\n")

	docs, err := dir.Corpus(t.Context())
	if err != nil || len(docs) != 1 || wixqa.Slug(docs[0]) != "adding-a-store" {
		t.Fatalf("docs = %+v, %v", docs, err)
	}
	for _, set := range []string{wixqa.SetExpert, wixqa.SetSimulated} {
		qs, err := dir.Questions(t.Context(), set)
		if err != nil || len(qs) != 1 || len(qs[0].ArticleIDs) != 1 || qs[0].ArticleIDs[0] != "abc123abc123abc1" {
			t.Fatalf("%s = %+v, %v", set, qs, err)
		}
	}
}

func TestSaysHowToDownloadAMissingDataset(t *testing.T) {
	_, err := dataset(t).Corpus(t.Context())
	if !errors.Is(err, wixqa.ErrMissing) || !strings.Contains(err.Error(), wixqa.DatasetURL) {
		t.Fatalf("err = %v", err)
	}
}
