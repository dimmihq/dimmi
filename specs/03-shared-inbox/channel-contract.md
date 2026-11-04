# Channel contract

Style: caveman. Owned by 03. Implemented by 01 (widget), 04 (email), 05 (slack, discord, telegram, whatsapp), and core `api` driver here. Contract test suite `tests/contracts/channel_test.go` every driver must pass.

Two directions:
- Outbound: core calls driver `Send` (interface below) from `send_channel_message` job.
- Inbound: driver (webhook controller, gateway worker, socket/poller, widget API) calls core action `ReceiveInboundMessage`. Drivers never write conversations/messages/contacts tables directly.

## 1. Interface (`app/contracts/channel.go`)

```go
package contracts

import (
    "context"
    "errors"
    "io"
    "time"

    "github.com/dimmihq/dimmi/app/enums"
    "github.com/dimmihq/dimmi/app/models"
    "github.com/dimmihq/dimmi/framework/database"
    "github.com/dimmihq/dimmi/framework/ids"
)

// Channel is one conversation channel adapter (widget, email, slack, discord, telegram, whatsapp, api).
// One instance per type, stateless, safe for concurrent use. Per-connection state comes in via conn.
type Channel interface {
    Type() enums.ChannelType
    // Capabilities may depend on connection config (e.g. WhatsApp tier, Slack plan). Must be cheap (no network).
    Capabilities(conn *models.ChannelConnection) ChannelCapabilities
    // Send delivers one outbound message. Called at most once per attempt; may be retried by job.
    // Must be idempotent for same msg.IdempotencyKey where provider allows (pass key to provider).
    // Must respect ctx deadline (inbox.outbound.send_timeout).
    Send(ctx context.Context, conn *models.ChannelConnection, msg OutboundMessage) (SendResult, error)
}

type ChannelCapabilities struct {
    Transport            Transport     // TransportPush or TransportRealtime or TransportNone
    StartConversation    StartMode     // StartNever, StartAlways, StartWithTemplate
    Formatting           Formatting    // FormatMarkdown, FormatHTML, FormatPlain, FormatSlackMrkdwn, FormatDiscordMarkdown, FormatTelegramHTML, FormatWhatsApp
    MaxBodyChars         int           // after formatting; 0 = no limit. Over limit: core rejects 422 (never truncates silently)
    Subject              bool          // email
    Attachments          bool
    MaxAttachments       int           // 0 with Attachments=true means use global limit (10)
    MaxAttachmentBytes   int64         // per file; 0 = global (25 MB)
    MaxTotalAttachmentBytes int64      // 0 = no extra limit (email ~ 25 MB total)
    AllowedContentTypes  []string      // nil = any not blocked by security rules; supports "image/*"
    Typing               bool          // driver implements ChannelTyping
    DeliveryReceipts     bool          // driver will call UpdateMessageDelivery with delivered
    ReadReceipts         bool          // driver will call UpdateMessageDelivery with read
    ParticipantsCC       bool          // email To and CC beyond primary
    ParticipantsBCC      bool          // email Bcc per send (records.md)
    MessagingWindow      time.Duration // 0 = none; WhatsApp 24h since last customer message
    RetryWindow          time.Duration // transient-error retry budget for send_channel_message; 0 = core default (inbox.outbound.retry_window, about 3 h); email (04) about 24 h
}

type Transport string
const (
    TransportPush     Transport = "push"     // core enqueues send_channel_message; delivery_status starts pending
    TransportRealtime Transport = "realtime" // widget: message visible via realtime/REST; status starts sent; no job
    TransportNone     Transport = "none"     // api: developer reads via webhook/REST; status not_applicable; no job
)

type StartMode string
const (
    StartNever        StartMode = "never"         // business cannot open thread (Telegram, Discord DM unless prior, Slack)
    StartAlways       StartMode = "always"        // email
    StartWithTemplate StartMode = "with_template" // WhatsApp: needs channel_options.template
)

type OutboundMessage struct {
    WorkspaceID       ids.UUID
    ConversationID    ids.UUID
    ConversationNumber int64
    MessageID         ids.UUID
    IdempotencyKey    string            // "msg_<public id>"; stable across retries
    ExternalThreadKey string            // empty when conversation has no thread yet (first outbound)
    Subject           string            // conversation subject; email uses, others ignore
    BodyMarkdown      string            // canonical Markdown, saved reply variables already rendered
    BodyText          string            // plain-text rendering (app/support/excerpt full mode, on markdown.PlainText)
    BodyHTML          string            // safe HTML rendering (goldmark safe); email uses
    Recipient         Recipient
    To                []Recipient       // added To participants, only when ParticipantsCC
    CC                []Recipient       // only when ParticipantsCC
    BCC               []Recipient       // this send only, only when ParticipantsBCC; never stored by driver on thread
    Author            OutboundAuthor
    Attachments       []OutboundAttachment
    ReplyTo           []string          // channel_message_ids of earlier messages in thread, newest last (email References/In-Reply-To, Slack thread)
    Options           []byte            // raw JSON channel_options from request (e.g. WhatsApp template); driver validates; nil when absent
    IsTest            bool              // driver may short-circuit in sandbox connections
}

type Recipient struct {
    ContactID ids.UUID
    Name      string
    // Identity chosen by core for this connection type: email, slack_user, discord_user, telegram_user, whatsapp_wa_id, whatsapp_bsuid (X1), widget_visitor.
    IdentityType  enums.IdentityType
    IdentityValue string
    Locale        string
}

type OutboundAuthor struct {
    Type      enums.AuthorType // member, agent, system
    ID        ids.UUID
    Name      string           // display: member name, agent name, workspace name for system
    AvatarURL string
}

type OutboundAttachment struct {
    ID          ids.UUID
    Filename    string
    ContentType string
    SizeBytes   int64
    SHA256      string
    Inline      bool
    ContentID   string
    Open        func(ctx context.Context) (io.ReadCloser, error) // streams from Filesystem; caller closes
    SignedURL   string // 1 h URL for channels that fetch by link (WhatsApp media by link, Slack unfurl)
}

type SendResult struct {
    ChannelMessageID  string               // required on success (provider id); empty only for TransportNone
    Status            enums.DeliveryStatus // sent or delivered
    ExternalThreadKey string               // non-empty when this send created thread (first email Message-ID, Slack ts); core stores if conversation key empty
    SentAt            time.Time            // provider timestamp if given, else zero (core uses a.Clock.Now)
    Metadata          map[string]any       // stored under messages.metadata.<channel type>; max 4 KB; Metadata["parts"] = extra provider ids (X4)
}

// Optional capabilities. Core type-asserts. Capabilities flags must match implemented interfaces (contract test checks).
type ChannelTyping interface {
    SendTyping(ctx context.Context, conn *models.ChannelConnection, thread ThreadRef) error
}
type ChannelReadMarker interface {
    // Called when member reads conversation (conversation.read) for channels that support business read receipts (WhatsApp mark-as-read).
    MarkRead(ctx context.Context, conn *models.ChannelConnection, ref MessageRef) error
}

// ChannelRehomer: optional, for connection types that cannot send (16 `import`). ReplyToConversation calls Rehome
// in reply tx before validation; returned connection becomes conversation channel_connection_id (`conversation.updated`).
// Errors: ErrRehomeUnavailable (no live connection or contact unreachable) -> 422 import_channel_cannot_send.
type ChannelRehomer interface {
    Rehome(ctx context.Context, tx *database.Tx, conv *models.Conversation) (*models.ChannelConnection, error)
}

type MessageRef struct {
    MessageID         ids.UUID
    ChannelMessageID  string
    ExternalThreadKey string
}
type ThreadRef struct {
    ConversationID    ids.UUID
    ExternalThreadKey string
    Recipient         Recipient
}

// Errors. Drivers wrap with fmt.Errorf("...: %w", contracts.ErrX). Unwrapped errors = transient (retry).
var (
    ErrPermanent          = errors.New("channel: permanent failure")        // fail now, no retry
    ErrRecipientUnreachable = errors.New("channel: recipient unreachable")  // permanent; e.g. user blocked bot, invalid address, hard bounce at SMTP time
    ErrWindowClosed       = errors.New("channel: messaging window closed")  // permanent; WhatsApp 24h
    ErrConnectionBroken   = errors.New("channel: connection credentials invalid or revoked") // permanent for message; core marks connection status error
    ErrContentRejected    = errors.New("channel: content rejected by provider") // permanent; too long, bad media
    ErrUnsupported        = errors.New("channel: operation unsupported")
    ErrRehomeUnavailable  = errors.New("channel: no live connection to rehome to")
)

// RateLimitedError: transient, core snoozes job for RetryAfter (does not consume attempt).
type RateLimitedError struct{ RetryAfter time.Duration }
func (e *RateLimitedError) Error() string { return "channel: rate limited" }

// SendError carries provider code for delivery_error and metrics. Never include message body or secrets.
type SendError struct {
    Code    string // stable snake_case: "smtp_550", "telegram_403_blocked", "whatsapp_131047"
    Message string // human readable, shown to members
    Err     error  // one of sentinel errors above, or nil (= transient)
}
func (e *SendError) Error() string { return e.Code + ": " + e.Message }
func (e *SendError) Unwrap() error { return e.Err }
```

