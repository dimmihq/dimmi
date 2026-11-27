/*
|--------------------------------------------------------------------------
| Help Center Themes On The Page
|--------------------------------------------------------------------------
|
| A help center's pages wear the theme its team published, else the
| default. What a team's theme may do depends on where the page is
| served. On the help center's own domain, a subdomain of the help
| domain or a custom one, its scripts run and its pages may load from
| the outside origins its theme.json names. On the app's own host, under
| /hc/, its scripts never run, its forms may post only to its own help
| center, and on an install with a help domain it is not worn at all:
| a page on the app's host carries the app's authority, so the team's
| own markup stays on the team's own address.
|
| A theme's assets are served here, never by a CDN, at an address that
| carries their hash, so browsers keep them forever and a new version
| is a new address.
|
*/

package web

import (
	"encoding/base64"
	"path"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
themeAssetsPath is where a theme's assets are served under a help
center's base.
*/
const themeAssetsPath = "/theme-assets/"

/*
defaultThemeKey names the default theme in its assets' addresses.
*/
const defaultThemeKey = "default"

/*
wearsTeamTheme says whether the page wears the team's own theme on this
host: always on the help center's own domain, and under /hc/ on the
app's host only on an install without a help domain.
*/
func (c *HelpCenterController) wearsTeamTheme(r *fhttp.Request) bool {
	return hostedHere(r.Context()) || c.Addresses.BaseDomain == ""
}

/*
theme dresses the page in the help center's theme. Root is the base
without the language prefix, where its assets are served.
*/
func (c *HelpCenterController) theme(r *fhttp.Request, center *helpcenter.Center, h models.HelpCenter,
	root string,
) error {
	center.Theming.FormToken = c.formToken(r)
	center.Theming.Scripts = hostedHere(r.Context())
	center.Theming.Assets = c.styles()
	def, err := c.Themes.Default()
	if err != nil {
		return err
	}
	center.Theming.Theme = def
	key := defaultThemeKey
	if h.ThemeID != nil && c.wearsTeamTheme(r) {
		live, err := c.Themes.Live(r.Context(), h.WorkspaceID, *h.ThemeID, h.CacheVersion)
		if err != nil {
			return err
		}
		center.Theming.Theme, center.Theming.Settings = live.Theme, live.Settings
		key = models.PublicID(models.PrefixHelpCenterTheme, *h.ThemeID)
	}
	center.Theming.AssetURL = func(t *themes.Theme, a themes.Asset) string {
		owner := key
		if !t.Custom() {
			owner = defaultThemeKey
		}
		return root + themeAssetsPath + owner + "/" + a.Hash + "/" + a.Path
	}
	return nil
}

/*
ThemeAsset serves one of a theme's assets with headers that keep it
forever. The address names the theme and the asset's hash; a hash that
no longer matches answers not found, so a stale address never serves
other bytes. On the app's own host a theme's scripts are never served.
*/
func (c *HelpCenterController) ThemeAsset(w fhttp.ResponseWriter, r *fhttp.Request) {
	rest := r.URL.Path[strings.Index(r.URL.Path, themeAssetsPath)+len(themeAssetsPath):]
	key, rest, _ := strings.Cut(rest, "/")
	hash, name, _ := strings.Cut(rest, "/")
	theme, ok := c.assetTheme(r, key)
	if !ok {
		w.WriteHeader(fhttp.StatusNotFound)
		return
	}
	asset, owner, found := theme.Asset(name)
	ext := strings.ToLower(path.Ext(name))
	scripted := ext == ".js" || ext == ".mjs"
	if !found || asset.Hash != hash || (owner.Custom() && scripted && !hostedHere(r.Context())) {
		w.WriteHeader(fhttp.StatusNotFound)
		return
	}
	h := w.Header()
	h.Set("Content-Type", asset.Type)
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("ETag", `"`+asset.Hash+`"`)
	if r.Header.Get("If-None-Match") == `"`+asset.Hash+`"` {
		w.WriteHeader(fhttp.StatusNotModified)
		return
	}
	w.WriteHeader(fhttp.StatusOK)
	if r.Method != fhttp.MethodHead {
		_, _ = w.Write(asset.Body)
	}
}

/*
assetTheme finds the theme an asset address names: the default, or the
version the help center wears, or wore, on this host.
*/
func (c *HelpCenterController) assetTheme(r *fhttp.Request, key string) (*themes.Theme, bool) {
	if key == defaultThemeKey {
		def, err := c.Themes.Default()
		return def, err == nil
	}
	if c.Public == nil || c.IDs == nil || !c.wearsTeamTheme(r) {
		return nil, false
	}
	snap, err := c.Public.LoadIn(r.Context(), r.PathValue("center"), "")
	if err != nil {
		return nil, false
	}
	id, err := c.IDs.Decode(string(models.PrefixHelpCenterTheme), key)
	if err != nil {
		return nil, false
	}
	live, err := c.Themes.Live(r.Context(), snap.HelpCenter.WorkspaceID, id, snap.HelpCenter.CacheVersion)
	if err != nil {
		return nil, false
	}
	return live.Theme, true
}

/*
formToken is the anti-forgery token the page's forms carry. It is
bound to the host, the help center and the reader's sign-in, so a page
of another help center, even a sibling on the help domain, can neither
read nor guess the one that acts as this reader.
*/
func (c *HelpCenterController) formToken(r *fhttp.Request) string {
	reader := ""
	if cookie, err := r.Cookie(readerCookie); err == nil {
		reader = cookie.Value
	}
	if cookie, err := r.Cookie(visitorCookie); err == nil {
		reader += "\x00" + cookie.Value
	}
	message := "help-form\x00" + strings.ToLower(r.Host) + "\x00" + r.PathValue("center") + "\x00" + reader
	return base64.RawURLEncoding.EncodeToString(hashing.HMAC(c.FormKey, []byte(message)))
}

/*
FormTokenHeader carries the anti-forgery token for a script that posts
without the form, which another site cannot set without the help center
allowing it.
*/
const FormTokenHeader = "X-Form-Token" //nolint:gosec // a header name, not a secret

/*
GuardForm refuses a form posted to the help center without its page's
token, in the form's field or the X-Form-Token header, or sent by a
browser from another site, with 403.
*/
func (c *HelpCenterController) GuardForm(next fhttp.HandlerFunc) fhttp.HandlerFunc {
	return func(w fhttp.ResponseWriter, r *fhttp.Request) {
		site := r.Header.Get("Sec-Fetch-Site")
		crossSite := site != "" && site != "same-origin" && site != "none"
		r.Body = fhttp.MaxBytesReader(w, r.Body, maxFormBytes)
		sent := r.Header.Get(FormTokenHeader)
		if sent == "" {
			sent = r.PostFormValue(helpcenter.FormTokenField)
		}
		if crossSite || !hashing.Equal([]byte(sent), []byte(c.formToken(r))) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(fhttp.StatusForbidden)
			_, _ = w.Write([]byte("This page expired. Go back, reload it and try again.\n"))
			return
		}
		next(w, r)
	}
}

