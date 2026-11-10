//go:build dev

/*
|--------------------------------------------------------------------------
| Broadcast Demo History
|--------------------------------------------------------------------------
|
| The broadcasts the demo seeds and what happened to each: an email that
| went well, one that paused itself on bounces, a Messenger chat, a post,
| a Slack post into every account's channel, a scheduled offer and a
| draft. Outcomes are drawn per person at realistic rates, so the report
| shows a believable funnel, reasons and accounts.
|
*/

package commands

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
demoBroadcast is one broadcast the demo seeds: its row, who it went to,
when, and the share of them that bounced, opened, clicked, replied and
left.
*/
type demoBroadcast struct {
	name, channel, status, subject, preview, body, audience string
	pause                                                   string
	daysAgo                                                 int
	bounce, open, click, reply, unsub, sentShare            float64
	links                                                   []string
}

/*
demoBroadcasts are the broadcasts the demo seeds, newest last.
*/
func demoBroadcasts() []demoBroadcast {
	return []demoBroadcast{
		{
			name: "November product update", channel: "email", status: "sent", daysAgo: 9,
			subject:  "What's new in November, {{ contact.first_name | default: \"there\" }}",
			preview:  "Cited AI answers, saved audiences and a faster inbox.",
			body:     "Hi {{ contact.first_name | default: \"there\" }},\n\nThis month the agent started citing its sources.\n\n[Read the changelog](https://acme.test/changelog)\n\n[See the docs](https://acme.test/docs)",
			audience: "paying", bounce: 0.011, open: 0.46, click: 0.13, reply: 0.028,
			unsub: 0.004, sentShare: 1, links: []string{"https://acme.test/changelog", "https://acme.test/docs"},
		},
		{
			name: "Webinar: set up your AI agent", channel: "email", status: "paused", pause: "bounce_rate", daysAgo: 1,
			subject: "Live on Thursday: your AI agent in 30 minutes", preview: "Bring your questions.",
			body:     "Join us on Thursday at 17:00 CET.\n\n[Save your seat](https://acme.test/webinar)",
			audience: "free", bounce: 0.072, open: 0.21, click: 0.05, reply: 0.006, unsub: 0.01, sentShare: 0.4,
			links: []string{"https://acme.test/webinar"},
		},
		{
			name: "Book an onboarding call", channel: "chat", status: "sent", daysAgo: 4,
			body: "Hi {{ contact.first_name | default: \"there\" }}, want a hand getting set up? " +
				"I can walk you through the inbox in 15 minutes.",
			audience: "free", open: 0.62, reply: 0.21, sentShare: 1,
		},
		{
			name: "New pricing explained", channel: "post", status: "sent", daysAgo: 12,
			subject:  "Our new pricing, and what it means for you",
			body:     "Nothing changes for existing workspaces until January.\n\n[Compare plans](https://acme.test/pricing)",
			audience: "admins", open: 0.58, click: 0.2, reply: 0.04, sentShare: 1,
			links: []string{"https://acme.test/pricing"},
		},
		{
			name: "Annual plan offer", channel: "email", status: "scheduled", daysAgo: -2,
			subject: "Two months free when you switch to annual", preview: "Lock in this year's price.",
			body: "Switch to annual before December and get two months free.", audience: "pro",
		},
		{
			name: "Black Friday", channel: "email", status: "draft", subject: "30% off your first year",
			body: "Only until Monday.", audience: "free",
		},
	}
}

/*
history seeds every demo broadcast and its recipients.
*/
func (s *demoSeed) history(ctx context.Context, tx *database.Tx) error {
	for _, b := range demoBroadcasts() {
		id, err := s.broadcast(ctx, tx, b)
		if err != nil {
			return err
		}

		if b.sentShare > 0 {
			if err := s.recipients(ctx, tx, id, b); err != nil {
				return err
			}
		}

		for i, url := range b.links {
			if _, err := tx.Exec(ctx, `INSERT INTO broadcast_links (workspace_id, broadcast_id, n, url, clicks)
				VALUES ($1, $2, $3, $4, (SELECT count(*) * $5::float8 FROM broadcast_recipients
				WHERE broadcast_id = $2 AND clicked_at IS NOT NULL))`, s.ws, id, i+1, url, 1/float64(i+1)); err != nil {
				return fmt.Errorf("seed link: %w", err)
			}
		}

		s.made = append(s.made, id)
	}

	return nil
}

