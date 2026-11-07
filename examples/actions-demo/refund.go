/*
|--------------------------------------------------------------------------
| Refunds
|--------------------------------------------------------------------------
|
| Refunds move money, so they are the one operation an agent must never
| repeat by accident. A request may carry an idempotency key: the first
| request under a key decides the outcome, every retry under the same key
| gets that same outcome back without touching the invoice again, and a
| different request under a used key is refused rather than guessed at.
|
| The policy itself: annual plans only, within thirty days of payment,
| and never more than the part of the invoice not yet refunded.
|
*/

package main

import (
	"fmt"
	"strings"
)

/*
refundRequest asks for Amount dollars of an invoice back, with the
reason the customer gave.
*/
type refundRequest struct {
	Customer  string  `json:"customer"`
	InvoiceID string  `json:"invoice_id"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason"`
}

/*
fingerprint identifies a request's content, so a reused idempotency key
can be told apart from an honest retry.
*/
func (r refundRequest) fingerprint() string {
	return fmt.Sprintf("%s|%s|%d|%s", r.Customer, r.InvoiceID, cents(r.Amount), r.Reason)
}

/*
refund issues a refund under an optional idempotency key. A retry under
a known key returns the first outcome, marked as replayed, whether that
outcome was a refund or a refusal.
*/
func (s *store) refund(req refundRequest, key string) (refundView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key == "" {
		return s.decideRefund(req, key)
	}

	if previous, ok := s.attempts[key]; ok {
		return replay(previous, req, key)
	}

	refund, err := s.decideRefund(req, key)
	s.attempts[key] = attempt{fingerprint: req.fingerprint(), refund: refund, err: err}

	return refund, err
}

/*
replay answers a retry with the outcome recorded under its key, or
refuses it when the key was first used for a different request.
*/
func replay(previous attempt, req refundRequest, key string) (refundView, error) {
	if previous.fingerprint != req.fingerprint() {
		return refundView{}, conflict("idempotency_key_reused",
			"idempotency key %q was already used for a different refund request", key)
	}

	if previous.err != nil {
		return refundView{}, previous.err
	}

	refund := previous.refund
	refund.Replayed = true

	return refund, nil
}

/*
decideRefund checks the request against the policy and, when it passes,
records the refund on the invoice.
*/
func (s *store) decideRefund(req refundRequest, key string) (refundView, error) {
	if err := validateRefund(req); err != nil {
		return refundView{}, err
	}

	if _, err := s.account(req.Customer); err != nil {
		return refundView{}, err
	}

	inv, ok := s.invoices[req.InvoiceID]
	if !ok || inv.customerID != req.Customer {
		return refundView{}, notFound("invoice_not_found", "no invoice %s for customer %s", req.InvoiceID, req.Customer)
	}

	if err := s.checkPolicy(inv, cents(req.Amount)); err != nil {
		return refundView{}, err
	}

	inv.refunded += cents(req.Amount)
	s.refundSeq++

	return refundView{
		ID:             fmt.Sprintf("re_%04d", s.refundSeq),
		CustomerID:     req.Customer,
		InvoiceID:      inv.id,
		Amount:         dollars(cents(req.Amount)),
		Currency:       currencyUSD,
		Reason:         req.Reason,
		Status:         "succeeded",
		CreatedAt:      s.now().UTC(),
		IdempotencyKey: key,
	}, nil
}

/*
validateRefund rejects a request missing what every refund needs.
*/
func validateRefund(req refundRequest) error {
	switch {
	case req.Customer == "":
		return invalid("customer_required", "customer is required")
	case req.InvoiceID == "":
		return invalid("invoice_id_required", "invoice_id is required")
	case cents(req.Amount) <= 0:
		return invalid("amount_invalid", "amount must be more than zero dollars")
	case strings.TrimSpace(req.Reason) == "":
		return invalid("reason_required", "reason is required")
	default:
		return nil
	}
}

/*
checkPolicy applies the refund policy to an invoice: annual plans only,
paid within the last thirty days, and no more than what is left of it.
*/
func (s *store) checkPolicy(inv *invoice, amount int64) error {
	if inv.interval != intervalYear {
		return refused("monthly_not_refundable",
			"invoice %s is for a monthly plan; only annual plans can be refunded", inv.id)
	}

	age := s.now().Sub(inv.paidAt)
	if age > refundWindow {
		return refused("refund_window_passed",
			"invoice %s was paid %d days ago; annual plans can be refunded within 30 days of payment",
			inv.id, int(age.Hours()/24))
	}

	left := inv.amount - inv.refunded
	if amount > left {
		return refused("amount_exceeds_balance",
			"only %s of invoice %s can still be refunded, not %s", usd(left), inv.id, usd(amount))
	}

	return nil
}
