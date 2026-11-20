/*
|--------------------------------------------------------------------------
| Status Reports
|--------------------------------------------------------------------------
|
| How far a message the business sent got, why it failed, and what
| Meta charges for it.
|
*/

package whatsapp

import "time"

/*
Status is how far a message the business sent got: sent, delivered,
read or failed, with Meta's reasons for a failure and what Meta charges.
*/
type Status struct {
	ID              string        `json:"id"`
	Status          string        `json:"status"`
	Timestamp       string        `json:"timestamp"`
	RecipientID     string        `json:"recipient_id"`
	RecipientUserID string        `json:"recipient_user_id"`
	Errors          []StatusError `json:"errors"`
	Pricing         *Pricing      `json:"pricing"`
}

/*
StatusError is why Meta could not deliver a message.
*/
type StatusError struct {
	Code    int    `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Data    struct {
		Details string `json:"details"`
	} `json:"error_data"`
}

/*
Pricing is what Meta charges for a delivered message.
*/
type Pricing struct {
	Billable bool   `json:"billable"`
	Model    string `json:"pricing_model"`
	Category string `json:"category"`
	Type     string `json:"type"`
}

/*
At reads the report's time, or the fallback when it has none.
*/
func (s Status) At(fallback time.Time) time.Time {
	return unixTime(s.Timestamp, fallback)
}
