/*
|--------------------------------------------------------------------------
| Note Resource
|--------------------------------------------------------------------------
|
| What a note carries beyond its text: the teammates and teams it names,
| and the emoji teammates reacted with. Both are read from the summary
| kept in the note's metadata, so listing a thread never runs another
| query for them.
|
*/

package resources

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Mention is a teammate or team a note names: its kind, member or team,
its public id and its name when the note was written.
*/
type Mention struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
Reaction is one emoji on a note: how many teammates reacted with it and
who, in the order they did.
*/
type Reaction struct {
	Emoji     string   `json:"emoji"`
	Count     int      `json:"count"`
	MemberIDs []string `json:"member_ids"`
}

/*
reactionsOf reads a note's reactions from the summary in its metadata.
*/
func reactionsOf(metadata json.RawMessage) []Reaction {
	var read struct {
		Reactions []struct {
			Emoji     string     `json:"emoji"`
			MemberIDs []ids.UUID `json:"member_ids"`
		} `json:"reactions"`
	}

	out := []Reaction{}
	if json.Unmarshal(metadata, &read) != nil {
		return out
	}

	for _, r := range read.Reactions {
		members := make([]string, 0, len(r.MemberIDs))
		for _, id := range r.MemberIDs {
			members = append(members, models.PublicID(models.PrefixMember, id))
		}

		out = append(out, Reaction{Emoji: r.Emoji, Count: len(members), MemberIDs: members})
	}

	return out
}

/*
mentionsOf reads a note's mentions from its metadata, an empty list
when it has none.
*/
func mentionsOf(metadata json.RawMessage) []Mention {
	var read struct {
		Mentions []Mention `json:"mentions"`
	}

	if json.Unmarshal(metadata, &read) != nil || read.Mentions == nil {
		return []Mention{}
	}

	return read.Mentions
}
