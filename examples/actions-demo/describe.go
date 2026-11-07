/*
|--------------------------------------------------------------------------
| Summaries
|--------------------------------------------------------------------------
|
| One sentence per record, used twice: as the text an agent reads next to
| a tool's structured result, and as the outcome on a journal line. A
| person scanning the log and a model reading the result see the same
| words.
|
*/

package main

import (
	"fmt"
	"strings"
)

func describeSubscription(s subscriptionView) string {
	return fmt.Sprintf("%s (%s) is on %s, %d seat(s), billed per %s: $%.2f, paid %s, renews %s",
		s.Name, s.CustomerID, s.Plan, s.Seats, s.BillingInterval, s.Amount,
		s.PaidAt.Format("2006-01-02"), s.RenewsAt.Format("2006-01-02"))
}

func describeInvoices(l invoiceListView) string {
	parts := make([]string, 0, len(l.Invoices))
	for _, i := range l.Invoices {
		parts = append(parts, fmt.Sprintf("%s $%.2f %s paid %s (%s)",
			i.ID, i.Amount, i.BillingInterval+"ly", i.PaidAt.Format("2006-01-02"), i.Status))
	}

	return fmt.Sprintf("%d invoice(s) for %s: %s", len(l.Invoices), l.CustomerID, strings.Join(parts, "; "))
}

func describeOrder(o orderView) string {
	return fmt.Sprintf("order %s: %s, $%.2f, %s, invoice %s", o.ID, o.Description, o.Amount, o.Status, o.InvoiceID)
}

func describeRefund(r refundView) string {
	if r.Replayed {
		return fmt.Sprintf("replayed refund %s of $%.2f on invoice %s; no new money moved", r.ID, r.Amount, r.InvoiceID)
	}

	return fmt.Sprintf("refunded $%.2f on invoice %s as %s", r.Amount, r.InvoiceID, r.ID)
}
