/*
|--------------------------------------------------------------------------
| BM25 Ranking
|--------------------------------------------------------------------------
|
| Articles become chunks, one per heading, and chunks become term
| counts. A query is ranked with BM25, the classic keyword score, and
| its similarity is the inverse-document-frequency weight of the query
| terms a chunk contains over the weight of all of them: a chunk that
| covers the rare words of a question scores high, one that shares
| only common words scores low, and a question about something the
| articles never mention scores zero.
|
*/

package knowledge

import (
	"cmp"
	"math"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
BM25 parameters: k1 saturates repeated terms, b normalizes for chunk
length.
*/
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

/*
indexed is one chunk with its term counts.
*/
type indexed struct {
	chunk  contracts.KnowledgeChunk
	terms  map[string]int
	length int
}

/*
corpus is what BM25 needs across all chunks.
*/
type corpus struct {
	docFreq map[string]int
	avgLen  float64
	count   int
}

/*
hit is one ranked chunk.
*/
type hit struct {
	at         int
	score      float64
	similarity float64
}

/*
ParseArticle reads one Markdown file: an optional front matter block
with title and url lines, then the body. Without front matter the
first "# " heading is the title.
*/
func ParseArticle(path, text string) Article {
	a := Article{Body: text}
	frontMatter(&a, text)
	if a.Title != "" {
		return a
	}

	a.Title = strings.TrimSuffix(path, ".md")
	for line := range strings.SplitSeq(a.Body, "\n") {
		if title, ok := strings.CutPrefix(line, "# "); ok {
			a.Title = strings.TrimSpace(title)
			break
		}
	}
	return a
}

/*
frontMatter reads the title and url lines of a leading "---" block
and drops the block from the body.
*/
func frontMatter(a *Article, text string) {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return
	}
	meta, body, found := strings.Cut(rest, "\n---\n")
	if !found {
		return
	}
	a.Body = body
	for line := range strings.SplitSeq(meta, "\n") {
		key, value, _ := strings.Cut(line, ":")
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch strings.TrimSpace(key) {
		case "title":
			a.Title = value
		case "url":
			a.URL = value
		}
	}
}

/*
cutArticle cuts an article at its "## " headings; the text before
the first one is the intro chunk. Empty sections are dropped.
*/
func cutArticle(a Article) []contracts.KnowledgeChunk {
	var out []contracts.KnowledgeChunk
	heading := ""
	var section []string

	flush := func() {
		text := strings.TrimSpace(strings.Join(section, "\n"))
		if text != "" {
			out = append(out, newChunk(a, heading, text))
		}
		section = nil
	}

	for line := range strings.SplitSeq(a.Body, "\n") {
		if strings.HasPrefix(line, "# ") {
			continue
		}
		if h, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			heading = strings.TrimSpace(h)
			continue
		}
		section = append(section, line)
	}
	flush()
	return out
}

/*
newChunk builds one public chunk of an article.
*/
func newChunk(a Article, heading, text string) contracts.KnowledgeChunk {
	path := []string{a.Title}
	if heading != "" {
		path = append(path, heading)
	}
	visibility := a.Visibility
	if visibility == "" {
		visibility = contracts.KnowledgeVisibilityPublic
	}
	return contracts.KnowledgeChunk{
		ID: ids.New(), SourceType: cmp.Or(a.SourceType, "article"), Title: a.Title, HeadingPath: path,
		ArticleID: a.ArticleID, HelpCenterID: a.HelpCenterID, SourceID: a.SourceID, URL: a.URL, Text: text,
		Visibility: visibility, Locale: cmp.Or(a.Locale, "en"), UpdatedAt: a.UpdatedAt,
	}
}

/*
index counts a chunk's terms, its title and heading included.
*/
func index(c contracts.KnowledgeChunk) indexed {
	terms := map[string]int{}
	words := tokens(strings.Join(c.HeadingPath, " ") + " " + c.Text)
	for _, w := range words {
		terms[w]++
	}
	return indexed{chunk: c, terms: terms, length: len(words)}
}

