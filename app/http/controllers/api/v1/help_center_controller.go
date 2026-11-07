/*
|--------------------------------------------------------------------------
| Help Center Controller
|--------------------------------------------------------------------------
|
| A workspace's help centers and their collections: listed, read and
| changed. Listing makes the default help center the first time.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
HelpCenterController serves help centers and collections.
*/
type HelpCenterController struct {
	HelpCenters *helpcenters.HelpCenters
	Collections *helpcenters.Collections
	Addresses   helpcenters.Addresses
	IDs         *ids.Registry
}

/*
HelpCentersOutput is the list of help centers.
*/
type HelpCentersOutput struct {
	Body fhttp.Collection[resources.HelpCenter]
}

/*
HelpCenterOutput is one help center.
*/
type HelpCenterOutput struct {
	Body fhttp.Resource[resources.HelpCenter]
}

/*
CollectionsOutput is a help center's collections.
*/
type CollectionsOutput struct {
	Body fhttp.Collection[resources.Collection]
}

/*
CollectionOutput is one collection.
*/
type CollectionOutput struct {
	Body fhttp.Resource[resources.Collection]
}

/*
Index lists the help centers.
*/
func (c *HelpCenterController) Index(ctx context.Context, _ *requests.NoInput) (*HelpCentersOutput, error) {
	rows, err := c.HelpCenters.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	items := make([]resources.HelpCenter, len(rows))
	for i, h := range rows {
		items[i] = resources.NewHelpCenter(h, c.Addresses.Root(h))
	}
	page := pagination.Page[resources.HelpCenter]{Items: items, Limit: 100}
	return &HelpCentersOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers one help center.
*/
func (c *HelpCenterController) Show(ctx context.Context, in *requests.HelpCenterRequest) (*HelpCenterOutput, error) {
	id, err := c.helpCenterID(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	h, err := c.HelpCenters.Show(ctx, membership(ctx), id)
	return c.one(h, err)
}

/*
Update changes a help center.
*/
func (c *HelpCenterController) Update(ctx context.Context, in *requests.UpdateHelpCenterRequest,
) (*HelpCenterOutput, error) {
	id, err := c.helpCenterID(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	b := in.Body
	change := helpcenters.HelpCenterChange{
		Membership: membership(ctx), ID: id, Name: b.Name, Subdomain: b.Subdomain, Locale: b.Locale, Status: b.Status,
		Indexing: b.IndexingEnabled, AskAI: b.AskAIEnabled,
	}
	if b.Settings != nil {
		change.Settings = new(helpcenters.HelpCenterSettings(*b.Settings))
	}
	h, err := c.HelpCenters.Update(ctx, change)
	return c.one(h, err)
}

func (c *HelpCenterController) one(h models.HelpCenter, err error) (*HelpCenterOutput, error) {
	if err != nil {
		return nil, err
	}
	return &HelpCenterOutput{Body: fhttp.NewResource(resources.NewHelpCenter(h, c.Addresses.Root(h)))}, nil
}

/*
IndexCollections lists a help center's collections.
*/
func (c *HelpCenterController) IndexCollections(ctx context.Context, in *requests.HelpCenterRequest,
) (*CollectionsOutput, error) {
	id, err := c.helpCenterID(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	rows, err := c.Collections.List(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.Collection]{Items: resources.NewCollections(rows), Limit: 500}
	return &CollectionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
StoreCollection creates a collection.
*/
func (c *HelpCenterController) StoreCollection(ctx context.Context, in *requests.StoreCollectionRequest,
) (*CollectionOutput, error) {
	id, err := c.helpCenterID(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	input, err := c.collectionInput(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	input.HelpCenterID = id
	col, err := c.Collections.Create(ctx, input)
	return collectionOutput(col, err)
}

/*
UpdateCollection changes a collection.
*/
func (c *HelpCenterController) UpdateCollection(ctx context.Context, in *requests.UpdateCollectionRequest,
) (*CollectionOutput, error) {
	id, err := c.decode(models.PrefixCollection, in.Collection)
	if err != nil {
		return nil, err
	}
	input, err := c.collectionInput(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	input.ID = id
	col, err := c.Collections.Update(ctx, input)
	return collectionOutput(col, err)
}

/*
DestroyCollection moves a collection to the trash.
*/
func (c *HelpCenterController) DestroyCollection(ctx context.Context, in *requests.CollectionRequest,
) (*struct{}, error) {
	id, err := c.decode(models.PrefixCollection, in.Collection)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Collections.Delete(ctx, membership(ctx), id)
}

/*
collectionInput reads a collection body; an empty parent_id clears the
parent.
*/
func (c *HelpCenterController) collectionInput(ctx context.Context, b requests.CollectionBody,
) (helpcenters.CollectionInput, error) {
	out := helpcenters.CollectionInput{
		Membership: membership(ctx), Name: b.Name, Slug: b.Slug, Description: b.Description, Icon: b.Icon,
		Position: b.Position,
	}
	if b.ParentID == nil {
		return out, nil
	}
	if *b.ParentID == "" {
		out.ClearParent = true
		return out, nil
	}
	parent, err := c.decode(models.PrefixCollection, *b.ParentID)
	if err != nil {
		return out, helpcenters.ErrCollectionNotFound
	}
	out.ParentID = &parent
	return out, nil
}

func collectionOutput(col models.Collection, err error) (*CollectionOutput, error) {
	if err != nil {
		return nil, err
	}
	return &CollectionOutput{Body: fhttp.NewResource(resources.NewCollection(col))}, nil
}

func (c *HelpCenterController) helpCenterID(public string) (ids.UUID, error) {
	return c.decode(models.PrefixHelpCenter, public)
}

func (c *HelpCenterController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
