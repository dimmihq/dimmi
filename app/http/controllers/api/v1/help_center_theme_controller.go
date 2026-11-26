/*
|--------------------------------------------------------------------------
| Help Center Theme Controller
|--------------------------------------------------------------------------
|
| A help center's themes: uploaded as a zip, each upload a version of its
| own, listed, read, downloaded again, given the team's settings, and
| made live, or the default theme made live again.
|
*/

package v1

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/liquid"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
HelpCenterThemeController serves help centers' themes.
*/
type HelpCenterThemeController struct {
	Themes      *helpcenters.Themes
	HelpCenters *helpcenters.HelpCenters
	Addresses   helpcenters.Addresses
	IDs         *ids.Registry
	Pages       func() (fhttp.Handler, error)
}

/*
ThemePreviewOutput is a page built for a preview.
*/
type ThemePreviewOutput struct {
	Body fhttp.Resource[resources.ThemePreview]
}

/*
previewTime bounds building one page for a preview.
*/
const previewTime = 10 * time.Second

/*
Preview builds the page at a path of the help center as a signed-out
reader gets it and answers with its template and variables, for
`dimmi theme preview` to render with the theme being written.
*/
func (c *HelpCenterThemeController) Preview(ctx context.Context, in *requests.ThemePreviewRequest,
) (*ThemePreviewOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	h, err := c.HelpCenters.Show(ctx, membership(ctx), hc)
	if err != nil {
		return nil, err
	}
	pages, err := c.Pages()
	if err != nil {
		return nil, err
	}
	build, cancel := context.WithTimeout(context.WithoutCancel(ctx), previewTime)
	defer cancel()
	p, err := web.PreviewPage(build, pages, h.Subdomain, cmp.Or(in.Path, "/"))
	if err != nil {
		return nil, err
	}
	out := resources.ThemePreview{Object: "theme_preview", Status: p.Status, Location: p.Location, Template: p.Template}
	if p.Globals != nil {
		out.Variables, _ = liquid.Encode(p.Globals).(map[string]any)
	}
	return &ThemePreviewOutput{Body: fhttp.NewResource(out)}, nil
}

/*
HelpCenterThemesOutput is a help center's theme versions.
*/
type HelpCenterThemesOutput struct {
	Body fhttp.Collection[resources.HelpCenterTheme]
}

/*
HelpCenterThemeOutput is one theme version.
*/
type HelpCenterThemeOutput struct {
	Body fhttp.Resource[resources.HelpCenterTheme]
}

/*
HelpCenterThemeArchiveOutput is a theme's zip.
*/
type HelpCenterThemeArchiveOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               []byte
}

