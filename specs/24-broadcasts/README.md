# 24 Broadcasts

Date 2026-11-10. Owner approved. Read with `specs/00-conventions/README.md`, `specs/04-email-channel/README.md`, `specs/05-chat-app-channels/README.md`, `specs/18-audiences/README.md`.

## Goal

One message, sent once, to many people, on the channel they use with you. B2C teams email or message their users; B2B teams post into each customer's Slack channel or email the admins of each account. Every reply becomes a normal conversation in the inbox, so a broadcast starts conversations instead of ending in a no-reply void.

Competitors: Intercom Outbound (email, chat, post; goals; subscription types), Plain and Pylon broadcasts (Slack, Teams, email; account filters; reactions and replies tracked), Customer.io (time-zone sends, rate limits, frequency caps). Nobody does B2C and B2B well in one tool; dimmi already has the three channels, audiences, events, verified sending domains, Slack threads that become conversations and an AI agent.

## Decisions

| Question | Decision |
|---|---|
| What a broadcast is | one message on one channel, sent once to the audience as it is when sending starts. Ongoing "anyone who matches later" sending is workflows and series, not here |
| Channels | `email`, `chat` (a teammate's message in the Messenger), `post` (an announcement card in the Messenger), `slack` (a post into each customer's Slack channel). Banners are out. Teams later |
| Who sends email | only one of the workspace's connected email addresses, on a verified domain. Never dimmi's own domain, never the forwarding address. No connected verified address, no email broadcast |
| Replies to email | Reply-To carries a reply token on the address's forwarding address, In-Reply-To matches the broadcast's Message-ID. A reply opens a new email conversation on that address, with the broadcast as its first message, routed to the team chosen on the broadcast |
| Replies in Messenger | a chat or post is delivered as a conversation the contact sees in the Messenger, created closed with the broadcast as its first message, so it does not clutter the inbox. A reply reopens it and routes it |
| Replies in Slack | a reply in the thread of the broadcast's post opens a conversation in that room, with the post as its first message |
| Audience | a saved audience (18) or rules, plus channel reach: email needs an address that is not suppressed, chat and post need a contact who has used the Messenger, Slack needs an active customer room (shared or community) whose company has someone in the audience, or rooms picked by hand. Frozen into recipient rows when sending starts |
| B2B | audience rules on company fields and contact role pick who in each account gets email; Slack targets the account's channel. Reports roll up per company |
| Consent | subscription types (`broadcast_topics`), opt-in or opt-out, plus one kind `service` that ignores marketing opt-outs but never suppressions. Every email has one-click unsubscribe (RFC 8058) and a preference page; Slack rooms can be unsubscribed per room |
| Sender in Slack | the bot, or the teammate's own Slack user when they connected it |
| Sending as | email and Messenger come from a teammate (name, avatar); the address is the connected address |
| Scheduling | now, at a time in the workspace's zone, or at a time in each contact's zone (fallback: workspace zone) |
| Pace | each broadcast hands out at most `broadcasts.email_per_minute` emails a minute (default 600), `slack_per_minute` Slack posts (50) and `messenger_per_minute` Messenger conversations (1200). A warm-up ramp for a newly connected address comes later. A Messenger broadcast is a closed conversation the person sees at once when their Messenger is open, unread otherwise |
| Frequency cap | a contact gets at most one non-service broadcast per `broadcasts.min_gap_hours` (default 72); a broadcast can opt out of the cap |
| Auto-pause | during a send, the email broadcast pauses itself when, after at least 100 delivered, bounces pass 5% or complaints pass 0.3%. A paused broadcast says why and can be resumed or stopped |
| Safety | review shows recipients and why others were skipped; test send to yourself (email) or a chosen channel (Slack); cancel before sending starts; stop at any time; Slack and Messenger can be edited and recalled after sending, email cannot |
| Goal | optional event (18 events) or attribute reached within N days of delivery; conversion counted per recipient |
| Tracking | email opens by pixel (marked as estimated), clicks by redirect (switchable per broadcast), replies, unsubscribes, bounces, complaints; Slack reactions and replies; Messenger seen, clicked, replied |
| Live report | counts update in real time over the workspace's realtime channel while sending |
| Variables | Liquid-style `{{ contact.first_name \| default: "there" }}`; keys: contact.first_name, contact.name, contact.email, contact.plan (custom attribute), company.name, sender.first_name. Unknown keys refuse to send |
| AI | Operator can draft or rewrite a broadcast; it never sends. The AI agent may answer replies when the broadcast allows it |
| Permissions | `broadcasts:read`, `broadcasts:write` (draft), `broadcasts:send` (send, stop, resume). Owner and admin hold all; members read |
| Metering | none |