Driver registration: `app/providers/app_service_provider.go` registers each driver on `ChannelManager` (`channels.Register(widget.New(...))`). Core ships `app/drivers/channel/api` (TransportNone, StartAlways, no capabilities) and `app/drivers/channel/fake` (tests).

### Expected capability profile (informative; owning spec verifies against provider docs)

| Type | Transport | Start | Typing | Receipts | Window |
|---|---|---|---|---|---|
| widget (01) | realtime | always (contact known) | yes | read | none |
| email (04) | push | always | no | delivery via bounce/complaint only | none |
| slack (05) | push | never | owning spec decides | no | none |
| discord (05) | push | never | yes | no | none |
| telegram (05) | push | never | yes | no | none |
| whatsapp (05) | push | with_template | owning spec decides | delivered, read | 24 h |
| api (03) | none | always | no | no | none |
| import (16) | none (never sends; `ChannelRehomer` moves reply to inbox email connection, 04 rules) | never | no | no | none |

Edits and deletes never propagate to providers at MVP (later: message-edit). Delete is local (api.md "Note edit, delete").

## 2. Outbound pipeline

```
ReplyToConversation (tx)
  |- ChannelRehomer.Rehome when routed driver implements it (16 import)
  |- validate vs Capabilities(routed connection)       -> 422/409 before insert
  |- insert message (delivery_status per Transport, channel_connection_id = routed)
  |- update conversation counters, auto-assign, then-status
  |- events.Dispatch(MessageCreated, ...)              -> after commit: BroadcastToRealtime
  |                                                    -> InsertTx: SendChannelOutbound listener job (only Transport push, type message, author not contact, not metadata.imported)
send_channel_message worker (queue mail|channels)
  |- re-fetch message, conversation, connection
  |- guards (below)
  |- driver.Send(ctx with timeout)
  |- UpdateMessageDelivery (tx) -> message.delivery_updated
```

