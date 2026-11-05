/*
|--------------------------------------------------------------------------
| Tag Resources
|--------------------------------------------------------------------------
|
| A tag as chips and pickers read it: its name and the color of its dot.
| The settings page also asks how many records carry each tag, which
| counts live records only.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Tag is one label of the workspace's vocabulary.
*/
type Tag struct {
	ID     string  `json:"id"`
	Object string  `json:"object"`
	Name   string  `json:"name"`
	Color  *string `json:"color"`
}

/*
NewTag shapes the tag.
*/
func NewTag(t models.Tag) Tag {
	return Tag{ID: models.PublicID(models.PrefixTag, t.ID), Object: "tag", Name: t.Name, Color: t.Color}
}

/*
NewTags shapes a list of tags, in the order given, an empty list when
there are none.
*/
func NewTags(rows []models.Tag) []Tag {
	out := make([]Tag, 0, len(rows))
	for _, t := range rows {
		out = append(out, NewTag(t))
	}

	return out
}

/*
TagCounts says how many live records carry a tag.
*/
type TagCounts struct {
	Conversations int64 `json:"conversations"`
	Contacts      int64 `json:"contacts"`
	Companies     int64 `json:"companies"`
}

/*
ListedTag is a tag in the list of tags, with its counts when asked for.
*/
type ListedTag struct {
	Tag

	Counts *TagCounts `json:"counts,omitempty"`
}

/*
NewListedTags shapes the listed tags, without counts.
*/
func NewListedTags(rows []tags.Listed) []ListedTag {
	out := make([]ListedTag, 0, len(rows))
	for _, row := range rows {
		out = append(out, ListedTag{Tag: NewTag(row.Tag)})
	}

	return out
}

/*
NewCountedTags shapes the listed tags with how many records carry each.
*/
func NewCountedTags(rows []tags.Listed) []ListedTag {
	out := NewListedTags(rows)
	for i, row := range rows {
		out[i].Counts = &TagCounts{
			Conversations: row.Conversations, Contacts: row.Contacts, Companies: row.Companies,
		}
	}

	return out
}
