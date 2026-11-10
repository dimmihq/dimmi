/*
|--------------------------------------------------------------------------
| Pleasantries
|--------------------------------------------------------------------------
|
| In a shared place people say thanks, ok and a thumbs up all the time.
| None of it is a question, so none of it may open a conversation, wake
| one, run the agent or count as anything billable. A message is a
| pleasantry when, short and without a question mark, it is only phrases
| from the word lists, emoji and punctuation. Anything else is a real
| message: better a conversation too many than a question lost.
|
| The word lists live with the translations, one per language, so a
| language is added without a code change.
|
*/

package chatspace

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

/*
listFile is the name of a language's word list.
*/
const listFile = "channels_pleasantries.json"

/*
emojiCode matches an emoji written as its short code, such as :tada:.
*/
var emojiCode = regexp.MustCompile(`:[a-z0-9_+\-']+:`)

/*
Pleasantries tells pleasantries from real messages. MaxChars is the
longest a pleasantry may be.
*/
type Pleasantries struct {
	phrases  map[string]bool
	MaxChars int
}

/*
LoadPleasantries reads every language's word list from the translation
files, one directory per locale.
*/
func LoadPleasantries(files fs.FS, maxChars int) (*Pleasantries, error) {
	matches, err := fs.Glob(files, path.Join("*", listFile))
	if err != nil {
		return nil, fmt.Errorf("find pleasantry lists: %w", err)
	}

	p := &Pleasantries{phrases: map[string]bool{}, MaxChars: maxChars}

	for _, file := range matches {
		raw, err := fs.ReadFile(files, file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}

		var list struct {
			Phrases string `json:"phrases"`
		}

		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}

		for phrase := range strings.SplitSeq(list.Phrases, "|") {
			if phrase = normalize(phrase); phrase != "" {
				p.phrases[phrase] = true
			}
		}
	}

	return p, nil
}

/*
Is reports whether the message is only a pleasantry.
*/
func (p *Pleasantries) Is(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.ContainsAny(trimmed, "?？") || utf8.RuneCountInString(trimmed) > p.MaxChars {
		return false
	}

	words := normalize(emojiCode.ReplaceAllString(trimmed, " "))
	if words == "" {
		return true
	}

	return p.phrasesOnly(strings.Fields(words))
}

/*
phrasesOnly reports whether the words split into listed phrases, each
of up to four words.
*/
func (p *Pleasantries) phrasesOnly(words []string) bool {
	if len(words) == 0 {
		return true
	}

	for n := min(4, len(words)); n > 0; n-- {
		if p.phrases[strings.Join(words[:n], " ")] && p.phrasesOnly(words[n:]) {
			return true
		}
	}

	return false
}

/*
normalize lowercases the text and keeps only its letters, digits and
single spaces, so emoji, punctuation and stretched words fall away.
*/
func normalize(text string) string {
	var b strings.Builder

	space := true

	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r):
			b.WriteRune(r)

			space = false
		case !space:
			b.WriteRune(' ')

			space = true
		}
	}

	return strings.TrimSpace(b.String())
}
