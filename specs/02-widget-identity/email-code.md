# 02 Widget identity: email code proof for anonymous visitors

Style: caveman. Decided 2026-11-07 after studying Intercom (Data connectors OTP), Zendesk, Ada and Decagon.

## Why

Many sites never wire identity tokens: marketing sites, docs, stores with guest checkout. Their visitors still ask "where is my order?". Intercom answers with an email one-time passcode before a data connector runs: 6 digits, 10 minutes, 3 tries, one proof covers 30 minutes, code typed into the conversation. Zendesk and Ada only collect an unverified email. Decagon argues for proof that grows with risk.

dimmi beats this on three points:

| Point | Intercom | dimmi |
|---|---|---|
| Where the code goes | typed as a chat message, so it passes through the model and the transcript | typed into a dimmi-rendered card; never a message, never in a prompt, never in the transcript (defeats the OTP-theft prompt injections shown at DEF CON 34) |
| What it unlocks | the one connector, then 30 minutes | the session: every action bound to `{contact.email}` for 24 hours, and the lead's email becomes verified |
| How the agent asks | workflow step authored by the admin | automatic: an action bound only to `{contact.email}` is offered to an anonymous visitor; calling it shows the card, the call runs after the code |

## Model

- Proof belongs to the widget session: `widget_sessions.verified_email`, `email_verified_at` (migration `20261201090003`). Lasts `EmailProofLifetime` 24 h.
- Proof level `email` sits below a signed-in session. In an email-proven context only `{contact.email}` resolves; `external_id` and custom attributes still need a signed-in session (same rule as aligned-DMARC email, actions.md).
- On success the lead's `email` is set to the proven address with `email_verified = true`. Typing another email through `PATCH /widget/v1/contact` drops `email_verified`.
- No merge into an existing customer holding that email: proving an inbox shows nobody else's history.

## Code rules

| Rule | Value |
|---|---|
| Code | 6 digits from crypto/rand, stored only as SHA-256 bound to the session, in Valkey, 10 minutes |
| Tries | 5 per code, then the code is burned (`email_code_locked`) |
| Sends | 1 per 30 s and 5 per hour per session; 5 per hour per address per workspace; 20 per hour per IP prefix |
| Mail | from the install's sender, subject "123456 is your <workspace> code", in the visitor's language |
| Signed-in session | refused `identity_already_verified`: a token is stronger proof |
| Errors | `email_code_expired` (410), `email_code_invalid` (422, `attempts_left`), `email_code_locked` (429), `too_many_attempts` (429, `retry_after`) |

## API

- `POST /widget/v1/email-verifications {email}` → 202 `{email: "a***@acme.com", expires_at, resend_after}`.
- `POST /widget/v1/email-verifications/checks {code}` → 200 session (with `email_verified: true`), and every call of the visitor waiting for the proof moves on.

## Agent actions

- New call state `awaiting_verification`. An action whose only contact reference is `{contact.email}` is offered to an anonymous widget visitor without proof; calling it records the call in that state with the arguments frozen and no bindings, and the reply carries a verify card.
- After the proof, each waiting call of the contact binds `contact.email` to the proven address and moves on as if just proposed: to `awaiting_confirmation` when the customer confirms writes, `pending_approval` when a teammate approves, else straight to execution; the follow-up run tells the customer the result.
- Expiry, supersede and cancel treat `awaiting_verification` like `awaiting_confirmation`.
- Messages sent from an email-proven session are stamped `metadata.widget.email_verified`, so later turns bind `{contact.email}` directly until the proof lapses.

## Widget

- Card state `verify`: email field (prefilled with what the visitor typed before), "Send code", then a six-box code field with paste support and `autocomplete="one-time-code"`, "Resend" after 30 s, "Use another email", "Cancel".
- After the proof the card turns into the action's normal card or its result line.