/*
maxFormBytes bounds a form posted to the help center.
*/
const maxFormBytes = 1 << 20

/*
themePolicy loosens the page's content security policy for a team's
theme on its own domain, adding the outside origins its theme.json
names, and keeps forms on the app's host posting to their own help
center only.
*/
func (c *HelpCenterController) themePolicy(w fhttp.ResponseWriter, r *fhttp.Request, center helpcenter.Center) {
	t := center.Theming.Theme
	if t == nil || !t.Custom() {
		return
	}
	policy := w.Header().Get("Content-Security-Policy")
	if policy == "" {
		return
	}
	if !hostedHere(r.Context()) {
		own := origin(r) + "/hc/" + r.PathValue("center") + "/"
		w.Header().Set("Content-Security-Policy", replaceDirective(policy, "form-action", "form-action "+own))
		return
	}
	o := t.Manifest.Origins
	for directive, extra := range map[string][]string{
		"script-src": o.Scripts, "style-src": o.Styles, "img-src": o.Images, "font-src": o.Fonts,
		"connect-src": o.Connect, "frame-src": o.Frames, "media-src": o.Media,
	} {
		if len(extra) > 0 {
			policy = appendDirective(policy, directive, extra)
		}
	}
	w.Header().Set("Content-Security-Policy", policy)
}

func replaceDirective(policy, name, with string) string {
	parts := strings.Split(policy, ";")
	for i, p := range parts {
		if fields := strings.Fields(p); len(fields) > 0 && fields[0] == name {
			parts[i] = " " + with
			return strings.TrimSpace(strings.Join(parts, ";"))
		}
	}
	return policy + "; " + with
}

func appendDirective(policy, name string, origins []string) string {
	parts := strings.Split(policy, ";")
	for i, p := range parts {
		if fields := strings.Fields(p); len(fields) > 0 && fields[0] == name {
			parts[i] = p + " " + strings.Join(origins, " ")
			return strings.Join(parts, ";")
		}
	}
	return policy + "; " + name + " " + strings.Join(origins, " ")
}