/*
stats computes document frequencies and the average chunk length.
*/
func stats(chunks []indexed) corpus {
	c := corpus{docFreq: map[string]int{}, count: len(chunks)}
	total := 0
	for _, ch := range chunks {
		total += ch.length
		for term := range ch.terms {
			c.docFreq[term]++
		}
	}
	if len(chunks) > 0 {
		c.avgLen = float64(total) / float64(len(chunks))
	}
	return c
}

/*
rank scores every visible chunk against every query and keeps, per
chunk, its best score and similarity, best first.
*/
func (s *Seed) rank(queries []string, visibility []contracts.KnowledgeVisibility) []hit {
	best := map[int]hit{}
	for _, q := range queries {
		s.rankQuery(unique(tokens(q)), visibility, best)
	}

	out := make([]hit, 0, len(best))
	for _, h := range best {
		out = append(out, h)
	}
	slices.SortFunc(out, func(a, b hit) int {
		if a.similarity != b.similarity {
			return compare(b.similarity, a.similarity)
		}
		return compare(b.score, a.score)
	})
	return out
}

/*
rankQuery scores the visible chunks for one query's terms, keeping
each chunk's best hit.
*/
func (s *Seed) rankQuery(terms []string, visibility []contracts.KnowledgeVisibility, best map[int]hit) {
	if len(terms) == 0 {
		return
	}
	for i, ch := range s.chunks {
		if len(visibility) > 0 && !slices.Contains(visibility, ch.chunk.Visibility) {
			continue
		}
		score, sim := s.score(ch, terms)
		if score <= 0 {
			continue
		}
		if prev, ok := best[i]; !ok || sim > prev.similarity {
			best[i] = hit{at: i, score: score, similarity: sim}
		}
	}
}

/*
score returns the chunk's BM25 score and its similarity for the terms.
*/
func (s *Seed) score(ch indexed, terms []string) (float64, float64) {
	score, covered, weight := 0.0, 0.0, 0.0
	for _, t := range terms {
		idf := s.idf(t)
		weight += idf
		tf := float64(ch.terms[t])
		if tf == 0 {
			continue
		}
		covered += idf
		norm := 1 - bm25B + bm25B*float64(ch.length)/math.Max(s.stats.avgLen, 1)
		score += idf * tf * (bm25K1 + 1) / (tf + bm25K1*norm)
	}
	if weight == 0 {
		return 0, 0
	}
	return score, covered / weight
}

/*
idf is the term's inverse document frequency; a term no chunk holds
weighs the most, so an unknown word pulls similarity down.
*/
func (s *Seed) idf(term string) float64 {
	n := float64(s.stats.docFreq[term])
	total := float64(s.stats.count)
	return math.Log(1 + (total-n+0.5)/(n+0.5))
}

/*
tokens lowercases the text, splits it on anything but letters and
digits, drops stop words and folds simple plurals.
*/
func tokens(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 2 || stopWords()[f] {
			continue
		}
		out = append(out, stem(f))
	}
	return out
}

/*
stem folds the plural and verb endings that matter most for matching
a question to an article.
*/
func stem(w string) string {
	switch {
	case len(w) > 5 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case len(w) > 5 && strings.HasSuffix(w, "ing"):
		return w[:len(w)-3]
	case len(w) > 4 && strings.HasSuffix(w, "ed"):
		return w[:len(w)-2]
	case len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		return w[:len(w)-1]
	}
	return w
}

/*
stopWords are the words too common to say what a question is about.
*/
func stopWords() map[string]bool {
	words := map[string]bool{}
	for w := range strings.FieldsSeq(`a an the and or but if of to in on at for from by with about as into
		is are was were be been being am do does did done have has had can could will would shall should
		may might must i me my we our you your it its this that these those there here what which who whom
		how when where why much many some any all no not so than then too very just also more most
		please hi hello hey thanks thank get got want need know tell let use using one quick question
		find look looking wondering wonder`) {
		words[w] = true
	}
	return words
}

/*
unique keeps the first occurrence of each term.
*/
func unique(terms []string) []string {
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

/*
compare orders two floats for sorting.
*/
func compare(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

/*
hostOf returns a URL's lowercase host, or "".
*/
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
