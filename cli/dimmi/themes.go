/*
|--------------------------------------------------------------------------
| Help Center Themes
|--------------------------------------------------------------------------
|
| What `dimmi theme` asks dimmi for: the workspace's help centers, a
| theme's zip, a page of a help center built for a preview, and
| uploading a theme and making it live.
|
*/

package dimmi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/url"
)

/*
HelpCenter is one of the workspace's help centers.
*/
type HelpCenter struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
	IsDefault bool   `json:"is_default"`
}

/*
HelpCenters lists the workspace's help centers.
*/
func (c *Client) HelpCenters(ctx context.Context) ([]HelpCenter, error) {
	var out struct {
		Data []HelpCenter `json:"data"`
	}
	err := c.do(ctx, call{method: http.MethodGet, path: "/help-centers"}, &out)
	return out.Data, err
}

/*
Theme is a version of a help center's theme.
*/
type Theme struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Name    string `json:"name"`
	Live    bool   `json:"live"`
}

/*
ThemeArchive downloads a theme version's zip, or the default theme's
for "default".
*/
func (c *Client) ThemeArchive(ctx context.Context, helpCenter, theme string) ([]byte, error) {
	var out rawBody
	err := c.do(ctx, call{
		method: http.MethodGet,
		path:   "/help-centers/" + url.PathEscape(helpCenter) + "/themes/" + url.PathEscape(theme) + "/archive",
	}, &out)
	return out, err
}

/*
rawBody keeps an answer's bytes as they came, such as a zip.
*/
type rawBody []byte

/*
UploadTheme sends a theme's zip as the help center's next version. A
theme with problems comes back as an *Error listing them.
*/
func (c *Client) UploadTheme(ctx context.Context, helpCenter string, archive []byte) (Theme, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "theme.zip")
	if err != nil {
		return Theme{}, err
	}
	if _, err := part.Write(archive); err != nil {
		return Theme{}, err
	}
	if err := form.Close(); err != nil {
		return Theme{}, err
	}
	var out struct {
		Data Theme `json:"data"`
	}
	err = c.do(ctx, call{
		method: http.MethodPost, path: "/help-centers/" + url.PathEscape(helpCenter) + "/themes",
		body: bytes.NewReader(body.Bytes()), contentType: form.FormDataContentType(),
	}, &out)
	return out.Data, err
}

/*
PublishTheme makes a theme version the one the help center's pages
wear, or the default theme for an empty id.
*/
func (c *Client) PublishTheme(ctx context.Context, helpCenter, theme string) error {
	body := map[string]any{"theme_id": nil}
	if theme != "" {
		body["theme_id"] = theme
	}
	return c.json(ctx, call{
		method: http.MethodPatch, path: "/help-centers/" + url.PathEscape(helpCenter) + "/live-theme",
	}, body, nil)
}

/*
Preview is a help center page built for a preview: what it answered,
and the template and variables it would have rendered, with HTML and
times written as {"$html": ...} and {"$time": ...}.
*/
type Preview struct {
	Status    int            `json:"status"`
	Location  string         `json:"location"`
	Template  string         `json:"template"`
	Variables map[string]any `json:"variables"`
}

/*
ThemePreview builds the page at the path of the help center.
*/
func (c *Client) ThemePreview(ctx context.Context, helpCenter, path string) (Preview, error) {
	var out struct {
		Data Preview `json:"data"`
	}
	err := c.do(ctx, call{
		method: http.MethodGet,
		path:   "/help-centers/" + url.PathEscape(helpCenter) + "/theme-preview?" + url.Values{"path": {path}}.Encode(),
	}, &out)
	return out.Data, err
}
