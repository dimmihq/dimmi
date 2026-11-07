/*
|--------------------------------------------------------------------------
| REST API
|--------------------------------------------------------------------------
|
| The API a support tool would call to look up and act on a Lovelace
| customer. Every request needs the bearer token; everything answers
| JSON, and every failure answers {"error": {"code", "message"}} with a
| status that says whether it was a missing record, a bad request, a
| policy refusal or a reused idempotency key.
|
*/

package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

/*
maxBody caps a request body; no Lovelace request needs more.
*/
const maxBody = 64 << 10

/*
reply is what a handler answers: the status and JSON body, plus the
customer and a one-line summary for the journal.
*/
type reply struct {
	status   int
	body     any
	customer string
	summary  string
}

/*
restAPI serves the REST endpoints from the store, guarded by Token.
*/
type restAPI struct {
	store   *store
	journal *journal
	token   string
}

/*
routes returns the API's handler with every endpoint under /api.
*/
func (a *restAPI) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/customers/{external_id}/subscription", a.endpoint(a.getSubscription))
	mux.Handle("GET /api/customers/{external_id}/invoices", a.endpoint(a.listInvoices))
	mux.Handle("GET /api/orders/{id}", a.endpoint(a.getOrder))
	mux.Handle("POST /api/refunds", a.endpoint(a.issueRefund))
	mux.Handle("POST /api/customers/{external_id}/password-reset", a.endpoint(a.sendPasswordReset))
	mux.Handle("POST /api/subscriptions/{external_id}/plan", a.endpoint(a.changePlan))
	mux.Handle("/", a.endpoint(func(r *http.Request) reply {
		return failure(notFound("route_not_found", "no endpoint %s %s", r.Method, r.URL.Path), "")
	}))

	return mux
}

/*
endpoint checks the bearer token, runs the handler, writes its JSON and
journals the request.
*/
func (a *restAPI) endpoint(handle func(r *http.Request) reply) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rep := reply{
			status: http.StatusUnauthorized, summary: "rejected: missing or wrong bearer token",
			body: errorBody("unauthorized", "send Authorization: Bearer <token>"), customer: requestedCustomer(r),
		}
		if authorized(r, a.token) {
			rep = handle(r)
		} else {
			w.Header().Set("WWW-Authenticate", `Bearer realm="lovelace"`)
		}

		if refund, ok := rep.body.(refundView); ok && refund.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}

		writeJSON(w, rep.status, rep.body)
		a.journal.record(entry{
			channel:  "REST",
			action:   r.Method + " " + r.URL.RequestURI(),
			customer: rep.customer,
			key:      r.Header.Get("Idempotency-Key"),
			outcome:  fmt.Sprintf("%d %s", rep.status, rep.summary),
		})
	})
}

func (a *restAPI) getSubscription(r *http.Request) reply {
	id := r.PathValue("external_id")

	sub, err := a.store.subscription(id)
	if err != nil {
		return failure(err, id)
	}

	return reply{status: http.StatusOK, body: sub, customer: id, summary: describeSubscription(sub)}
}

func (a *restAPI) listInvoices(r *http.Request) reply {
	id := r.PathValue("external_id")

	list, err := a.store.invoiceList(id)
	if err != nil {
		return failure(err, id)
	}

	return reply{status: http.StatusOK, body: list, customer: id, summary: describeInvoices(list)}
}

func (a *restAPI) getOrder(r *http.Request) reply {
	id := r.URL.Query().Get("customer")
	if id == "" {
		return failure(invalid("customer_required", "pass the customer's external ID as ?customer="), "")
	}

	o, err := a.store.order(r.PathValue("id"), id)
	if err != nil {
		return failure(err, id)
	}

	return reply{status: http.StatusOK, body: o, customer: id, summary: describeOrder(o)}
}

func (a *restAPI) issueRefund(r *http.Request) reply {
	var req refundRequest
	if err := decode(r, &req); err != nil {
		return failure(err, "")
	}

	refund, err := a.store.refund(req, r.Header.Get("Idempotency-Key"))
	if err != nil {
		return failure(err, req.Customer)
	}

	return reply{status: http.StatusCreated, body: refund, customer: req.Customer, summary: describeRefund(refund)}
}

func (a *restAPI) sendPasswordReset(r *http.Request) reply {
	id := r.PathValue("external_id")

	sent, err := a.store.passwordReset(id)
	if err != nil {
		return failure(err, id)
	}

	return reply{status: http.StatusOK, body: sent, customer: id, summary: "password reset requested (nothing sent)"}
}

func (a *restAPI) changePlan(r *http.Request) reply {
	id := r.PathValue("external_id")

	var req struct {
		Plan string `json:"plan"`
	}
	if err := decode(r, &req); err != nil {
		return failure(err, id)
	}

	sub, err := a.store.changePlan(id, req.Plan)
	if err != nil {
		return failure(err, id)
	}

	return reply{status: http.StatusOK, body: sub, customer: id, summary: "plan is now " + describeSubscription(sub)}
}

/*
requestedCustomer names the customer a request is about from its path or
query, for journaling a request that was rejected before it was read.
*/
func requestedCustomer(r *http.Request) string {
	if id := r.PathValue("external_id"); id != "" {
		return id
	}

	return r.URL.Query().Get("customer")
}

/*
authorized reports whether the request carries the bearer token,
compared in constant time.
*/
func authorized(r *http.Request, token string) bool {
	given, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")

	return ok && subtle.ConstantTimeCompare([]byte(given), []byte(token)) == 1
}

/*
decode reads a JSON request body into v.
*/
func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBody)).Decode(v); err != nil {
		return invalid("invalid_json", "the request body must be a JSON object: %v", err)
	}

	return nil
}

/*
failure turns an error into a reply: a problem keeps its code and gets
the status of its kind; anything else is an internal error.
*/
func failure(err error, customer string) reply {
	var p *problemError
	if !errors.As(err, &p) {
		return reply{
			status: http.StatusInternalServerError, customer: customer, summary: "error: " + err.Error(),
			body: errorBody("internal_error", err.Error()),
		}
	}

	status := map[int]int{
		kindNotFound: http.StatusNotFound,
		kindInvalid:  http.StatusBadRequest,
		kindRefused:  http.StatusUnprocessableEntity,
		kindConflict: http.StatusConflict,
	}[p.kind]

	return reply{
		status: status, body: errorBody(p.code, p.message), customer: customer,
		summary: p.code + ": " + p.message,
	}
}

func errorBody(code, message string) map[string]any {
	return map[string]any{"error": map[string]string{"code": code, "message": message}}
}

/*
writeJSON writes v as the response body. A failed write means the client
went away, and there is nobody left to tell.
*/
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
