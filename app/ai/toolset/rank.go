/*
|--------------------------------------------------------------------------
| Likely Tools
|--------------------------------------------------------------------------
|
| Before a turn the layer ranks the tools behind it against the newest
| message and names the likeliest in that message, so the model can load
| the right one without listing a group first. Keywords rank by default.
| When the workspace embeds, tools are ranked by meaning instead, each
| tool's text embedded once per process and kept. A decider, when one is
| configured, re-orders the leading candidates; without one the ranking
| stands as it is.
|
*/

package toolset

import (
	"context"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/dimmihq/dimmi/framework/hashing"

	"github.com/dimmihq/dimmi/app/contracts"
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
Embedder embeds texts, one unit vector per text: the workspace's
embeddings driver and model.
*/
type Embedder func(ctx context.Context, texts []string) ([][]float32, error)

/*
Vectors ranks tools by cosine similarity to the message, embedding each
tool's text once and keeping it under the model's name and a hash of
the text. Query optionally hands over the message's vector when it was
already embedded, as for a knowledge search.
*/
type Vectors struct {
	Model string
	Embed Embedder
	Query []float32

	mu    sync.Mutex
	cache map[string][]float32
}

/*
Rank embeds what is missing and orders by similarity; when embedding
fails it falls back to keywords.
*/
func (v *Vectors) Rank(ctx context.Context, message string, entries []Entry, n int) ([]string, error) {
	keys, missing, missingKeys := v.missing(entries)
	query := v.Query
	texts := missing
	if query == nil {
		texts = append(slices.Clone(missing), message)
	}
	if len(texts) > 0 {
		vecs, err := v.Embed(ctx, texts)
		if err != nil || len(vecs) != len(texts) {
			return Keywords{}.Rank(ctx, message, entries, n)
		}
		v.keep(missingKeys, vecs)
		if query == nil {
			query = vecs[len(vecs)-1]
		}
	}
	scores := make([]float64, len(entries))
	v.mu.Lock()
	for i, k := range keys {
		scores[i] = cosine(query, v.cache[k])
	}
	v.mu.Unlock()
	return top(entries, scores, n), nil
}

/*
missing returns each entry's key, and the texts and keys not yet
embedded.
*/
func (v *Vectors) missing(entries []Entry) ([]string, []string, []string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cache == nil {
		v.cache = map[string][]float32{}
	}
	keys := make([]string, len(entries))
	var texts, missingKeys []string
	for i, e := range entries {
		text := toolText(e)
		keys[i] = v.key(text)
		if _, ok := v.cache[keys[i]]; !ok && !slices.Contains(missingKeys, keys[i]) {
			texts, missingKeys = append(texts, text), append(missingKeys, keys[i])
		}
	}
	return keys, texts, missingKeys
}

/*
keep stores embedded vectors under their keys.
*/
func (v *Vectors) keep(keys []string, vecs [][]float32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i, k := range keys {
		v.cache[k] = vecs[i]
	}
}

/*
key names a tool's vector by model and text.
*/
func (v *Vectors) key(text string) string {
	return hashing.SHA256Hex([]byte(v.Model + "\x00" + text))
}

/*
Decided asks a decider to order the base ranking's candidates, and
keeps the base order when there is no decider or it fails.
*/
type Decided struct {
	Base    Ranker
	Decider contracts.Decider
}

/*
decisionName names the decider's question.
*/
const decisionName = "tool"

/*
Rank re-orders the base candidates by the decider's probabilities.
*/
func (d Decided) Rank(ctx context.Context, message string, entries []Entry, n int) ([]string, error) {
	names, err := d.Base.Rank(ctx, message, entries, n)
	if err != nil || d.Decider == nil || len(names) < 2 {
		return names, err
	}
	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Spec.Name] = e
	}
	return reorder(names, d.decide(ctx, message, names, byName)), nil
}

/*
decide asks the decider how likely each candidate is; nil when it
cannot say.
*/
func (d Decided) decide(ctx context.Context, message string, names []string, byName map[string]Entry,
) map[string]float64 {
	choices := map[string]string{}
	for _, name := range names {
		choices[name] = firstSentence(byName[name].Spec.Description)
	}
	res, err := d.Decider.Decide(ctx, contracts.DecisionRequest{
		State: message,
		Questions: map[string]contracts.DecisionQuestion{decisionName: {
			Type: contracts.DecisionChoice, Choices: choices,
			Instructions: "Which tool does a support agent most likely need to answer this message?",
		}},
	})
	if err != nil || res == nil {
		return nil
	}
	return res.Answers[decisionName].Probabilities
}

/*
reorder sorts names by probability, keeping the order of ties.
*/
func reorder(names []string, p map[string]float64) []string {
	out := slices.Clone(names)
	sort.SliceStable(out, func(i, j int) bool { return p[out[i]] > p[out[j]] })
	return out
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
toolText is what a tool is ranked by.
*/
func toolText(e Entry) string {
	return strings.ReplaceAll(e.Spec.Name, "_", " ") + ". " + e.Group + ". " + e.Spec.Description
}

/*
cosine is the similarity of two vectors.
*/
func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
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
