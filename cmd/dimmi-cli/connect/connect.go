/*
Package connect runs the local page where a person signs in to dimmi
and pastes their source tool's token. It listens on 127.0.0.1 only, at
a path holding a one-time secret, so nothing else on the network and
no other site in the browser can reach it. Keys typed there go straight
into the keychain: the terminal, and any AI agent reading it, only ever
sees the page's address and "connected".
*/
package connect

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/dimmi"
)

//go:embed page.html
var pages embed.FS

var page = template.Must(template.ParseFS(pages, "page.html"))

/*
CallbackPath is where dimmi's sign-in returns. The dimmi CLI's OAuth
client registers http://127.0.0.1/oauth/callback, which matches on any
port.
*/
const CallbackPath = "/oauth/callback"

/*
Option is a choice in the source form, such as a region.
*/
type Option struct {
	Value string
	Label string
}

/*
Setup is what a source shows on the page: how to make a token, the
regions to pick from, and how to check a token works.
*/
type Setup struct {
	Tool    string
	Help    template.HTML
	Regions []Option
	Check   func(ctx context.Context, region, token string) (account string, err error)
	Save    func(region, token string) error
	Saved   func() (account string, ok bool)
}

/*
Server is one connect session.
*/
type Server struct {
	DimmiURL  string
	HTTP      *http.Client
	SaveToken func(dimmi.Token) error
	DimmiDone func() bool
	Source    Setup

	secret string
	origin string

	mu     sync.Mutex
	flow   *dimmi.Flow
	errMsg string
	done   chan struct{}
	closed bool
}

/*
Start listens on a free loopback port and returns the page's address.
*/
func (s *Server) Start(ctx context.Context) (string, func() error, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}

	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}

	s.secret = base64.RawURLEncoding.EncodeToString(b)
	s.origin = "http://" + ln.Addr().String()
	s.done = make(chan struct{})

	if s.HTTP == nil {
		s.HTTP = &http.Client{Timeout: 30 * time.Second}
	}

	mux := http.NewServeMux()
	base := "/" + s.secret
	mux.HandleFunc("GET "+base+"/{$}", s.index)
	mux.HandleFunc("GET "+base+"/dimmi", s.startDimmi)
	mux.HandleFunc("POST "+base+"/source", s.saveSource)
	mux.HandleFunc("POST "+base+"/finish", s.finish)
	mux.HandleFunc("GET "+CallbackPath, s.callback)

	srv := &http.Server{Handler: secure(mux), ReadHeaderTimeout: 10 * time.Second}

	go func() { _ = srv.Serve(ln) }()

	stop := func() error {
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		return srv.Shutdown(sctx)
	}

	go func() {
		<-ctx.Done()
		_ = stop()
	}()

	return s.origin + base + "/", stop, nil
}

/*
Done is closed when the person finishes on the page.
*/
func (s *Server) Done() <-chan struct{} { return s.done }

/*
secure sets headers that keep the page out of frames and caches.
*/
func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self' https:; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

type view struct {
	Base       string
	DimmiURL   string
	DimmiDone  bool
	Tool       string
	Help       template.HTML
	Regions    []Option
	SourceDone bool
	Account    string
	Error      string
	AllDone    bool
	Finished   bool
}

func (s *Server) render(w http.ResponseWriter, finished bool) {
	s.mu.Lock()
	msg := s.errMsg
	s.errMsg = ""
	s.mu.Unlock()

	account, sourceDone := "", false
	if s.Source.Saved != nil {
		account, sourceDone = s.Source.Saved()
	}

	dimmiDone := s.DimmiDone()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Execute(w, view{
		Base: "/" + s.secret, DimmiURL: s.DimmiURL, DimmiDone: dimmiDone, Tool: s.Source.Tool, Help: s.Source.Help,
		Regions: s.Source.Regions, SourceDone: sourceDone, Account: account, Error: msg,
		AllDone: dimmiDone && (s.Source.Tool == "" || sourceDone), Finished: finished,
	})
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) { s.render(w, false) }

func (s *Server) fail(msg string) {
	s.mu.Lock()
	s.errMsg = msg
	s.mu.Unlock()
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/"+s.secret+"/", http.StatusSeeOther)
}

/*
sameOrigin refuses form posts from any other page.
*/
func (s *Server) sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")

	return o == "" || o == s.origin
}

func (s *Server) startDimmi(w http.ResponseWriter, r *http.Request) {
	meta, err := dimmi.Discover(r.Context(), s.HTTP, s.DimmiURL)
	if err != nil {
		s.fail("Couldn't reach dimmi at " + s.DimmiURL + ". Check the address and your connection.")
		s.home(w, r)

		return
	}

	flow, err := dimmi.NewFlow(meta, dimmi.ClientID, s.origin+CallbackPath)
	if err != nil {
		s.fail(err.Error())
		s.home(w, r)

		return
	}

	s.mu.Lock()
	s.flow = flow
	s.mu.Unlock()

	http.Redirect(w, r, flow.AuthorizeURL(), http.StatusFound)
}

func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	flow := s.flow
	s.flow = nil
	s.mu.Unlock()

	if flow == nil {
		http.Error(w, "This sign-in has expired. Start again from the connect page.", http.StatusBadRequest)

		return
	}

	tok, err := flow.Exchange(r.Context(), s.HTTP, s.DimmiURL, r.URL.Query())
	if err == nil {
		err = s.SaveToken(tok)
	}

	if err != nil {
		s.fail("dimmi sign-in didn't finish: " + err.Error())
	}

	s.home(w, r)
}

func (s *Server) saveSource(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) || s.Source.Check == nil {
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)

		return
	}

	region := r.PostForm.Get("region")
	token := strings.TrimSpace(r.PostForm.Get("token"))

	if token == "" {
		s.fail("Paste the access token first.")
		s.home(w, r)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if _, err := s.Source.Check(ctx, region, token); err != nil {
		s.fail(fmt.Sprintf("%s didn't accept that token: %v", s.Source.Tool, err))
		s.home(w, r)

		return
	}

	if err := s.Source.Save(region, token); err != nil {
		s.fail("Couldn't save the token: " + err.Error())
	}

	s.home(w, r)
}

func (s *Server) finish(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	s.render(w, true)

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		close(s.done)
	}
}

/*
Valid reports whether u is a URL the browser may be sent to.
*/
func Valid(u string) bool {
	p, err := url.Parse(u)

	return err == nil && (p.Scheme == "http" || p.Scheme == "https") && p.Host != ""
}
