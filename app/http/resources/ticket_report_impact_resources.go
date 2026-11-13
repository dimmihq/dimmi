/*
|--------------------------------------------------------------------------
| Ticket Impact Report Resources
|--------------------------------------------------------------------------
|
| Service level attainment per policy and target, and the trackers
| ranked by the customers, companies and revenue they reach.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/ticketreports"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketSLAAttainment is one target of one policy.
*/
type TicketSLAAttainment struct {
	PolicyID       string   `json:"policy_id"`
	PolicyName     string   `json:"policy_name"`
	Target         string   `json:"target"     enum:"first_reply,next_reply,follow_up,resolution"`
	Met            int64    `json:"met"`
	Breached       int64    `json:"breached"`
	HitRate        *float64 `json:"hit_rate"`
	MeanMetSeconds float64  `json:"mean_met_seconds"`
	Running        int64    `json:"running"`
}

/*
NewTicketSLAAttainments shapes the attainment rows.
*/
func NewTicketSLAAttainments(rows []ticketreports.Attainment) []TicketSLAAttainment {
	out := make([]TicketSLAAttainment, 0, len(rows))
	for _, r := range rows {
		out = append(out, TicketSLAAttainment{
			PolicyID: models.PublicID(models.PrefixSLAPolicy, r.PolicyID), PolicyName: r.PolicyName,
			Target: r.Target, Met: r.Met, Breached: r.Breached, HitRate: r.Percent, MeanMetSeconds: r.MetSeconds,
			Running: r.Running,
		})
	}

	return out
}

/*
TicketTrackerImpact is one tracker and what it reaches.
*/
type TicketTrackerImpact struct {
	ID                   string    `json:"id"`
	Reference            string    `json:"reference"`
	Kind                 string    `json:"kind"                   enum:"tracker"`
	Title                string    `json:"title"`
	StateGroup           string    `json:"state_group"`
	Conversations        int64     `json:"conversations"`
	ConversationsInRange int64     `json:"conversations_in_range"`
	Customers            int64     `json:"customers"`
	Companies            int64     `json:"companies"`
	Revenue              float64   `json:"revenue"`
	CreatedAt            time.Time `json:"created_at"`
}

/*
NewTicketTrackerImpacts shapes the tracker ranking.
*/
func NewTicketTrackerImpacts(rows []ticketreports.Tracker) []TicketTrackerImpact {
	out := make([]TicketTrackerImpact, 0, len(rows))
	for _, r := range rows {
		out = append(out, TicketTrackerImpact{
			ID: models.PublicID(models.PrefixTicket, r.ID), Reference: tickets.Reference(r.Number), Kind: r.Kind,
			Title: r.Title, StateGroup: r.StateGroup, Conversations: r.Conversations,
			ConversationsInRange: r.LinkedInRange, Customers: r.Customers, Companies: r.Companies,
			Revenue: r.Revenue, CreatedAt: r.CreatedAt,
		})
	}

	return out
}