### Routing rule (`RouteOutboundChannel`)
1. Channel = `channel_connection_id` of latest `type = message` authored by contact in conversation that has non-null connection of a sendable type (reply follows customer; `import` connections skipped).
2. None (member-started or imported-only conversation): conversation `channel_connection_id` (after `Rehome` for imported).
3. Chosen connection soft-deleted or `disconnected`: fall back to conversation origin if active; else 409 `channel_disconnected` at request time (nothing stored).
4. Recipient identity: contact identity of connection's type (`email` for email, `telegram_user` for telegram, ...) preferring verified, newest. None: 422 `contact_unreachable_on_channel`.

### Request-time validation (before insert)
- `MaxBodyChars` (measured after formatting by driver helper `contracts.FormatLength(caps.Formatting, markdown)`): 422 on `body`.
- Attachments: count, per-file size, total size, allowed types: 422 on `attachment_ids.N` with channel name in message ("Telegram accepts files up to 50 MB.").
- `MessagingWindow`: `now - last_customer_message_at > window` and no `channel_options.template`: 409 `channel_window_closed` with `window_closed_at`.
- `StartConversation = never` and conversation has no contact message on routed connection and no thread key: 422 `channel_cannot_initiate`.

### Worker guards (in order)
1. Message missing, deleted, or `delivery_status` not `pending`: exit (idempotent no-op). Deleted before send: set `failed`, `delivery_error = "cancelled"`, `final = true` not billed.
2. Connection status `disconnected` or soft-deleted: `failed` (`channel_disconnected`), final.
3. Connection status `error`: snooze 60 s; after `inbox.outbound.max_attempts` worth of snoozes (10 min total), `failed` (`channel_error`).
4. Ordering: earlier message in same conversation with `delivery_status = pending` and same connection exists: snooze 2 s; give up waiting after `ordering_wait_max` (2 min) and send anyway (log warn).
5. Attachments with `scan_status = pending`: snooze 10 s up to `scan_wait_max`; then `failed` (`attachment_scan_timeout`). Any `infected`: `failed` (`attachment_infected`).
6. Workspace `suspended_at` set: snooze 5 min, up to 24 h, then `failed` (`workspace_suspended`).

