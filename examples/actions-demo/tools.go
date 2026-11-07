/*
|--------------------------------------------------------------------------
| MCP Tools
|--------------------------------------------------------------------------
|
| Each tool is a thin call into the store: the same operation the REST
| endpoint runs, answered with the same JSON and a summary sentence. The
| read tools' descriptions carry the pricing and the refund policy, so an
| agent knows what is possible before it asks for anything that moves
| money.
|
*/

package main

/*
Descriptions of the read tools, as agents see them.
*/
const (
	describeGetSubscription = "Get a customer's subscription: plan (free, pro or business), seats, " +
		"billing interval (month or year), amount paid, when it was paid and when it renews. " +
		"Pro is $12 and Business $24 per seat per month; annual plans are billed 12 months upfront."
	describeListInvoices = "List a customer's invoices, newest first, with amount, amount already " +
		"refunded, billing interval and payment date. Use it to find the invoice_id for issue_refund."
	describeGetOrder = "Look up one of a customer's orders by ID. An order that belongs to another " +
		"customer is reported as not found."
)

/*
tools runs each MCP tool against the store.
*/
type tools struct {
	store *store
}

func (t tools) getSubscription(in customerInput, _ string) (subscriptionView, string, error) {
	sub, err := t.store.subscription(in.CustomerID)

	return sub, describeSubscription(sub), err
}

func (t tools) listInvoices(in customerInput, _ string) (invoiceListView, string, error) {
	list, err := t.store.invoiceList(in.CustomerID)

	return list, describeInvoices(list), err
}

func (t tools) getOrder(in orderInput, _ string) (orderView, string, error) {
	o, err := t.store.order(in.OrderID, in.CustomerID)

	return o, describeOrder(o), err
}

func (t tools) issueRefund(in refundInput, key string) (refundView, string, error) {
	req := refundRequest{Customer: in.CustomerID, InvoiceID: in.InvoiceID, Amount: in.Amount, Reason: in.Reason}
	refund, err := t.store.refund(req, key)

	return refund, describeRefund(refund), err
}

func (t tools) changePlan(in planInput, _ string) (subscriptionView, string, error) {
	sub, err := t.store.changePlan(in.CustomerID, in.Plan)

	return sub, "plan is now " + describeSubscription(sub), err
}

func (t tools) sendPasswordReset(in customerInput, _ string) (passwordResetView, string, error) {
	sent, err := t.store.passwordReset(in.CustomerID)

	return sent, sent.Message, err
}
