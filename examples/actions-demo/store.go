/*
|--------------------------------------------------------------------------
| Lovelace Billing Store
|--------------------------------------------------------------------------
|
| Lovelace is a fictional project management product. This store stands in
| for its billing system: customers, subscriptions, invoices and orders,
| all in memory and seeded at start, so a demo always begins from the same
| story. The REST API and the MCP server share one store, so whatever an
| agent does through one is visible through the other.
|
| The refund policy is the point of the demo: only annual plans are
| refundable, only within thirty days of payment, and never for more than
| the part of an invoice that has not been refunded yet.
|
*/

package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

/*
The plans Lovelace sells, and the billing intervals they come in.
*/
const (
	planFree     = "free"
	planPro      = "pro"
	planBusiness = "business"

	intervalMonth = "month"
	intervalYear  = "year"

	currencyUSD  = "usd"
	refundWindow = 30 * 24 * time.Hour
)

/*
seatPrice returns the monthly price of one seat on a plan, in cents, and
false for a plan Lovelace does not sell.
*/
func seatPrice(plan string) (int64, bool) {
	switch plan {
	case planFree:
		return 0, true
	case planPro:
		return 1200, true
	case planBusiness:
		return 2400, true
	default:
		return 0, false
	}
}

/*
account is one customer with their subscription. PaidAt is when the
current period was paid and RenewsAt when the next one is charged.
*/
type account struct {
	id       string
	name     string
	email    string
	company  string
	plan     string
	seats    int
	interval string
	paidAt   time.Time
	renewsAt time.Time
}

/*
invoice is one payment. Refunded counts the cents already given back, so
a second refund can never return the same money twice.
*/
type invoice struct {
	id          string
	customerID  string
	description string
	plan        string
	interval    string
	amount      int64
	refunded    int64
	paidAt      time.Time
}

/*
order is one purchase that produced an invoice.
*/
type order struct {
	id          string
	customerID  string
	description string
	status      string
	amount      int64
	invoiceID   string
	createdAt   time.Time
}

/*
attempt remembers what a refund request with an idempotency key produced,
along with a fingerprint of the request, so a retry gets the same answer
and a different request under the same key is refused.
*/
type attempt struct {
	fingerprint string
	refund      refundView
	err         error
}

/*
store holds every record behind one lock. Now is the clock the refund
window is measured with.
*/
type store struct {
	mu        sync.Mutex
	now       func() time.Time
	accounts  map[string]*account
	invoices  map[string]*invoice
	orders    map[string]*order
	attempts  map[string]attempt
	refundSeq int
}

/*
newStore returns a store seeded with three customers: Ada on Pro annual,
paid ten days ago and refundable; Grace on Pro monthly, never refundable;
and Katherine on Business annual, paid too long ago to refund.
*/
func newStore(now func() time.Time) *store {
	s := &store{
		now:      now,
		accounts: map[string]*account{},
		invoices: map[string]*invoice{},
		orders:   map[string]*order{},
		attempts: map[string]attempt{},
	}
	s.seed(now())

	return s
}

/*
seed writes the demo records, dated relative to the moment the store
starts so the refund windows always tell the same story.
*/
func (s *store) seed(today time.Time) {
	adaPaid := today.AddDate(0, 0, -10)
	gracePaid := today.AddDate(0, 0, -5)
	katherinePaid := today.AddDate(0, 0, -45)

	s.addAccount(&account{
		id: "user_42", name: "Ada Lovelace", email: "ada@analytical.example", company: "Analytical Engines",
		plan: planPro, seats: 1, interval: intervalYear, paidAt: adaPaid, renewsAt: adaPaid.AddDate(1, 0, 0),
	})
	s.addAccount(&account{
		id: "user_7", name: "Grace Hopper", email: "grace@compilers.example", company: "Compiler Co",
		plan: planPro, seats: 3, interval: intervalMonth, paidAt: gracePaid, renewsAt: gracePaid.AddDate(0, 1, 0),
	})
	s.addAccount(&account{
		id: "user_99", name: "Katherine Johnson", email: "katherine@orbital.example", company: "Orbital Labs",
		plan: planBusiness, seats: 5, interval: intervalYear, paidAt: katherinePaid,
		renewsAt: katherinePaid.AddDate(1, 0, 0),
	})

	s.addInvoice(&invoice{
		id: "inv_1001", customerID: "user_42", plan: planPro, interval: intervalYear,
		description: "Lovelace Pro, annual, 1 seat", amount: 14400, paidAt: adaPaid,
	})
	s.addInvoice(&invoice{
		id: "inv_0901", customerID: "user_42", plan: planPro, interval: intervalYear,
		description: "Lovelace Pro, annual, 1 seat", amount: 14400, paidAt: adaPaid.AddDate(-1, 0, 0),
	})
	s.addInvoice(&invoice{
		id: "inv_1002", customerID: "user_7", plan: planPro, interval: intervalMonth,
		description: "Lovelace Pro, monthly, 3 seats", amount: 3600, paidAt: gracePaid,
	})
	s.addInvoice(&invoice{
		id: "inv_0977", customerID: "user_7", plan: planPro, interval: intervalMonth,
		description: "Lovelace Pro, monthly, 3 seats", amount: 3600, paidAt: gracePaid.AddDate(0, -1, 0),
	})
	s.addInvoice(&invoice{
		id: "inv_0950", customerID: "user_99", plan: planBusiness, interval: intervalYear,
		description: "Lovelace Business, annual, 5 seats", amount: 144000, paidAt: katherinePaid,
	})

	s.seedOrders()
}