### Result handling
| Driver outcome | Message | Job |
|---|---|---|
| success | `delivery_status = result.Status`, `channel_message_id`, `sent_at`, metadata; conversation `external_thread_key` set if empty and returned (unique conflict: keep existing, log) | complete |
| `RateLimitedError` | stays pending | `queue.Postpone(RetryAfter)` (min 1 s, max 15 min) |
| transient error | stays pending, `delivery_error` = last error message (visible "retrying") | River retry with backoff until `Capabilities(conn).RetryWindow` (0 = default about 3 h) elapsed since message `created_at`; then `failed`, final |
| `ErrConnectionBroken` | `failed` | connection status `error` + `last_error` via 05's `MarkConnectionErrored` action, emits `channel.errored` |
| other permanent | `failed`, `delivery_error = SendError.Message`, `metadata.delivery_error_code` | complete |

`message.delivery_updated` payload: `{message_id, conversation_id, from, to, error_code, final}`. `final = true` when status `failed` and no further retry scheduled. 12 credits D13 when final failure and no other reply delivered in window.

### Manual retry
`POST /conversations/{c}/messages/{m}/delivery-attempts` (202, `delivery_attempt` resource): only `failed` messages authored by workspace (member/agent/system) on push channels; re-runs request-time validation (window may have closed); sets `pending`, clears error, inserts job. Same message id, same `IdempotencyKey` (provider-side dedupe where supported).

### Deletes of outbound messages
- Reply still `pending`: delete cancels send (guard 1). Sent: local delete only; widget hides it; push channel copy stays (no provider call).

## 3. Inbound contract (`app/actions/conversations/receive_inbound_message.go`, `conversations.ReceiveInboundMessage`)