/*
broadcast seeds one broadcast's row.
*/
func (s *demoSeed) broadcast(ctx context.Context, tx *database.Tx, b demoBroadcast) (ids.UUID, error) {
	var id ids.UUID

	err := tx.QueryRow(ctx, `INSERT INTO broadcasts (workspace_id, name, channel, status, pause_reason, rules,
		subject, preview, body, sender_member_id, created_by_member_id, sent_by_member_id, schedule_kind, scheduled_at,
		started_at, finished_at, created_at, updated_at, email_address_id)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6::jsonb, $7, $8, $9, $10, $10,
			CASE WHEN $4 IN ('sent', 'paused') THEN $10::uuid END,
			CASE WHEN $4 = 'scheduled' THEN 'local' ELSE 'now' END,
			CASE WHEN $4 = 'scheduled' THEN now() + make_interval(days => -$11::int) END,
			CASE WHEN $4 IN ('sent', 'paused') THEN now() - make_interval(days => $11::int) END,
			CASE WHEN $4 = 'sent' THEN now() - make_interval(days => $11::int) + interval '25 minutes' END,
			now() - make_interval(days => abs($11::int) + 1), now() - make_interval(days => abs($11::int)),
			(SELECT id FROM email_addresses WHERE workspace_id = $1 AND deleted_at IS NULL ORDER BY created_at LIMIT 1))
		RETURNING id`,
		s.ws, b.name, b.channel, b.status, b.pause, rulesOf(b.audience), b.subject, b.preview, b.body, s.member,
		b.daysAgo).Scan(&id)
	if err != nil {
		return id, fmt.Errorf("seed broadcast %s: %w", b.name, err)
	}

	return id, nil
}

/*
recipients seeds who a broadcast went to and what each did.
*/
func (s *demoSeed) recipients(ctx context.Context, tx *database.Tx, id ids.UUID, b demoBroadcast) error {
	_, err := tx.Exec(ctx, `INSERT INTO broadcast_recipients (workspace_id, broadcast_id, contact_id, company_id,
		address, status, reason, reply_token, due_at, sent_at, delivered_at, opened_at, clicked_at, replied_at,
		unsubscribed_at, created_at, updated_at)
		SELECT $1, $2, c.id, c.company_id,
			CASE WHEN $3 = 'email' THEN coalesce(lower(c.email), '') ELSE '' END,
			CASE WHEN $3 = 'email' AND c.email IS NULL THEN 'skipped'
				WHEN x.q > $9 THEN 'queued'
				WHEN x.r < $4 THEN 'bounced' ELSE 'delivered' END,
			CASE WHEN $3 = 'email' AND c.email IS NULL THEN 'no_email' WHEN x.q <= $9 AND x.r < $4 THEN '5.1.1' END,
			md5(random()::text || c.id::text), t.at,
			CASE WHEN f.sent THEN t.at END,
			CASE WHEN f.landed THEN t.at + interval '4 seconds' END,
			CASE WHEN f.landed AND x.o < $5 THEN t.at + x.o * interval '2 days' END,
			CASE WHEN f.landed AND x.o < $6 THEN t.at + x.o * interval '2 days' + interval '1 minute' END,
			CASE WHEN f.landed AND x.p < $7 THEN t.at + x.p * interval '3 days' END,
			CASE WHEN f.landed AND x.u < $8 THEN t.at + interval '1 hour' END,
			t.at, t.at
		FROM contacts c,
			LATERAL (SELECT random() AS r, random() AS o, random() AS p, random() AS u, random() AS q, random() AS w) x,
			LATERAL (SELECT now() - make_interval(days => $10::int) + x.w * interval '20 minutes' AS at) t,
			LATERAL (SELECT x.q <= $9 AND NOT ($3 = 'email' AND c.email IS NULL) AS sent) g,
			LATERAL (SELECT g.sent AND x.r >= $4 AS landed, g.sent AS sent) f
		WHERE c.workspace_id = $1 AND c.deleted_at IS NULL AND `+audienceSQL(b.audience),
		s.ws, id, b.channel, b.bounce, b.open, b.click, b.reply, b.unsub, b.sentShare, b.daysAgo)
	if err != nil {
		return fmt.Errorf("seed recipients of %s: %w", b.name, err)
	}

	return nil
}

/*
audienceSQL is the demo audience as a condition on the contact's
attributes.
*/
func audienceSQL(audience string) string {
	return map[string]string{
		"paying": "c.custom_attributes ->> 'plan' IN ('pro', 'enterprise', 'growth')",
		"free":   "c.custom_attributes ->> 'plan' = 'free'",
		"pro":    "c.custom_attributes ->> 'plan' = 'pro'",
		"admins": "c.custom_attributes ->> 'role' = 'admin'",
	}[audience]
}

/*
rulesOf is the demo audience as the rule a broadcast stores.
*/
func rulesOf(audience string) string {
	return map[string]string{
		"paying": `{"rules": {"field": "contact.custom.plan", "op": "in", "value": ["pro", "enterprise", "growth"]}}`,
		"free":   `{"rules": {"field": "contact.custom.plan", "op": "eq", "value": "free"}}`,
		"pro":    `{"rules": {"field": "contact.custom.plan", "op": "eq", "value": "pro"}}`,
		"admins": `{"rules": {"field": "contact.custom.role", "op": "eq", "value": "admin"}}`,
	}[audience]
}