## Data model

- `broadcast_topics` (`btp_`): workspace_id, name, description, consent `opt_in|opt_out`, archived_at.
- `contact_topic_subscriptions`: workspace_id, contact_id, topic_id, subscribed bool, source, updated_at. Absent row means the topic's default (subscribed for opt_out, not for opt_in).
- `broadcasts` (`brd_`): workspace_id, name, channel, status `draft|scheduled|sending|paused|sent|stopped`, pause_reason, audience_id, rules jsonb, room_ids uuid[], topic_id, service bool, subject, preview, body (Markdown), sender_member_id, email_address_id, slack_as `bot|member`, reply_team_id, reply_member_id, ai_replies bool, track_clicks bool, skip_frequency_cap bool, schedule_kind `now|at|local`, scheduled_at, goal jsonb, counts jsonb, started_at, finished_at, timestamps.
- `broadcast_recipients`: workspace_id, broadcast_id, contact_id, company_id, room_id, address, status `queued|sent|delivered|failed|bounced|complained|suppressed|skipped`, reason, reply_token, message_id_header, outbound_email_id, external_ref (Slack ts or conversation id), due_at, sent_at, delivered_at, opened_at, clicked_at, replied_at, reacted_at, unsubscribed_at, converted_at, conversation_id. Unique (broadcast_id, contact_id) and (broadcast_id, room_id).
- `broadcast_links`: broadcast_id, n, url; clicks redirect `/b/c/{token}`.
- `outbound_emails.kind` gains `broadcast`; bounces and complaints already land there and update the recipient.

## Flow

1. Draft and review: `GET /broadcasts/{brd}/estimate` counts reach per reason.
2. Send: status `sending` (or `scheduled`), a job freezes the audience into recipients in pages of 1,000, applying suppression, consent, frequency cap and dedupe by address.
3. Dispatch: `DispatchBroadcast` picks due queued recipients in batches within the pace, enqueues one `SendBroadcastRecipient` job each; idempotent per recipient.
4. Each send updates counts in `broadcasts.counts` and publishes `broadcast.progress`.
5. Bounce, complaint, open, click, reply, reaction and goal events update the recipient and counts; the auto-pause check runs on each bounce or complaint.
6. When no recipient is queued, status `sent`.

## HTTP API (workspace API, `/api/v1`)

- `GET/POST /broadcasts`, `GET/PATCH/DELETE /broadcasts/{brd}` (delete only drafts)
- `POST /broadcasts/{brd}/tests`, `GET /broadcasts/{brd}/estimate`
- `POST /broadcasts/{brd}/send`, `/stop`, `/resume`, `/cancel`, `/duplicate`
- `GET /broadcasts/{brd}/recipients?status=&company=`, `GET /broadcasts/{brd}/report`
- `GET/POST /broadcast-topics`, `PATCH /broadcast-topics/{btp}`
- Public: `GET /b/u/{token}` (preference page), `POST /b/u/{token}/one-click` (RFC 8058), `GET /b/o/{token}.gif` (open), `GET /b/c/{token}` (click)
- Widget: broadcasts are delivered as conversations; the first message carries `metadata.broadcast` with `display` `message` or `post`.
- Preference page: `/b/u/{token}` opens `/email-preferences?t={token}` in the app, which reads and saves through `GET/PATCH /api/v1/broadcast-preferences/{token}`; mail apps post `/b/u/{token}/one-click`.

## Done means

Seeded demo data and screenshots of: an email broadcast arriving in Mailpit with unsubscribe headers, a reply becoming a conversation, a Messenger chat and post in the widget, a Slack broadcast in hivy with a reply becoming a conversation and a reaction counted, the report filling live during a send, a send auto-paused by bounces, the per-company report, the preference page.
