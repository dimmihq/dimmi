/*
|--------------------------------------------------------------------------
| Conversation Attribute Files
|--------------------------------------------------------------------------
|
| A file attribute holds the public ids of uploads. Each one must be an
| upload of the workspace when it is set; the conversation keeps the ids
| its values hold, so the purge of unused uploads leaves them alone; and
| reading the values presents each file with a short-lived link.
|
*/

package aiattributes

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/models"
)

/*
present reads the files the values hold, each with a short-lived link
once the malware scan lets it through.
*/
func (a *Attributes) present(ctx context.Context, q *models.Queries, workspaceID ids.UUID, out *Found) error {
	var want []ids.UUID

	for _, v := range out.Values {
		if v.Attribute.DataType == TypeFile {
			want = append(want, filesOf(v.Value)...)
		}
	}

	if len(want) == 0 || a.Disks == nil {
		return nil
	}

	rows, err := q.ListAttachmentsByID(ctx, models.ListAttachmentsByIDParams{WorkspaceID: workspaceID, Ids: want})
	if err != nil {
		return fmt.Errorf("list attribute files: %w", err)
	}

	out.Files = map[string]messages.File{}
	for _, row := range rows {
		out.Files[models.PublicID(models.PrefixAttachment, row.ID)] = messages.PresentFile(ctx, a.Disks, row)
	}

	return nil
}

/*
fileRegistry reads the public ids of files.
*/
func fileRegistry() *ids.Registry {
	r := ids.NewRegistry()
	_ = r.Register(string(models.PrefixAttachment), "attachment")

	return r
}

/*
filesOf returns the files a value of a file attribute holds.
*/
func filesOf(value any) []ids.UUID {
	list, _ := value.([]any)
	registry := fileRegistry()

	out := make([]ids.UUID, 0, len(list))
	for _, x := range list {
		s, _ := x.(string)
		if id, err := registry.Decode(string(models.PrefixAttachment), s); err == nil {
			out = append(out, id)
		}
	}

	return out
}

/*
fileIDs returns every file the values of file attributes hold, so the
purge of unused uploads keeps them.
*/
func fileIDs(defs []models.AiConversationAttribute, values map[string]any) []ids.UUID {
	out := []ids.UUID{}

	for _, d := range defs {
		if d.DataType == TypeFile {
			for _, id := range filesOf(values[d.Key]) {
				if !slices.Contains(out, id) {
					out = append(out, id)
				}
			}
		}
	}

	return out
}

/*
wantedFiles returns the files each file value set names, by key.
*/
func wantedFiles(defs []models.AiConversationAttribute, fitted map[string]any) map[string][]ids.UUID {
	out := map[string][]ids.UUID{}

	for _, d := range defs {
		if value, ok := fitted[d.Key]; ok && d.DataType == TypeFile && value != nil {
			out[d.Key] = filesOf(value)
		}
	}

	return out
}

/*
filesExist refuses a file value naming an upload the workspace does not
have.
*/
func filesExist(ctx context.Context, q *models.Queries, workspaceID ids.UUID, wanted map[string][]ids.UUID) error {
	problems := map[string]string{}

	for key, want := range wanted {
		rows, err := q.ListAttachmentsByID(ctx, models.ListAttachmentsByIDParams{WorkspaceID: workspaceID, Ids: want})
		if err != nil {
			return fmt.Errorf("list attribute files: %w", err)
		}

		if len(rows) != len(want) {
			problems[key] = ProblemUnknownFile
		}
	}

	if len(problems) > 0 {
		return ErrValueInvalid.With("problems", problems)
	}

	return nil
}
