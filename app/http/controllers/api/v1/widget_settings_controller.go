/*
|--------------------------------------------------------------------------
| Widget Settings Controller
|--------------------------------------------------------------------------
|
| The dashboard's widgets: GET and POST /widget-settings, and GET, PATCH and
| DELETE /widget-settings/{widget}. Ids of other workspaces' widgets are
| simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxWidgetsListed is the most widgets a workspace lists, all on one page.
*/
const maxWidgetsListed = 200

/*
WidgetSettingsListOutput is the workspace's widgets.
*/
type WidgetSettingsListOutput struct {
	Body fhttp.Collection[resources.WidgetSettings]
}

/*
WidgetSettingsOutput is one widget.
*/
type WidgetSettingsOutput struct {
	Body fhttp.Resource[resources.WidgetSettings]
}

/*
WidgetSettingsController serves the workspace's widgets. Base is this
install's address.
*/
type WidgetSettingsController struct {
	Read   *widget.ShowWidgetSettings
	Create *widget.CreateWidget
	Change *widget.UpdateWidgetSettings
	Remove *widget.DeleteWidget
	IDs    *ids.Registry
	Base   string
}

/*
Index lists the widgets, oldest first, on one page.
*/
func (c *WidgetSettingsController) Index(ctx context.Context, _ *requests.NoInput) (*WidgetSettingsListOutput, error) {
	rows, err := c.Read.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.WidgetSettings, 0, len(rows))
	for _, row := range rows {
		items = append(items, resources.NewWidgetSettings(row, c.Base))
	}

	page := pagination.Page[resources.WidgetSettings]{Items: items, Limit: maxWidgetsListed}

	return &WidgetSettingsListOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Get answers with one widget.
*/
func (c *WidgetSettingsController) Get(
	ctx context.Context, in *requests.ShowWidgetSettingsRequest,
) (*WidgetSettingsOutput, error) {
	id, err := c.widgetID(in.Widget)
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, id)
}

/*
Store adds a widget, with any settings the body already sends, and answers
with it.
*/
func (c *WidgetSettingsController) Store(
	ctx context.Context, in *requests.StoreWidgetSettingsRequest,
) (*WidgetSettingsOutput, error) {
	change, err := c.change(in.Body, in.RawBody)
	if err != nil {
		return nil, err
	}

	name := ""
	if in.Body.Name != nil {
		name = *in.Body.Name
	}

	created, err := c.Create.Handle(ctx, widget.CreateWidgetInput{
		Membership: membership(ctx), Name: name, InboxID: change.InboxID,
	})
	if err != nil {
		return nil, err
	}

	change.Name, change.InboxID = nil, nil

	_, err = c.Change.Handle(ctx, widget.UpdateWidgetSettingsInput{
		Membership: membership(ctx), WidgetID: created.ID, Change: change,
	})
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, created.ID)
}

/*
Update changes a widget and answers with it as it now is.
*/
func (c *WidgetSettingsController) Update(
	ctx context.Context, in *requests.UpdateWidgetSettingsRequest,
) (*WidgetSettingsOutput, error) {
	id, err := c.widgetID(in.Widget)
	if err != nil {
		return nil, err
	}

	change, err := c.change(in.Body, in.RawBody)
	if err != nil {
		return nil, err
	}

	_, err = c.Change.Handle(ctx, widget.UpdateWidgetSettingsInput{
		Membership: membership(ctx), WidgetID: id, Change: change,
	})
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, id)
}

/*
Destroy removes a widget.
*/
func (c *WidgetSettingsController) Destroy(
	ctx context.Context, in *requests.ShowWidgetSettingsRequest,
) (*EmptyOutput, error) {
	id, err := c.widgetID(in.Widget)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Remove.Handle(ctx, widget.DeleteWidgetInput{Membership: membership(ctx), WidgetID: id})
}

/*
answer reads the widget and shapes it.
*/
func (c *WidgetSettingsController) answer(ctx context.Context, id ids.UUID) (*WidgetSettingsOutput, error) {
	row, err := c.Read.Find(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &WidgetSettingsOutput{Body: fhttp.NewResource(resources.NewWidgetSettings(row, c.Base))}, nil
}

/*
widgetID decodes a widget's public id; anything else is simply not found.
*/
func (c *WidgetSettingsController) widgetID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixWidget), public)
	if err != nil {
		return id, exceptions.ErrNotFound
	}

	return id, nil
}

/*
change decodes the ids the body names and returns the settings change.
*/
func (c *WidgetSettingsController) change(body requests.WidgetSettingsBody, raw []byte) (widget.SettingsChange, error) {
	out := body.Change(raw)

	if body.InboxID != nil {
		id, err := c.IDs.Decode(string(models.PrefixInbox), *body.InboxID)
		if err != nil {
			return out, exceptions.ErrInboxNotFound
		}

		out.InboxID = &id
	}

	if body.HelpCenterID != nil {
		id, err := c.IDs.Decode("hc", *body.HelpCenterID)
		if err != nil {
			return out, exceptions.WidgetField("help_center_id", "This workspace has no such help center.")
		}

		out.HelpCenterID = &id
	}

	return out, nil
}