/*
Index lists a help center's theme versions, newest first.
*/
func (c *HelpCenterThemeController) Index(ctx context.Context, in *requests.HelpCenterRequest,
) (*HelpCenterThemesOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	list, err := c.Themes.List(ctx, membership(ctx), hc)
	if err != nil {
		return nil, err
	}
	items := make([]resources.HelpCenterTheme, len(list.Themes))
	for i, t := range list.Themes {
		items[i] = resources.NewHelpCenterTheme(t, list.Live != nil && *list.Live == t.ID)
	}
	page := pagination.Page[resources.HelpCenterTheme]{Items: items, Limit: 100}
	return &HelpCenterThemesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store uploads a theme's zip as the help center's next version. It is
checked whole; a theme with problems is refused with every problem, and
nothing is stored. Storing does not make it live.
*/
func (c *HelpCenterThemeController) Store(ctx context.Context, in *requests.StoreHelpCenterThemeRequest,
) (*HelpCenterThemeOutput, error) {
	defer func() { _ = in.RawBody.RemoveAll() }()
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	uploads := in.RawBody.File["file"]
	if len(uploads) == 0 {
		return nil, helpcenters.ErrThemeArchive
	}
	f, err := uploads[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open theme upload: %w", err)
	}
	defer func() { _ = f.Close() }()
	archive, err := io.ReadAll(io.LimitReader(f, helpcenters.MaxThemeArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read theme upload: %w", err)
	}
	if len(archive) > helpcenters.MaxThemeArchiveBytes {
		return nil, helpcenters.ErrThemeArchive.With("reason", "the zip is larger than 26 MB")
	}
	t, err := c.Themes.Upload(ctx, helpcenters.ThemeUpload{
		Membership: membership(ctx), HelpCenterID: hc, Archive: archive,
	})
	if err != nil {
		return nil, err
	}
	return &HelpCenterThemeOutput{Body: fhttp.NewResource(resources.NewHelpCenterTheme(t, false))}, nil
}

/*
Show answers one theme version.
*/
func (c *HelpCenterThemeController) Show(ctx context.Context, in *requests.HelpCenterThemeRequest,
) (*HelpCenterThemeOutput, error) {
	hc, id, err := c.ids(in.HelpCenter, in.Theme)
	if err != nil {
		return nil, err
	}
	t, err := c.Themes.Show(ctx, membership(ctx), hc, id)
	if err != nil {
		return nil, err
	}
	return c.one(ctx, hc, t)
}

/*
Update stores the team's values for a theme version's settings.
*/
func (c *HelpCenterThemeController) Update(ctx context.Context, in *requests.UpdateHelpCenterThemeRequest,
) (*HelpCenterThemeOutput, error) {
	hc, id, err := c.ids(in.HelpCenter, in.Theme)
	if err != nil {
		return nil, err
	}
	t, err := c.Themes.UpdateSettings(ctx, helpcenters.ThemeSettingsChange{
		Membership: membership(ctx), HelpCenterID: hc, ID: id, Values: in.Body.Settings,
	})
	if err != nil {
		return nil, err
	}
	return c.one(ctx, hc, t)
}

/*
Download answers a theme version, or the default theme, as a zip of its
folder.
*/
func (c *HelpCenterThemeController) Download(ctx context.Context, in *requests.HelpCenterThemeArchiveRequest,
) (*HelpCenterThemeArchiveOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	var id *ids.UUID
	name := "default"
	if in.Theme != "default" {
		decoded, err := c.decode(models.PrefixHelpCenterTheme, in.Theme)
		if err != nil {
			return nil, err
		}
		id, name = &decoded, in.Theme
	}
	archive, err := c.Themes.Archive(ctx, membership(ctx), hc, id)
	if err != nil {
		return nil, err
	}
	return &HelpCenterThemeArchiveOutput{
		ContentType: "application/zip", ContentDisposition: `attachment; filename="theme-` + name + `.zip"`,
		CacheControl: "no-store", Body: archive,
	}, nil
}

/*
UpdateLive makes a theme version the one the help center's pages wear,
or the default theme for a null theme_id, and answers with the help
center.
*/
func (c *HelpCenterThemeController) UpdateLive(ctx context.Context, in *requests.UpdateLiveThemeRequest,
) (*HelpCenterOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	var id *ids.UUID
	if in.Body.ThemeID != nil {
		decoded, err := c.IDs.Decode(string(models.PrefixHelpCenterTheme), *in.Body.ThemeID)
		if err != nil {
			return nil, helpcenters.ErrThemeNotFound.OnField("theme_id")
		}
		id = &decoded
	}
	h, err := c.Themes.Publish(ctx, membership(ctx), hc, id)
	if err != nil {
		return nil, err
	}
	return &HelpCenterOutput{Body: fhttp.NewResource(resources.NewHelpCenter(h, c.Addresses.Root(h)))}, nil
}

func (c *HelpCenterThemeController) one(ctx context.Context, hc ids.UUID, t models.HelpCenterTheme,
) (*HelpCenterThemeOutput, error) {
	list, err := c.Themes.List(ctx, membership(ctx), hc)
	if err != nil {
		return nil, err
	}
	live := list.Live != nil && *list.Live == t.ID
	return &HelpCenterThemeOutput{Body: fhttp.NewResource(resources.NewHelpCenterTheme(t, live))}, nil
}

func (c *HelpCenterThemeController) ids(helpCenter, theme string) (ids.UUID, ids.UUID, error) {
	hc, err := c.decode(models.PrefixHelpCenter, helpCenter)
	if err != nil {
		return hc, hc, err
	}
	id, err := c.decode(models.PrefixHelpCenterTheme, theme)
	return hc, id, err
}

func (c *HelpCenterThemeController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
