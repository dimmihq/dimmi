/*
|--------------------------------------------------------------------------
| Idempotency Keys
|--------------------------------------------------------------------------
|
| A POST sent with an idempotency key runs once. A retry with the same key
| and the same request gets the first response again, marked as replayed;
| a retry while the first is still running is told to wait; the same key
| on a different request is refused. A response the server failed to
| produce is not kept, so the retry runs again.
|
*/

package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/idempotency"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
The headers Idempotent reads and writes by default.
*/
const (
	DefaultIdempotencyHeader = "Idempotency-Key"
	ReplayedHeader           = "Idempotent-Replayed"
)

/*
maxKeyLength bounds the keys clients may send.
*/
const maxKeyLength = 255

/*
Idempotent makes POST requests carrying the header run once per key. The
scope function keeps keys of different callers apart, such as by user;
without it keys are shared by everyone in a tenant.
*/
func Idempotent(store idempotency.Store, header string, scope func(r *http.Request) string) fhttp.Middleware {
	if header == "" {
		header = DefaultIdempotencyHeader
	}

	if scope == nil {
		scope = func(*http.Request) string { return "" }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(header)
			if r.Method != http.MethodPost || key == "" {
				next.ServeHTTP(w, r)

				return
			}

			if len(key) > maxKeyLength {
				refuse(w, r, fault.New(fault.Invalid, "invalid_idempotency_key", "The idempotency key is too long."))

				return
			}

			once := idempotentRun{store: store, scope: scope(r), key: key, next: next}
			once.serve(w, r)
		})
	}
}

/*
idempotentRun is one keyed request.
*/
type idempotentRun struct {
	store idempotency.Store
	scope string
	key   string
	next  http.Handler
}

func (run idempotentRun) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		refuse(w, r, fmt.Errorf("read body: %w", err))

		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	ctx := r.Context()

	stored, err := run.store.Reserve(ctx, run.scope, run.key, fingerprint(r, body))
	if err != nil {
		refuse(w, r, reserveFault(err))

		return
	}

	if stored != nil {
		replay(w, stored)

		return
	}

	run.record(w, r)
}

/*
record runs the request and keeps its response. A server error or a
panic releases the key instead, so the client's retry runs again.
*/
func (run idempotentRun) record(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rec := &recorder{ResponseWriter: w, body: &bytes.Buffer{}}
	kept := false

	defer func() {
		if !kept {
			run.release(context.WithoutCancel(ctx))
		}
	}()

	run.next.ServeHTTP(rec, r)

	if rec.status >= http.StatusInternalServerError {
		return
	}

	res := idempotency.Response{Status: rec.status, Header: w.Header().Clone(), Body: rec.body.Bytes()}
	if err := run.store.Complete(context.WithoutCancel(ctx), run.scope, run.key, res); err != nil {
		log.From(ctx).ErrorContext(ctx, "idempotent response not stored", log.Err(err))

		return
	}

	kept = true
}

func (run idempotentRun) release(ctx context.Context) {
	if err := run.store.Release(ctx, run.scope, run.key); err != nil {
		log.From(ctx).ErrorContext(ctx, "idempotency key not released", log.Err(err))
	}
}

func replay(w http.ResponseWriter, res *idempotency.Response) {
	maps.Copy(w.Header(), res.Header)
	w.Header().Set(ReplayedHeader, "true")
	w.WriteHeader(res.Status)
	_, _ = io.Copy(w, bytes.NewReader(res.Body))
}

/*
fingerprint identifies the request a key was used for: its method, path,
query and body.
*/
func fingerprint(r *http.Request, body []byte) string {
	head := r.Method + " " + r.URL.RequestURI() + "\n"

	return hashing.SHA256Hex(append([]byte(head), body...))
}

func reserveFault(err error) error {
	switch {
	case errors.Is(err, idempotency.ErrKeyReused):
		return fault.New(fault.Invalid, "idempotency_key_reused",
			"This idempotency key was used for a different request.")
	case errors.Is(err, idempotency.ErrInProgress):
		return fault.New(fault.Conflict, "idempotency_key_in_use",
			"A request with this idempotency key is in progress.")
	default:
		return err
	}
}
