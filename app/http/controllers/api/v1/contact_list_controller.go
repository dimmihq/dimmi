/*
|--------------------------------------------------------------------------
| Contact List Controller
|--------------------------------------------------------------------------
|
| POST /contact-lists tags every customer a CSV list names and answers
| how many it matched; POST /audiences/{audience}/freezes tags everyone a
| saved audience matches now.
|
*/

package v1

import (
	"context"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/actions/contactlists"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactListController tags customers in bulk.
*/
type ContactListController struct {
	Lists     *contactlists.Lists
	Audiences *audiences.Audiences
	IDs       *ids.Registry
}

/*
TaggedList is what tagging in bulk did.
*/
type TaggedList struct {
	Object    string   `json:"object"`
	TagID     string   `json:"tag_id"`
	TagName   string   `json:"tag_name"`
	Rows      int      `json:"rows,omitempty"`
	Matched   int      `json:"matched"`
	Tagged    int64    `json:"tagged"`
	Unmatched int      `json:"unmatched"`
	Missing   []string `json:"unmatched_sample"`
}

/*
TaggedListOutput is what tagging in bulk did.
*/
type TaggedListOutput struct {
	Body fhttp.Resource[TaggedList]
}

/*
Import tags the customers an uploaded list names.
*/
func (c *ContactListController) Import(ctx context.Context, in *requests.StoreContactListRequest,
) (*TaggedListOutput, error) {
	defer func() { _ = in.RawBody.RemoveAll() }()

	uploads := in.RawBody.File["file"]
	if len(uploads) == 0 {
		return nil, contactlists.ErrUnreadable
	}

	file, err := uploads[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded list: %w", err)
	}

	defer func() { _ = file.Close() }()

	r, err := c.Lists.Import(ctx, contactlists.ImportInput{Membership: membership(ctx), TagName: in.Tag(), File: file})
	if err != nil {
		return nil, err
	}

	out := TaggedList{
		Object: "contact_list", TagID: models.PublicID(models.PrefixTag, r.Tag.ID), TagName: r.Tag.Name,
		Rows: r.Rows, Matched: r.Matched, Tagged: r.Tagged, Unmatched: r.Unmatched, Missing: nonEmpty(r.Missing),
	}

	return &TaggedListOutput{Body: fhttp.NewResource(out)}, nil
}

/*
Freeze tags everyone a saved audience matches now.
*/
func (c *ContactListController) Freeze(ctx context.Context, in *requests.FreezeAudienceRequest,
) (*TaggedListOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixAudience), in.Audience)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	f, err := c.Audiences.Freeze(ctx, audiences.FreezeInput{Membership: membership(ctx), ID: id, TagName: in.Body.Tag})
	if err != nil {
		return nil, err
	}

	out := TaggedList{
		Object: "audience_freeze", TagID: models.PublicID(models.PrefixTag, f.Tag.ID), TagName: f.Tag.Name,
		Matched: f.Matched, Tagged: f.Tagged, Missing: []string{},
	}

	return &TaggedListOutput{Body: fhttp.NewResource(out)}, nil
}

func nonEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}
