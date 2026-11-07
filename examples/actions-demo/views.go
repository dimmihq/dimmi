/*
|--------------------------------------------------------------------------
| Views and Problems
|--------------------------------------------------------------------------
|
| Both the REST API and the MCP tools answer with the same JSON shapes,
| amounts in dollars and times in UTC, so an agent sees one Lovelace no
| matter how it connects. A problem is an expected refusal with a stable
| code and a message written for the agent reading it; each side maps the
| kind to its own way of failing.
|
*/

package main

import (
	"fmt"
	"slices"
	"time"
)

/*
The kinds of problem, each answered with its own HTTP status.
*/
const (
	kindNotFound = iota + 1
	kindInvalid
	kindRefused
	kindConflict
)

/*
problemError is an expected failure: Kind says what went wrong, Code is a
stable identifier and Message explains it in a sentence.
*/
type problemError struct {
	kind    int
	code    string
	message string
}

func (e *problemError) Error() string { return e.message }

func problem(kind int, code, format string, args ...any) error {
	return &problemError{kind: kind, code: code, message: fmt.Sprintf(format, args...)}
}

func notFound(code, format string, args ...any) error {
	return problem(kindNotFound, code, format, args...)
}

func invalid(code, format string, args ...any) error {
	return problem(kindInvalid, code, format, args...)
}

func refused(code, format string, args ...any) error {
	return problem(kindRefused, code, format, args...)
}

func conflict(code, format string, args ...any) error {
	return problem(kindConflict, code, format, args...)
}

/*
subscriptionView is a customer's plan as the API shows it.
*/
type subscriptionView struct {
	CustomerID      string    `json:"customer_id"`
	Name            string    `json:"name"`
	Email           string    `json:"email"`
	Company         string    `json:"company"`
	Plan            string    `json:"plan"`
	Seats           int       `json:"seats"`
	BillingInterval string    `json:"billing_interval"`
	Status          string    `json:"status"`
	PricePerSeat    float64   `json:"price_per_seat"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	PaidAt          time.Time `json:"paid_at"`
	RenewsAt        time.Time `json:"renews_at"`
}

/*
invoiceView is one invoice as the API shows it.
*/
type invoiceView struct {
	ID              string    `json:"id"`
	CustomerID      string    `json:"customer_id"`
	Description     string    `json:"description"`
	Plan            string    `json:"plan"`
	BillingInterval string    `json:"billing_interval"`
	Amount          float64   `json:"amount"`
	AmountRefunded  float64   `json:"amount_refunded"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	PaidAt          time.Time `json:"paid_at"`
}

/*
invoiceListView is every invoice of one customer, newest first.
*/
type invoiceListView struct {
	CustomerID string        `json:"customer_id"`
	Invoices   []invoiceView `json:"invoices"`
}

/*
orderView is one order as the API shows it.
*/
type orderView struct {
	ID          string    `json:"id"`
	CustomerID  string    `json:"customer_id"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	InvoiceID   string    `json:"invoice_id"`
	CreatedAt   time.Time `json:"created_at"`
}

/*
refundView is one refund. Replayed is true when the answer repeats an
earlier request with the same idempotency key and no money moved.
*/
type refundView struct {
	ID             string    `json:"id"`
	CustomerID     string    `json:"customer_id"`
	InvoiceID      string    `json:"invoice_id"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Replayed       bool      `json:"replayed"`
}

/*
passwordResetView confirms a reset link was requested for a customer.
*/
type passwordResetView struct {
	OK         bool   `json:"ok"`
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	Message    string `json:"message"`
}

func viewSubscription(a *account) subscriptionView {
	price, _ := seatPrice(a.plan)

	return subscriptionView{
		CustomerID:      a.id,
		Name:            a.name,
		Email:           a.email,
		Company:         a.company,
		Plan:            a.plan,
		Seats:           a.seats,
		BillingInterval: a.interval,
		Status:          "active",
		PricePerSeat:    dollars(price),
		Amount:          dollars(periodAmount(a)),
		Currency:        currencyUSD,
		PaidAt:          a.paidAt.UTC(),
		RenewsAt:        a.renewsAt.UTC(),
	}
}

func viewInvoice(i *invoice) invoiceView {
	status := "paid"
	if i.refunded > 0 {
		status = "partially_refunded"
	}

	if i.refunded >= i.amount {
		status = "refunded"
	}

	return invoiceView{
		ID:              i.id,
		CustomerID:      i.customerID,
		Description:     i.description,
		Plan:            i.plan,
		BillingInterval: i.interval,
		Amount:          dollars(i.amount),
		AmountRefunded:  dollars(i.refunded),
		Currency:        currencyUSD,
		Status:          status,
		PaidAt:          i.paidAt.UTC(),
	}
}

func viewOrder(o *order) orderView {
	return orderView{
		ID:          o.id,
		CustomerID:  o.customerID,
		Description: o.description,
		Status:      o.status,
		Amount:      dollars(o.amount),
		Currency:    currencyUSD,
		InvoiceID:   o.invoiceID,
		CreatedAt:   o.createdAt.UTC(),
	}
}

func sortInvoices(list []invoiceView) {
	slices.SortFunc(list, func(a, b invoiceView) int { return b.PaidAt.Compare(a.PaidAt) })
}