```go
type InboundMessage struct {
    ConnectionID       ids.UUID          // required; resolves workspace + inbox
    DedupeKey          string             // required; "<type>:<provider scoped id>", e.g. "email:<Message-ID>", "telegram:<bot_id>:<update_id>"; max 255
    ExternalThreadKey  string             // required for push channels; empty for widget/api
    ConversationID     *ids.UUID         // when adapter knows target (widget conversation, email reply token address)
    ForceNewConversation bool             // widget "start new conversation"; ignored if ConversationID set
    Sender             InboundSender      // required
    Participants       []InboundSender    // CC / extra thread members; max 50 kept
    Subject            string             // email subject (trimmed 255)
    Body               string             // Markdown (adapter converts); max 64 KB, longer truncated with marker + full original kept as attachment "message.txt"
    BodyHTML           string             // raw provider HTML (email); core sanitizes (`framework/sanitize`, 04 email policy) before store
    Attachments        []InboundAttachment
    ChannelMessageID   string
    ReplyToChannelMessageID string        // quoted/replied message (Slack thread parent, Telegram reply_to)
    SentAt             time.Time          // provider timestamp; core stores in metadata, uses a.Clock.Now for created_at
    AutoGenerated      bool               // OOO, auto-reply, bounce notice: substantive=false, no reopen, no unsnooze, no waiting_since, no notify
    SpamVerdict        enums.SpamVerdict  // unknown | ham | spam (adapter/provider verdict, e.g. SES spam verdict)
    HoneypotTripped    bool               // widget honeypot
    Source             *models.ConversationSource // used only when new conversation created
    IsTest             bool               // widget test mode -> conversations.is_test (03 column)
    Metadata           map[string]any     // stored under messages.metadata.<channel type>; max 8 KB
    // AfterPersist: optional driver hook, called inside ReceiveInboundMessage tx after message insert (step 9b).
    // Driver writes own rows (email_message_refs, channel_message_refs, channel_contact_states...) atomically with ingest.
    // Error aborts whole ingest (rollback); not called on Duplicate. Keep fast, no network.
    AfterPersist       func(ctx context.Context, tx *database.Tx, conv models.Conversation, msg models.Message) error
}

type InboundSender struct {
    ContactID   *ids.UUID       // when adapter already authenticated contact (widget session, 02 JWT)
    Identities  []IdentityClaim  // at least one unless ContactID set
    Name, Email, Phone, AvatarURL, Locale, Timezone string
    Kind        enums.ContactKind // user only when verified (JWT/API); default lead
    ExternalID  string            // only from verified source (02)
    CustomAttributes map[string]any // only from verified source; validated by NormalizeCustomAttributes
    CompanyClaim *CompanyClaim      // 02 JWT company claim
}

type IdentityClaim struct {
    Type     enums.IdentityType // registry enum (P1); MVP: email, phone, widget_visitor, slack_user, discord_user, telegram_user, whatsapp_wa_id, whatsapp_bsuid (X1), external_id
    Value    string             // normalized by adapter: email lowercased, phone E.164, provider ids as-is
    Verified bool               // email: DKIM/SPF aligned pass (04) or JWT (02); phone: WhatsApp wa_id; provider user ids always true
}

type InboundAttachment struct {
    Filename, ContentType string
    SizeBytes  int64
    Inline     bool
    ContentID  string
    Open       func(ctx context.Context) (io.ReadCloser, error) // or AttachmentID when already stored (widget upload)
    AttachmentID *ids.UUID
}

type InboundResult struct {
    MessageID       ids.UUID
    ConversationID  ids.UUID
    ContactID       ids.UUID
    Duplicate       bool   // dedupe hit: no changes, existing ids returned
    ConversationCreated bool
    Reopened        bool
    Spam            bool
}
```

