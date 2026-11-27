/*
|--------------------------------------------------------------------------
| Theme Preview Server
|--------------------------------------------------------------------------
|
| `dimmi theme preview` serves the help center from this computer, in
| the theme being written. Each page is built by dimmi with the help
| center's live content, as a signed-out reader gets it, then rendered
| here with the same Go renderer dimmi uses, so what the preview shows
| is what readers will see. Saving a file reloads the open pages.
|
| The help center's own files, such as the platform's stylesheet and
| uploaded images, are fetched from dimmi. Forms do not post from a
| preview.
|
*/

package themepreview

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/liquid"

	"github.com/dimmihq/dimmi/cli/dimmi"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
Pages builds a help center's pages for a preview.
*/
type Pages interface {
	/*
		ThemePreview builds the page at the path of the help center.
	*/
	ThemePreview(ctx context.Context, helpCenter, path string) (dimmi.Preview, error)
}

/*
Server is the preview: the folder being written, the help center its
pages come from and where that help center's dimmi answers.
*/
type Server struct {
	Folder     *Folder
	Pages      Pages
	HelpCenter dimmi.HelpCenter
	DimmiURL   string
	Lang       *i18n.Translator

	mu      sync.Mutex
	waiters []chan struct{}
}

/*
base is the path the help center's pages live under in the preview,
the same as on dimmi's own host.
*/
func (s *Server) base() string { return "/hc/" + s.HelpCenter.Subdomain }

/*
Changed wakes every open page to reload.
*/
func (s *Server) Changed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.waiters {
		close(w)
	}
	s.waiters = nil
}

/*
ServeHTTP answers the preview's requests.
*/
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/":
		http.Redirect(w, r, s.base(), http.StatusFound)
	case r.URL.Path == "/__dimmi/reload":
		s.reload(w, r)
	case strings.HasPrefix(r.URL.Path, s.base()+"/theme-assets/"):
		s.asset(w, r)
	case r.Method != http.MethodGet && r.Method != http.MethodHead:
		s.message(w, http.StatusOK, "Forms don't post from a preview", "Publish the theme to try its forms.")
	case strings.HasPrefix(r.URL.Path, s.base()):
		s.page(w, r)
	default:
		s.proxy(w, r)
	}
}

/*
reload holds a page's request open until the theme changes, then
answers, and the page reloads itself.
*/
func (s *Server) reload(w http.ResponseWriter, r *http.Request) {
	wait := make(chan struct{})
	s.mu.Lock()
	s.waiters = append(s.waiters, wait)
	s.mu.Unlock()
	select {
	case <-wait:
		w.WriteHeader(http.StatusNoContent)
	case <-time.After(55 * time.Second):
		w.WriteHeader(http.StatusAccepted)
	case <-r.Context().Done():
	}
}

/*
page builds the page on dimmi and renders it in the folder's theme.
*/
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	theme, problems, _ := s.Folder.State()
	if len(problems) > 0 || theme == nil {
		s.problems(w, problems)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, s.base())
	if path == "" {
		path = "/"
	}
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	p, err := s.Pages.ThemePreview(r.Context(), s.HelpCenter.ID, path)
	if err != nil {
		s.message(w, http.StatusBadGateway, "dimmi could not build this page", err.Error())
		return
	}
	if p.Location != "" {
		http.Redirect(w, r, p.Location, http.StatusFound)
		return
	}
	if p.Template == "" {
		s.proxy(w, r)
		return
	}
	s.render(w, r, theme, p)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, theme *themes.Theme, p dimmi.Preview) {
	globals, _ := liquid.Decode(p.Variables).(map[string]any)
	if globals == nil {
		globals = map[string]any{}
	}
	// The page comes with the live theme's settings; the folder's theme reads its
	// own, from its theme.json defaults and any value the team set.
	values, _ := globals["settings"].(map[string]any)
	globals["settings"] = liquid.Value(theme.Manifest.SettingValues(values))
	locale := "en"
	if l, ok := globals["locale"].(map[string]any); ok {
		if code, ok := l["code"].(string); ok {
			locale = code
		}
	}
	env := helpcenter.PreviewEnv(r.Context(), s.Lang, locale, s.base())
	env.Scripts = true
	env.AssetURL = func(t *themes.Theme, a themes.Asset) string {
		owner := "local"
		if !t.Custom() {
			owner = "default"
		}
		return s.base() + "/theme-assets/" + owner + "/" + a.Hash + "/" + a.Path
	}
	var out strings.Builder
	err := theme.Render(r.Context(), &out, themes.Page{Name: p.Template, Globals: globals, Env: env})
	if err != nil {
		s.message(w, http.StatusInternalServerError, "The theme could not render "+p.Template, err.Error())
		return
	}
	s.write(w, max(p.Status, http.StatusOK), out.String())
}

/*
asset serves an asset of the folder's theme, or of the default theme.
*/
func (s *Server) asset(w http.ResponseWriter, r *http.Request) {
	theme, _, _ := s.Folder.State()
	rest := strings.TrimPrefix(r.URL.Path, s.base()+"/theme-assets/")
	parts := strings.SplitN(rest, "/", 3)
	if theme == nil || len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	a, _, ok := theme.Asset(parts[2])
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", a.Type)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(a.Body)
}

/*
proxy fetches what the preview does not render from dimmi itself, such
as the platform's stylesheet and the help center's images.
*/
func (s *Server) proxy(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(s.DimmiURL)
	if err != nil {
		http.Error(w, "bad dimmi address", http.StatusBadGateway)
		return
	}
	p := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)
		pr.Out.Header.Del("Cookie")
	}}
	p.ServeHTTP(w, r)
}

func (s *Server) problems(w http.ResponseWriter, problems []themes.Problem) {
	var b strings.Builder
	for _, p := range problems {
		b.WriteString(p.String() + "\n")
	}
	if b.Len() == 0 {
		b.WriteString("The theme has not loaded yet.")
	}
	s.message(w, http.StatusOK, "The theme has problems", b.String())
}

func (s *Server) message(w http.ResponseWriter, status int, title, detail string) {
	page := `<!DOCTYPE html><html><head><meta charset="utf-8"><title>` + html.EscapeString(title) +
		`</title><style>body{font:15px/1.5 system-ui,sans-serif;max-width:46rem;margin:4rem auto;padding:0 1rem}` +
		`pre{white-space:pre-wrap;background:#f4f4f5;padding:1rem;border-radius:.5rem}</style></head><body><h1>` +
		html.EscapeString(title) + `</h1><pre>` + html.EscapeString(detail) + `</pre></body></html>`
	s.write(w, status, page)
}

/*
reloader is the script every preview page carries: it waits for the
theme to change, then reloads the page.
*/
const reloader = `<script>(function w(){fetch("/__dimmi/reload").then(function(r){` +
	`if(r.status===204){location.reload()}else{w()}}).catch(function(){setTimeout(w,1000)})})()</script>`

func (s *Server) write(w http.ResponseWriter, status int, page string) {
	if i := strings.LastIndex(page, "</body>"); i >= 0 {
		page = page[:i] + reloader + page[i:]
	} else {
		page += reloader
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, page)
}

/*
Listen serves the preview on the address until the context ends, and
returns the address it serves at once it listens.
*/
func (s *Server) Listen(ctx context.Context, addr string, ready func(address string)) error {
	srv := &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 10 * time.Second}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	ready("http://" + ln.Addr().String() + s.base())
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
