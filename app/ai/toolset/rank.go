/*
|--------------------------------------------------------------------------
| Likely Tools
|--------------------------------------------------------------------------
|
| Before a turn the layer ranks the tools behind it against the newest
| message and names the likeliest in that message, so the model can load
| the right one without listing a group first. Tools are ranked by
| keywords.
|
*/

package toolset

import (
	"context"
	"math"
	"slices"
	"sort"
	"strings"
	"unicode"
)

/*
SuggestCount is how many likely tools the newest message names.
*/
const SuggestCount = 20

/*
Ranker orders tools by how likely the message needs them, returning at
most n names.
*/
type Ranker interface {
	/*
		Rank returns the names of the likeliest tools, best first.
	*/
	Rank(ctx context.Context, message string, entries []Entry, n int) ([]string, error)
}

/*
Keywords ranks tools by BM25 over their names and descriptions.
*/
type Keywords struct{}

/*
BM25's constants.
*/
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

/*
Rank scores each tool's text against the message's words.
*/
func (Keywords) Rank(_ context.Context, message string, entries []Entry, n int) ([]string, error) {
	query := uniqueWords(message)
	docs := make([][]string, len(entries))
	total := 0
	frequency := map[string]int{}
	for i, e := range entries {
		docs[i] = words(e.Spec.Name + " " + e.Group + " " + e.Spec.Description)
		total += len(docs[i])
		for _, w := range uniqueWords(strings.Join(docs[i], " ")) {
			frequency[w]++
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}
	average := float64(total) / float64(len(entries))
	scores := make([]float64, len(entries))
	for i, doc := range docs {
		counts := map[string]int{}
		for _, w := range doc {
			counts[w]++
		}
		for _, q := range query {
			tf := float64(counts[q])
			if tf == 0 {
				continue
			}
			df := float64(frequency[q])
			idf := math.Log(1 + (float64(len(entries))-df+0.5)/(df+0.5))
			scores[i] += idf * tf * (bm25K1 + 1) / (tf + bm25K1*(1-bm25B+bm25B*float64(len(doc))/average))
		}
	}
	return top(entries, scores, n), nil
}

/*
Suggest ranks the tools against the message, never failing the turn: a
ranking that fails names no tools.
*/
func (s *Set) Suggest(ctx context.Context, r Ranker, message string) []string {
	if r == nil || strings.TrimSpace(message) == "" {
		return nil
	}
	names, err := r.Rank(ctx, message, s.Entries(), SuggestCount)
	if err != nil {
		return nil
	}
	return names
}

/*
top returns the names of the best-scoring entries, ties by name, no
entry scoring zero.
*/
func top(entries []Entry, scores []float64, n int) []string {
	idx := make([]int, 0, len(entries))
	for i := range entries {
		if scores[i] > 0 {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		if scores[idx[a]] != scores[idx[b]] {
			return scores[idx[a]] > scores[idx[b]]
		}
		return entries[idx[a]].Spec.Name < entries[idx[b]].Spec.Name
	})
	if len(idx) > n {
		idx = idx[:n]
	}
	out := make([]string, 0, len(idx))
	for _, i := range idx {
		out = append(out, entries[i].Spec.Name)
	}
	return out
}

/*
words splits a text into lower-case words, tool names split at their
underscores, with a plural's s dropped so "tickets" meets "ticket".
*/
func words(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 3 || slices.Contains(stopWords(), f) {
			continue
		}
		if len(f) > 3 && strings.HasSuffix(f, "s") && !strings.HasSuffix(f, "ss") {
			f = f[:len(f)-1]
		}
		out = append(out, f)
	}
	return out
}

/*
uniqueWords are a text's words, each once.
*/
func uniqueWords(text string) []string {
	w := words(text)
	slices.Sort(w)
	return slices.Compact(w)
}

/*
stopWords carry no meaning for ranking.
*/
func stopWords() []string {
	return []string{
		"the", "and", "for", "you", "your", "with", "this", "that", "are", "was", "but", "not", "can", "from",
		"have", "has", "what", "when", "how", "why", "who", "all", "any", "one", "our", "out", "get", "use",
		"its", "their", "them", "there", "then", "than", "into", "about", "just", "please", "thank", "thanks",
	}
}