### Algorithm (one transaction; attachment bytes stored before tx, orphan purge cleans on rollback)
1. Load connection (`FOR SHARE`); missing/deleted: `ErrConnectionGone` (adapter returns 200 to provider, logs). Workspace suspended: store nothing, return `ErrWorkspaceSuspended` (adapter acks provider; foundation: "blocks inbound processing except storage" means raw event stored by adapter, replay later by 11/12 tooling).
2. Dedupe: `SELECT ... WHERE workspace_id = $1 AND dedupe_key = $2`; hit: return existing (`Duplicate`). Insert race: unique violation caught, re-select, return `Duplicate`.
3. Resolve contact (`ResolveInboundContact`): ContactID given: use (follow `merged_into_id` chain). Else lookup identities in order verified first; found: that contact (follow merge chain). Else create lead (or user if Kind user) + identities. Attach new claims: identity value owned by other contact: if both verified and auto-merge allowed (section 4) merge; verified claim vs unverified row: takeover (section 4); else skip identity (log). Update name/avatar only if empty (inbound never overwrites member-edited fields), `last_seen_at = now`.
4. Pick conversation:
   - `ConversationID` set: that conversation (follow `merged_into_id`); must belong to same workspace and contact or be participant; else ignore hint.
   - Else by `(channel_connection_id, external_thread_key)`: newest row (any status) via `conversations_thread_lookup_idx`, follow merge chain.
   - Else (no key or no match or `ForceNewConversation`): create new.
   - Found + `closed` + `closed_at < now - inbox.reopen_window_days` (or window 0): create new with `previous_conversation_id` = found.
   - Found + deleted: create new with `previous_conversation_id`.
5. Spam decision: contact `blocked_at` set, or `SpamVerdict = spam`, or `HoneypotTripped`, or `app/support/spam` heuristics score over threshold: new conversation created with `spam = true`; existing non-spam conversation gets message but `spam` NOT flipped automatically (avoid hiding real thread), message `metadata.spam_suspected = true`.
6. New conversation: allocate number, `status = open`, inbox = connection inbox, `assignee_team_id = inbox.default_team_id`, subject (Subject or excerpt), source, `is_test` rules, emit `conversation.created`.
7. Insert message: `type = message`, `author_type = contact`, `author_id = contact`, `substantive = !AutoGenerated`, `channel_connection_id`, `delivery_status = not_applicable`, sanitized `body_html`, attachments linked.
8. State effects (skip all when `AutoGenerated` or conversation spam):
   - `snoozed`: to `open`, `snoozed_until = null`, emit `conversation.unsnoozed` reason `customer_message`.
   - `closed` (within window): to `open`, `closed_at = null`, emit `conversation.reopened` reason `customer_message`. Assignee kept.
   - `waiting_since` set if null; `last_customer_message_at`, `first_customer_message_at` if null.
   - Always: `last_message_at`, `last_activity_at`, `last_message_id`, `message_count`.
9. Participants: upsert `conversation_participants` role `cc` for resolved participant contacts (max 50).
9b. `AfterPersist` (when set): driver hook with same tx, final conversation and message. Error: rollback, `ReceiveInboundMessage` returns error; adapter answers provider non-2xx (or job retries), redelivery hits no dedupe row (rolled back) and ingests cleanly. Driver rows keyed by message id or dedupe key, never duplicated.
10. Emit `message.created` (+ `contact.created`/`contact.updated`/`contact.merged` as applicable).

### Inbound edits and deletes
Not applied at MVP: drivers drop provider edit/delete events before ingest (later: message-edit). Stored message keeps body as first received.

### Delivery updates from channels
`UpdateMessageDelivery{MessageID? | (ConnectionID, ChannelMessageID), Status, ErrorCode, ErrorMessage, OccurredAt}`. Monotonic order `pending < sent < delivered < read`; regress ignored. `failed` accepted from `pending`, `sent`, `delivered` (late bounce), never from `read`. Same status repeated: no-op, no event. Lookup miss: ignore.

## 4. Auto-merge rules (`AutoMergeOnVerifiedIdentity`)

Triggered when verified identity claim (type, value) arrives for contact A but identity row belongs to contact B.
- Allowed only when claim verified AND existing identity row verified.
- Both contacts have non-null different `external_id`: never merge (different host users share email); log `dimmi_contact_merge_blocked_total{why="external_id_conflict"}`.
- Survivor: contact with `external_id` (kind user); else older `created_at`.
- `reason`: `verified_email`, `verified_external_id`, `verified_phone`.
- Unverified claim matching verified identity: never merge, never attach; new lead stays separate; `find_duplicate_contacts` surfaces pair to members.

