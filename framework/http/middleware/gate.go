/*
|--------------------------------------------------------------------------
| Shared Password Gate
|--------------------------------------------------------------------------
|
| A public host that only invited people should use, such as a staging
| environment, sits behind one shared password. A person types it once
| and keeps a cookie; automated checks send a key header instead. Paths
| that other systems must reach, such as signed webhooks, stay open. The
| gate also keeps search engines out: robots.txt disallows everything and
| every response says noindex. It is off unless configured; the host
| name being known is no protection, since certificate logs publish it.
|
*/

package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"html"
	"net/http"
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
Gate paths and names. GatePath receives the password form; GateKeyHeader
carries the key automated checks use.
*/
const (
	GatePath      = "/__gate"
	GateKeyHeader = "X-Gate-Key"
	gateCookie    = "__Host-gate"
	gateRobots    = "User-agent: *\nDisallow: /\n"
)

/*
GateOptions configure the gate. Password is what people type and Key
what automated checks send; either may be empty to disable that way in.
Secret signs the cookie, so changing it or the password signs everyone
out. The cookie is a Secure __Host- cookie, which browsers also keep on
http://localhost. Except lists paths that stay open; one ending in "*"
covers every path starting with the rest.
*/
type GateOptions struct {
	Password string
	Key      string
	Secret   []byte
	Except   []string
}

/*
Gate lets a request through when it carries the gate cookie or key, or
its path is excepted, and otherwise answers with the password form.
*/
func Gate(opts GateOptions) fhttp.Middleware {
	g := gate{opts: opts, token: gateToken(opts.Secret, opts.Password)}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			g.serve(next, w, r)
		})
	}
}

type gate struct {
	opts  GateOptions
	token string
}

func (g gate) serve(next http.Handler, w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/robots.txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(gateRobots))
	case fhttp.MatchesPath(r.URL.Path, g.opts.Except...), g.admitted(r):
		next.ServeHTTP(w, r)
	case r.URL.Path == GatePath && r.Method == http.MethodPost:
		g.signIn(w, r)
	default:
		g.form(w, r, http.StatusUnauthorized, "")
	}
}

/*
admitted reports whether the request carries the key or a valid cookie.
Both compare in constant time.
*/
func (g gate) admitted(r *http.Request) bool {
	if key := r.Header.Get(GateKeyHeader); key != "" && g.opts.Key != "" {
		return subtle.ConstantTimeCompare([]byte(key), []byte(g.opts.Key)) == 1
	}

	cookie, err := r.Cookie(gateCookie)
	if err != nil || g.opts.Password == "" {
		return false
	}

	return hmac.Equal([]byte(cookie.Value), []byte(g.token))
}

/*
signIn checks the password and sets the cookie, then sends the person
back to where they were going, on this host only.
*/
func (g gate) signIn(w http.ResponseWriter, r *http.Request) {
	password := r.PostFormValue("password")
	if g.opts.Password == "" || subtle.ConstantTimeCompare([]byte(password), []byte(g.opts.Password)) != 1 {
		g.form(w, r, http.StatusUnauthorized, "That password is not right.")

		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     gateCookie,
		Value:    g.token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60,
	})

	w.Header().Set("Location", localPath(r.PostFormValue("next")))
	w.WriteHeader(http.StatusSeeOther)
}

/*
form answers with a minimal password form. It loads nothing and runs no
script, so it works under any content security policy.
*/
func (g gate) form(w http.ResponseWriter, r *http.Request, status int, problem string) {
	next := r.URL.RequestURI()
	if r.URL.Path == GatePath {
		next = localPath(r.PostFormValue("next"))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width"><meta name="robots" content="noindex">` +
		`<title>Private</title></head><body><main><h1>This site is private</h1>` +
		`<p>` + html.EscapeString(problem) + `</p>` +
		`<form method="post" action="` + GatePath + `">` +
		`<input type="hidden" name="next" value="` + html.EscapeString(next) + `">` +
		`<label>Password <input type="password" name="password" autocomplete="current-password" required autofocus>` +
		`</label> <button type="submit">Enter</button></form></main></body></html>`))
}

/*
gateToken is the cookie value: a MAC of the password under the secret,
so the cookie proves the password was typed without holding it.
*/
func gateToken(secret []byte, password string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("gate:" + password))

	return hex.EncodeToString(mac.Sum(nil))
}

/*
localPath returns the target when it is a path on this host, and "/"
otherwise, so the form never sends anyone elsewhere.
*/
func localPath(target string) string {
	parsed, err := url.Parse(target)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(target, "/") ||
		strings.HasPrefix(target, "//") || strings.Contains(target, "\\") {
		return "/"
	}

	return parsed.RequestURI()
}
