/*
|--------------------------------------------------------------------------
| The Newest Message's Note
|--------------------------------------------------------------------------
|
| Each customer or teammate message starts a fresh turn, and the tools
| loaded during the turns before it are gone with their results. The
| newest message carries a short note: the tools loaded earlier, which
| must be loaded again before use, and the tools likeliest to help with
| this message. The note sits in the newest message so everything before
| it stays the same from call to call and can be cached.
|
*/

package toolset

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Note is the newest message's note, empty when there is nothing to say.
*/
func Note(earlier, likely []string) string {
	var lines []string
	if len(earlier) > 0 {
		lines = append(lines, "Tools loaded earlier in this conversation, load again before use: "+
			strings.Join(earlier, ", ")+".")
	}
	if len(likely) > 0 {
		lines = append(lines, "Likely tools for this message: "+strings.Join(likely, ", ")+".")
	}
	if len(lines) == 0 {
		return ""
	}
	return "[tools] " + strings.Join(lines, " ")
}

/*
Record is what the layer did during one run, kept on the run: the
tools it loaded, the tools the newest message named as likely, its
counts, and the estimated tokens of the fixed tool list and the groups'
lines.
*/
type Record struct {
	Loaded       []string `json:"loaded"`
	Suggested    []string `json:"suggested,omitempty"`
	Earlier      []string `json:"earlier,omitempty"`
	Stats        Stats    `json:"stats"`
	FixedTokens  int      `json:"fixed_tokens"`
	GroupsTokens int      `json:"groups_tokens"`
}

/*
Record sums up the run so far.
*/
func (s *Set) Record(fixed []contracts.ToolSpec, suggested, earlier []string) Record {
	return Record{
		Loaded: s.Loaded(), Suggested: suggested, Earlier: earlier, Stats: s.Stats(),
		FixedTokens: Tokens(fixed), GroupsTokens: (len(s.Prompt()) + charsPerToken - 1) / charsPerToken,
	}
}

/*
charsPerToken estimates tokens from characters.
*/
const charsPerToken = 4

/*
Tokens estimates the tokens of tool definitions.
*/
func Tokens(specs []contracts.ToolSpec) int {
	n := 0
	for _, s := range specs {
		n += len(s.Name) + len(s.Description) + len(s.Schema)
	}
	return (n + charsPerToken - 1) / charsPerToken
}

/*
Earlier reads the tools earlier runs loaded from their records, each
once and by name, keeping only the tools still behind the layer.
*/
func Earlier(records []json.RawMessage, known func(string) bool) []string {
	var out []string
	for _, raw := range records {
		var r Record
		if json.Unmarshal(raw, &r) != nil {
			continue
		}
		for _, n := range r.Loaded {
			if known == nil || known(n) {
				out = append(out, n)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