Verified-claim takeover (`ResolveInboundContact`, requested by 14; also correct for 04 DMARC-aligned inbound mail and 02 JWT `email_verified`):
- Verified claim (type, value) arrives for contact A; identity row with same (type, value) is **unverified** and held by contact B (typed email on widget lead, unverified relay): row moves to A, `verified = true`. B keeps `contacts.email` text (display only, no identity row). `contact.updated` on both (B `changes.identities`, A `changes.identities` + `email_verified`). No merge, no conversation move: B's history never joins A (leak rule 02 R2/R3 intact).
- Claim itself unverified: no takeover (existing rule above).
- Row verified and held by B: auto-merge rules above (never takeover).
- Used by 14 portal email sign-in (`IdentifyContact` / magic code), 04 inbound mail with DMARC-aligned pass (`IdentityClaim.Verified = true`), 02 JWT `email_verified: true`.
- Metric `dimmi_contact_identity_takeovers_total{type}`; audit row (actor system, from/to contact).

## 5. Additions for 05 (X1, X3-X5)

Requested by `05-chat-app-channels/README.md` ("Extensions 05 needs from 03"); canonical per `00-foundation/contracts.md` Channel section. 05 code assumes these names.

### X1. Identity type `whatsapp_bsuid`
- `enums.IdentityType` adds `whatsapp_bsuid` (Meta business-scoped user id; `wa_id` absent when user enabled username, since April 2026).
- `contact_identities.type` is registry enum (`00-foundation/extensibility.md` P1: `text` + format CHECK, Go registry validation). 05 registers `whatsapp_bsuid` in `enums.IdentityType` registry at boot; no migration, no CHECK change. Any later identity type (SMS, Instagram, Teams, portal) same path.
- `Recipient.IdentityType` and `IdentityClaim.Type` may be `whatsapp_bsuid`. Verified = true (provider id). 05 keys WhatsApp threads on BSUID.

### X2. `InboundMessage.Echo`
Removed from MVP (later: teammate-echo). Teammate native replies inside provider threads are not ingested (05 drops them before ingest).

### X3. Optional `ChannelWindowResolver`
```go
type ChannelWindowResolver interface {
    WindowState(ctx context.Context, conn *models.ChannelConnection, recipient Recipient) (open bool, closesAt time.Time, err error)
}
```
- Core type-asserts. Request-time window check in `ReplyToConversation` uses it when implemented, else `last_customer_message_at + Capabilities.MessagingWindow`. WhatsApp window is per (business number, user), counts any user message incl. other conversations.
- Closed + no `channel_options.template`: 409 `channel_window_closed` (`data.window_closed_at = closesAt`); dashboard composer switches to template picker (05 templates).
- Send-time race (window closed between request and job): driver returns `SendError{Code: "whatsapp_window_closed", Err: ErrWindowClosed}`; message `failed`, final; UI shows retry-with-template.
- Resolver error: treat window as open, rely on send-time `ErrWindowClosed`.

### X4. Extra provider message ids
- Split messages and Slack file shares produce several provider ids (receipts need all). Part 0 id = `SendResult.ChannelMessageID` (`messages.channel_message_id`); driver writes all part ids to 05 table `channel_message_refs` itself (same step as provider response) and lists them in `SendResult.Metadata["parts"]`.
- 03 lookups by `(connection, channel_message_id)` (`UpdateMessageDelivery`, `FindConversationByChannelMessage`) accept ids resolved by driver via `channel_message_refs` to owning message id; 03 never reads that table directly.

### X5. Driver-created thread key at ingest
- Driver may create provider thread before calling core and pass resulting key as `InboundMessage.ExternalThreadKey`. `ReceiveInboundMessage` accepts it unchanged: lookup by `(channel_connection_id, external_thread_key)`, miss creates conversation storing key. Already true per section 3 (key is input); contract test covers key unseen before. No MVP driver needs it (Discord thread-at-ingest deferred with Discord thread modes, 05).

