/*
|--------------------------------------------------------------------------
| Broadcast Report Resources
|--------------------------------------------------------------------------
|
| How a broadcast's report reads over the API: its counts, why people
| were left out, how each link did and which companies it reached.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BroadcastReport is a broadcast's results.
*/
type BroadcastReport struct {
	Object    string                             `json:"object"`
	Broadcast Broadcast                          `json:"broadcast"`
	Counts    models.TallyBroadcastRecipientsRow `json:"counts"`
	Reasons   []BroadcastReason                  `json:"reasons"`
	Links     []BroadcastLink                    `json:"links"`
	Companies []BroadcastCompany                 `json:"companies"`
	Waiting   []BroadcastReason                  `json:"waiting"`
}

/*
BroadcastReason is why some recipients were left out, and how many.
*/
type BroadcastReason struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

/*
BroadcastLink is one counted link and its clicks.
*/
type BroadcastLink struct {
	N      int32  `json:"n"`
	URL    string `json:"url"`
	Clicks int32  `json:"clicks"`
}

/*
BroadcastCompany is one account's results.
*/
type BroadcastCompany struct {
	CompanyID string `json:"company_id"`
	Name      string `json:"name"`
	Total     int64  `json:"total"`
	Delivered int64  `json:"delivered"`
	Opened    int64  `json:"opened"`
	Clicked   int64  `json:"clicked"`
	Replied   int64  `json:"replied"`
	Reactions int64  `json:"reactions"`
	Failed    int64  `json:"failed"`
}

/*
NewBroadcastReport shapes a broadcast's report.
*/
func NewBroadcastReport(r broadcasts.Report) BroadcastReport {
	out := BroadcastReport{
		Object: "broadcast_report", Broadcast: NewBroadcast(r.Broadcast), Counts: r.Counts,
		Reasons: []BroadcastReason{}, Links: []BroadcastLink{}, Companies: []BroadcastCompany{},
		Waiting: []BroadcastReason{},
	}

	for _, w := range r.Waiting {
		out.Waiting = append(out.Waiting, BroadcastReason{Reason: w.Reason, Count: w.N})
	}

	for _, reason := range r.Reasons {
		out.Reasons = append(out.Reasons, BroadcastReason{Reason: reason.Reason, Count: reason.N})
	}

	for _, l := range r.Links {
		out.Links = append(out.Links, BroadcastLink{N: l.N, URL: l.Url, Clicks: l.Clicks})
	}

	for _, c := range r.Companies {
		out.Companies = append(out.Companies, BroadcastCompany{
			CompanyID: deref(broadcastRef(models.PrefixCompany, c.CompanyID)), Name: c.CompanyName, Total: c.Total,
			Delivered: c.Delivered, Opened: c.Opened, Clicked: c.Clicked, Replied: c.Replied, Reactions: c.Reactions,
			Failed: c.Failed,
		})
	}

	return out
}
