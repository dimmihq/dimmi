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
demoSeed holds the workspace, sender member and seeded broadcast ids for one
broadcast seeding run.
*/
type demoSeed struct {
	ws, member ids.UUID
	made       []ids.UUID
}

/*
topics seeds the lists people subscribe to.
*/
func (s *demoSeed) topics(ctx context.Context, tx *database.Tx) error {
	_, err := tx.Exec(ctx, `INSERT INTO broadcast_topics (workspace_id, name, description, consent) VALUES
		($1, 'Product updates', 'New features and improvements, about once a month.', 'opt_out'),
		($1, 'Webinars and events', 'Invitations to live sessions and meetups.', 'opt_in'),
		($1, 'Offers', 'Discounts and plan offers.', 'opt_in')
		ON CONFLICT DO NOTHING`, s.ws)
	if err != nil {
		return fmt.Errorf("seed topics: %w", err)
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