/*
seedOrders writes one order per current invoice, placed when the invoice
was paid.
*/
func (s *store) seedOrders() {
	s.addOrder(&order{
		id: "ord_5001", customerID: "user_42", description: "Pro annual plan, 1 seat",
		status: "fulfilled", amount: 14400, invoiceID: "inv_1001",
	})
	s.addOrder(&order{
		id: "ord_5002", customerID: "user_7", description: "2 extra Pro seats",
		status: "fulfilled", amount: 2400, invoiceID: "inv_1002",
	})
	s.addOrder(&order{
		id: "ord_5003", customerID: "user_99", description: "Business annual plan, 5 seats",
		status: "fulfilled", amount: 144000, invoiceID: "inv_0950",
	})
}

func (s *store) addAccount(a *account) { s.accounts[a.id] = a }

func (s *store) addInvoice(i *invoice) { s.invoices[i.id] = i }

func (s *store) addOrder(o *order) {
	o.createdAt = s.invoices[o.invoiceID].paidAt
	s.orders[o.id] = o
}

/*
subscription returns a customer's current plan, seats and billing dates.
*/
func (s *store) subscription(customerID string) (subscriptionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, err := s.account(customerID)
	if err != nil {
		return subscriptionView{}, err
	}

	return viewSubscription(a), nil
}

/*
invoiceList returns every invoice of a customer, newest first.
*/
func (s *store) invoiceList(customerID string) (invoiceListView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.account(customerID); err != nil {
		return invoiceListView{}, err
	}

	list := invoiceListView{CustomerID: customerID, Invoices: []invoiceView{}}
	for _, i := range s.invoices {
		if i.customerID == customerID {
			list.Invoices = append(list.Invoices, viewInvoice(i))
		}
	}
	sortInvoices(list.Invoices)

	return list, nil
}

/*
order returns one order, but only to the customer who placed it: anyone
else is told it does not exist, so order IDs cannot be probed.
*/
func (s *store) order(orderID, customerID string) (orderView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.orders[orderID]
	if !ok || o.customerID != customerID {
		return orderView{}, notFound("order_not_found", "no order %s for customer %s", orderID, customerID)
	}

	return viewOrder(o), nil
}

/*
passwordReset pretends to email the customer a reset link. Nothing is
sent; the answer says where the email would have gone.
*/
func (s *store) passwordReset(customerID string) (passwordResetView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, err := s.account(customerID)
	if err != nil {
		return passwordResetView{}, err
	}

	return passwordResetView{
		OK:         true,
		CustomerID: a.id,
		Email:      a.email,
		Message:    "A password reset link is on its way to " + a.email + ". (Demo: nothing was sent.)",
	}, nil
}

/*
changePlan moves a customer to another plan at once and reprices the
subscription for its seats and interval. Moving to the plan the customer
already has changes nothing, so the call is safe to repeat.
*/
func (s *store) changePlan(customerID, plan string) (subscriptionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, err := s.account(customerID)
	if err != nil {
		return subscriptionView{}, err
	}

	if _, ok := seatPrice(plan); !ok {
		return subscriptionView{}, invalid("unknown_plan", "plan must be free, pro or business, not %q", plan)
	}

	a.plan = plan

	return viewSubscription(a), nil
}

/*
account finds a customer by external ID.
*/
func (s *store) account(customerID string) (*account, error) {
	a, ok := s.accounts[customerID]
	if !ok {
		return nil, notFound("customer_not_found", "no customer %s", customerID)
	}

	return a, nil
}

/*
periodAmount is what a subscription costs per billing period, in cents.
*/
func periodAmount(a *account) int64 {
	price, _ := seatPrice(a.plan)
	amount := price * int64(a.seats)
	if a.interval == intervalYear {
		return amount * 12
	}

	return amount
}

/*
cents converts a dollar amount to whole cents.
*/
func cents(dollars float64) int64 { return int64(math.Round(dollars * 100)) }

/*
dollars converts cents to a dollar amount.
*/
func dollars(c int64) float64 { return float64(c) / 100 }

/*
usd formats cents as a dollar amount for people.
*/
func usd(c int64) string { return fmt.Sprintf("$%.2f", dollars(c)) }
