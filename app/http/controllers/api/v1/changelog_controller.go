/*
|--------------------------------------------------------------------------
| Changelog Controller
|--------------------------------------------------------------------------
|
| A help center's changelog entries: listed a numbered page at a time,
| written, saved, published now or later, unpublished and deleted.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
changelogPage is how many entries a page lists.
*/
const changelogPage = 25

/*
ChangelogController serves changelog entries.
*/
type ChangelogController struct {
	Entries *changelog.Entries
	IDs     *ids.Registry
}

/*
ChangelogEntriesOutput is a page of entries with how many are in each
state.
*/
type ChangelogEntriesOutput struct {
	Body struct {
		fhttp.Collection[resources.ChangelogEntry]
		Counts map[string]int32 `json:"counts"`
	}
}

/*
ChangelogEntryOutput is one entry with its body.
*/
type ChangelogEntryOutput struct {
	Body fhttp.Resource[resources.ChangelogEntry]
}

/*
Index lists a page of a changelog's entries.
*/
func (c *ChangelogController) Index(ctx context.Context, in *requests.IndexChangelogEntriesRequest,
) (*ChangelogEntriesOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	f := changelog.EntryFilter{HelpCenterID: hc, Status: in.Status, Query: in.Query, Limit: changelogPage}
	offset, _, err := numberedPage(ctx, changelogPage)
	if err != nil {
		return nil, err
	}
	f.Offset = offset
	if in.Label != "" {
		label, err := c.decode(models.PrefixChangelogLabel, in.Label)
		if err != nil {
			return nil, err
		}
		f.LabelID = &label
	}
	page, err := c.Entries.List(ctx, membership(ctx), f)
	if err != nil {
		return nil, err
	}
	out := &ChangelogEntriesOutput{}
	out.Body.Collection = fhttp.NewCollection(pagination.Page[resources.ChangelogEntry]{
		Items: resources.NewChangelogEntries(page.Entries), Limit: changelogPage, Total: &page.Total,
	})
	out.Body.Counts = page.ByStatus
	return out, nil
}

/*
Store creates a draft.
*/
func (c *ChangelogController) Store(ctx context.Context, in *requests.StoreChangelogEntryRequest,
) (*ChangelogEntryOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	input, err := c.input(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	input.HelpCenterID = hc
	e, err := c.Entries.Create(ctx, input)
	return entryOutput(e, err)
}

/*
Show answers one entry with its body.
*/
func (c *ChangelogController) Show(ctx context.Context, in *requests.ChangelogEntryRequest,
) (*ChangelogEntryOutput, error) {
	id, err := c.decode(models.PrefixChangelogEntry, in.Entry)
	if err != nil {
		return nil, err
	}
	e, err := c.Entries.Show(ctx, membership(ctx), id)
	return entryOutput(e, err)
}

/*
Update saves the working copy.
*/
func (c *ChangelogController) Update(ctx context.Context, in *requests.UpdateChangelogEntryRequest,
) (*ChangelogEntryOutput, error) {
	id, err := c.decode(models.PrefixChangelogEntry, in.Entry)
	if err != nil {
		return nil, err
	}
	input, err := c.input(ctx, in.Body.ChangelogEntryBody)
	if err != nil {
		return nil, err
	}
	input.ID, input.LockVersion = id, in.Body.LockVersion
	e, err := c.Entries.Update(ctx, input)
	return entryOutput(e, err)
}

/*
Publish makes the working copy live, or schedules it.
*/
func (c *ChangelogController) Publish(ctx context.Context, in *requests.PublishChangelogEntryRequest,
) (*ChangelogEntryOutput, error) {
	id, err := c.decode(models.PrefixChangelogEntry, in.Entry)
	if err != nil {
		return nil, err
	}
	e, err := c.Entries.Publish(ctx, membership(ctx), id, changelog.PublishInput{
		Notify: in.Body.Notify, At: in.Body.PublishedAt, ScheduledFor: in.Body.ScheduledFor,
	})
	return entryOutput(e, err)
}

/*
Unpublish takes an entry back to a draft.
*/
func (c *ChangelogController) Unpublish(ctx context.Context, in *requests.ChangelogEntryRequest,
) (*ChangelogEntryOutput, error) {
	id, err := c.decode(models.PrefixChangelogEntry, in.Entry)
	if err != nil {
		return nil, err
	}
	e, err := c.Entries.Unpublish(ctx, membership(ctx), id)
	return entryOutput(e, err)
}

/*
Destroy deletes an entry.
*/
func (c *ChangelogController) Destroy(ctx context.Context, in *requests.ChangelogEntryRequest) (*struct{}, error) {
	id, err := c.decode(models.PrefixChangelogEntry, in.Entry)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Entries.Delete(ctx, membership(ctx), id)
}

/*
input reads an entry's fields, decoding the labels and authors named.
*/
func (c *ChangelogController) input(ctx context.Context, b requests.ChangelogEntryBody,
) (changelog.EntryInput, error) {
	out := changelog.EntryInput{
		Membership: membership(ctx), Title: b.Title, Slug: b.Slug, Summary: b.Summary, Body: b.BodyMarkdown,
		CoverURL: b.CoverURL,
	}
	var err error
	if out.LabelIDs, err = c.decodeAll(models.PrefixChangelogLabel, b.LabelIDs); err != nil {
		return out, err
	}
	out.AuthorIDs, err = c.decodeAll(models.PrefixMember, b.AuthorIDs)
	return out, err
}

func (c *ChangelogController) decodeAll(prefix models.Prefix, public *[]string) (*[]ids.UUID, error) {
	if public == nil {
		return nil, nil
	}
	out := make([]ids.UUID, len(*public))
	for i, p := range *public {
		id, err := c.decode(prefix, p)
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return &out, nil
}

func entryOutput(e changelog.Entry, err error) (*ChangelogEntryOutput, error) {
	if err != nil {
		return nil, err
	}
	return &ChangelogEntryOutput{Body: fhttp.NewResource(resources.NewChangelogEntry(e))}, nil
}

func (c *ChangelogController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
