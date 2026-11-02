/*
|--------------------------------------------------------------------------
| Session Middleware
|--------------------------------------------------------------------------
|
| The middleware loads the session before the handler runs and saves it
| the moment the handler starts its response, because a cookie can only
| be set while the headers have not gone out yet. When saving fails, the
| client gets the error instead of the handler's response: a login whose
| session was never stored must not look like it worked.
|
*/

package session

import (
	"context"
	"net/http"
)

/*
Middleware loads the request's session into the context and saves it
before the response is sent.
*/
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Start(r)
		if err != nil {
			m.cfg.Errors.Render(w, r, err)

			return
		}

		ctx := WithSession(r.Context(), s)
		r = r.WithContext(ctx)
		cw := &committer{ResponseWriter: w, manager: m, request: r, session: s}

		next.ServeHTTP(cw, r)
		cw.flush(ctx)
	})
}

/*
committer saves the session right before the first byte of the response
is written, or after the handler returns when it wrote nothing. When the
save fails it sends the error and drops what the handler writes.
*/
type committer struct {
	http.ResponseWriter
	manager *Manager
	request *http.Request
	session *Session
	done    bool
	failed  bool
}

/*
WriteHeader saves the session, then sends the handler's status.
*/
func (c *committer) WriteHeader(status int) {
	if !c.flush(c.request.Context()) {
		return
	}

	c.ResponseWriter.WriteHeader(status)
}

/*
Write saves the session, then sends the bytes.
*/
func (c *committer) Write(b []byte) (int, error) {
	if !c.flush(c.request.Context()) {
		return len(b), nil
	}

	return c.ResponseWriter.Write(b)
}

/*
Unwrap returns the original writer, so flushing and hijacking still
reach the connection.
*/
func (c *committer) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}

/*
flush saves the session once and reports whether the handler's response
may still be sent.
*/
func (c *committer) flush(ctx context.Context) bool {
	if !c.done {
		c.done = true

		if err := c.manager.commit(ctx, c.ResponseWriter, c.session); err != nil {
			c.failed = true
			c.manager.cfg.Errors.Render(c.ResponseWriter, c.request, err)
		}
	}

	return !c.failed
}
