# Scope audit: MVP specs vs the 4 target competitors

Date: 2026-11-02. Read-only audit of `specs/*/README.md` "Scope / In (MVP)" sections against Intercom (incl. Fin), Productlane, Featurebase, Ferndesk.

PO rule: research only these 4; remove features none of them offer, unless PO explicitly asked for them.

Sources: `research/competitors/{fin,productlane,featurebase,ferndesk}.md` plus fresh doc checks on 2026-11-02 (links inline). Abbreviations: **IC** = Intercom/Fin, **PL** = Productlane, **FB** = Featurebase, **FD** = Ferndesk. Y = yes, P = partial, N = no, U = unclear (no doc found; not proof of absence).

Note: spec folder `16-announcements-surveys` was renamed to `15-announcements-surveys` during this audit (status page gone). Both names refer to same spec.

Weight = rough grep count of mentions in that spec's `edge-cases.md` / `tasks.md` (ec / tasks). S = small, M = medium, L = large.

## 1. TL;DR cut list (none of the 4 offer it, assistant added it)

| # | Feature | Spec | Evidence none offer | Weight |
|---|---|---|---|---|
| C1 | IMAP mailbox adapter (Gmail, Fastmail, iCloud, Zoho, Yahoo; IDLE + poll) | 04 In: "IMAP adapter" | IC forwarding only ([9744849](https://www.intercom.com/help/en/articles/9744849-connect-your-email-support-channel)); FB forwarding only ([6551225](https://help.featurebase.app/articles/6551225-connect-your-email-support-channel)); PL forwarding only ([docs](https://productlane.com/docs/channels/email-forwarding)); FD no email channel | L (ec 16 / tasks 11; own driver, IDLE connections, memory budget) |
| C2 | Customer-visible message edit, plus edit/delete propagation to Slack/Discord/Telegram/WhatsApp | 03 In: "edit within window"; 05 In: "Edits and deletes both ways" | IC replies cannot be edited, only deleted ([inbox FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs)); FB U; PL U (has undo send instead); FD no inbox | M-L (03 ec 13; 05 edit/delete ec ~30 / tasks 12) |
| C3 | RS256 and EdDSA (Ed25519) uploaded public keys for widget identity | 02 In: `identity_secrets` | IC HS256 only ([JWT doc](https://www.intercom.com/help/en/articles/10589769-authenticating-users-in-the-messenger-with-json-web-tokens-jwts)); FB HS256 ([5257986](https://help.featurebase.app/articles/5257986)); PL HS256 ([widget](https://productlane.com/docs/setup/widget)) | S-M (ec 3 / tasks 1, plus key upload API, alg pinning) |
| C4 | Single-use JWT `nonce` with Valkey replay cache | 02 In: verification service | None document nonce replay protection (same JWT docs) | S (ec 8 / tasks 7) |
| C5 | Adaptive proof-of-work (ALTCHA) on anonymous widget sessions, bot score | 01 In: anti-abuse | IC none, suggests require-email workaround; FB require-email + ban (P); PL JWT + blocklist (P) | S-M (ec 3 / tasks 1; Web Worker solver, 428 flow). Keep origin allowlist, rate limits, honeypot (security hygiene) |
| C6 | `dimmi listen --forward-to localhost` (SSE stream of signed events to local dev) | 10 In; 09 CLI | IC/FB/PL/FD none (Stripe CLI pattern, not a competitor) | M (10 ec 8 / tasks 6, plus 09 client) |
| C7 | Bulk webhook "recover all failed since timestamp" | 10 In | PL single replay only ([webhooks](https://productlane.com/docs/webhooks)); IC no manual redeliver; FB says backfill via API | S (ec 5 / tasks 3). Keep single redeliver (PL has it) |
| C8 | Changelog weekly digest cadence (per subscriber override) | 13 In: email subscribers | FB U, PL N, IC news not emailed, FD N | S-M (ec 5 / tasks 4; second mailing scheduler) |
| C9 | Atom and JSON Feed (RSS stays) | 13 In: public site | FB RSS, PL RSS, FD RSS only; IC no feed | S (ec 2 / tasks 3) |
| C10 | Portal `.md` twins for feedback pages and roadmap | 14 In: public portal | FB, PL, FD do `.md` for docs only (FD/FB help center); none for feedback boards | S (ec 5 / tasks 6, shared with llms entries) |
| C11 | AI summary of open-text survey answers (themes + quotes) | 15 In: surveys | IC U ("summaries only for conversations"); FB U; PL N (no surveys); FD N | M (ec 5 / tasks 4; BYOK call, spend cap, caching) |
| C12 | Dimmi-hosted survey page (signed/public link, works without JS, daily cap for unpaid) | 15 In: "hosted link page" | IC P: link opens your own page with Messenger ([6059151](https://www.intercom.com/help/en/articles/6059151-multiple-ways-to-share-a-survey)); FB N in-app only; PL N; FD N | M (ec 6 / tasks 4; plus 12 `HasPaid` cap seam) |
| C13 | `anonymize_responses` survey setting | 15 In | None found in IC/FB survey docs | S (ec 4) |
| C14 | Product-market-fit survey template | 15 In: templates | Not in IC survey types ([5973834](https://www.intercom.com/help/en/articles/5973834-intercom-s-survey-types)); FB templates NPS/CSAT/CES only | Trivial (template only) |
| C15 | Teammate "is typing" relay between teammates (collision) | 03 In: collision detection | IC shows who is viewing, no teammate typing ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained)); FB/PL U | S (ec 7 / tasks 2). Keep "is viewing" (IC has it) and 409 `new_messages_since` guard (quality) |
| C16 | Next.js/SvelteKit/Nuxt/Astro/Laravel/gem/pip/Go identity helper packages (9 stacks) | 02 In: snippets API | All 4 publish copy-paste snippets only, no per-stack helper packages | M (package maintenance cost). Suggest: keep raw snippets + one Node/Next helper used by `dimmi init` |

Total cut candidates: 16. Biggest wins by weight: C1 IMAP, C2 message edit propagation, C6 `dimmi listen`, C11/C12 survey AI summary + hosted page, C16 helper packages.

## 2. Borderline list

One competitor partial, coverage unclear, or a quality detail rather than a feature. PO decides.

| # | Item | Spec | Coverage | Note |
|---|---|---|---|---|
| B1 | Discord notifications to team (personal webhook + workspace destination) | 06 In | FB P: Discord alerts for new feedback posts only ([7019078](https://help.featurebase.app/articles/7019078)); IC N; PL N | ec 6 / tasks 5. Slack notifications keep (IC, FB, PL) |
| B2 | Telegram groups (mention/reply chains) and forum topics | 05 In: threading model | IC has Telegram channel ([collection](https://www.intercom.com/help/en/collections/19678202-telegram)); group/topic support U | Private chats suffice to match IC |
| B3 | Discord private `/support` tickets, forum posts, text-channel threads | 05 In | IC Discord channel ([11868746](https://www.intercom.com/help/en/articles/11868746-deploy-fin-ai-agent-over-discord)); sub-modes U | Pick IC's mode set |
| B4 | Teammate echo: teammates replying natively inside Slack/Discord thread recorded as replies | 05 In | PL reply-from-Linear and Slack inbox P; IC U | Needs 03 extension |
| B5 | "Unverified sender" flag from inbound DKIM/SPF/DMARC | 04 In | IC P: impersonation warning + Fin skips DMARC-fail ([13976166](https://fin.ai/help/en/articles/13976166-user-impersonation-warning-explained)) | Arguably security hygiene |
| B6 | Self-host inbound adapters: Cloudflare Email Routing worker, Postmark, Mailgun; outbound drivers postmark/resend/mailgun/sendgrid | 04 In | None (no competitor self-hosts) | Self-host packaging, kept under PO rule; could trim to SES + SMTP + one webhook |
| B7 | Identity validations endpoint, 6-mistake detector, rollout stats, `off/optional/required` modes | 02 In | IC P: enforcement toggle only | Quality detail serving "never fail silent"; cheap |
| B8 | Agent send modes `confirm` / `drafts_only` / `direct` | 09 In | FB P: MCP drafts, confirmation in client ([6144007](https://help.featurebase.app/articles/6144007-featurebase-mcp-server)); PL N; IC MCP cannot send | Safety rail for PO's agents ask |
| B9 | Server-side reply drafts (`reply_drafts`) over REST/MCP/CLI | 09 In | FB P (MCP composer drafts) | Same as B8 |
| B10 | Feedback post ETA (month/quarter, public toggle) | 14 In | FB U, PL U | ec 9 / tasks 4 |
| B11 | On-demand AI summary of requests per feedback post | 14 In: AI | FB P (Feedback 2.0 Insights quotes); PL N | Small BYOK call |
| B12 | Dedicated changelog custom domain (`purpose = changelog`) separate from help center domain | 13 In; 08 domains | FB/PL portal custom domain covers changelog; separate per-purpose domain N | ec 2 / tasks 4 |
| B13 | Public MCP tool `dimmi_list_updates` (changelog) | 13 In | FD public MCP covers help center ([83x1a](https://ferndesk.com/help/let-readers-use-your-help-center-in-ai-tools-83x1a)); changelog tool U | Tiny once 08 public MCP exists |
| B14 | Changelog email replies and widget "Send feedback" land in inbox | 13 In | None documented | Cheap seam; showcases inbox |
| B15 | CSAT over Slack/Discord/Telegram/WhatsApp (plain link, default off) | 15 In | IC/FB CSAT on chat channels U | Default off already |
| B16 | Workspace domain join (verified domain, users see and join) | 00 In | IC/FB/PL U | Accounts flow; tasks 3 |
| B17 | Help center path mode `/hc/{sub}` plus subdomain plus custom domain | 08 In | FD subfolder hosting Y ([713ia](https://ferndesk.com/help/put-your-docs-on-a-sub-folder-with-cloudflare-713ia)) | Keep, FD has subfolder |
| B18 | Per-hunk accept with 3-way rebase onto current article | 08 In: review queue | PL accept/reject per edit Y; rebase is quality | Keep per-edit; rebase quality |
| B19 | Notification delivery inspection API (`/notifications/{id}/deliveries`) | 06 In | None documented | Debuggability, quality |
| B20 | Conversation soft delete + restore (admin) | 03 In | IC delete Y; restore U | Small |

## 3. Explicitly requested by PO (keep unless PO says otherwise)

| PO request | Spec | IC | PL | FB | FD | Any competitor? |
|---|---|---|---|---|---|---|
| Built-in AI agent | 07 | Y Fin | Y | Y Fibi | Y AI answers | Yes, all 4 |
| BYOK AI (6 providers incl. Ollama) | 07 | N (Apex) | N (Opus, credits) | N (model hidden) | N (OpenRouter, hidden) | **None**: Dimmi differentiator |
| Per-conversation token cost, monthly spend cap | 07 | N | N | P spend alerts + cap on resolutions | N | Mostly none (BYOK-specific) |
| Users' own agents via MCP | 09 | Y 14-tool MCP ([guide](https://developers.intercom.com/docs/guides/mcp)) | Y ([docs](https://productlane.com/docs/mcp-server)) | Y 168 tools | Y workspace MCP | Yes, all 4 |
| Public REST API + OpenAPI | 09 | Y | Y v2 | Y | Y | Yes, all 4 |
| CLI | 09 | Y Fin CLI | N | U | N | IC only |
| `npx dimmi init` (repo to widget + identity + seeded docs) | 09 | P `intercom setup --content-dir` | N | N | N | IC partial |
| Install by prompt (Claude Code skill/plugin, Cursor rule, AGENTS.md) | 09 | P (CLI marketed for Claude Code/Cursor) | N | N | P ("use with AI tools") | Partial |
| OAuth device flow for CLI | 09 | N (no browser OAuth) | N | N | N | **None** (supports CLI ask) |
| More chat channels: Slack | 05 | Y | Y Slack Connect | Y | N (read-only source) | Yes |
| Discord | 05 | Y Fin over Discord | N | P feedback bot | N (forum source only) | IC only |
| Telegram | 05 | Y | N | N | N | **IC only** |
| WhatsApp | 05 | Y | P API custom channel | Y | N | IC, FB |
| Changelog | 13 | P News (reactions, no email, no RSS) | Y | Y | Y | Yes |
| Changelog GitHub/PR drafts, AI rewrite | 13 | N | Y merged PRs | P via MCP | Y merged PRs ([47487](https://ferndesk.com/help/changelogs-are-here-47487)) | PL, FD |
| Changelog reactions | 13 | Y ([news_item](https://developers.intercom.com/docs/references/rest-api/api.intercom.io/news/news_item)) | Y | P | Y | Yes |
| Changelog email subscribers | 13 | N | Y | Y ([2705284](https://help.featurebase.app/articles/2705284)) | N | PL, FB |
| Feedback board + voting | 14 | N | Y requests portal | Y | N | PL, FB |
| Public roadmap | 14 | N | Y (Linear mirror) | Y | N | PL, FB |
| AI duplicate detection | 14 | N | N | Y Autopilot ([0367048](https://help.featurebase.app/articles/0367048)) | N | FB |
| AI capture from conversations | 14 | N | P (threads to Linear) | Y | N | FB |
| CSV import | 14 | N | P | Y ([1516262](https://help.featurebase.app/articles/1516262)) | N | FB |
| Portal SSO (JWT) | 14 | N | Y ([sso](https://productlane.com/docs/setup/sso)) | Y ([0874531](https://help.featurebase.app/articles/0874531)) | N | PL, FB |
| Revenue weighting | 14 | N | P | Y ([7739447](https://help.featurebase.app/articles/7739447)) | N | FB |
| Moderation queue, merge posts, internal comments, anonymous voting | 14 | N | N / N / P / U | Y / Y / Y / Y (anon: blog only) | N | FB |
| In-app announcements (banner, modal, chat) | 15 | Y Chats/Posts/Banners ([8771588](https://www.intercom.com/help/en/articles/8771588-chats-posts-and-banners-faqs)) | P changelog popup | Y | P changelog modal | IC, FB |
| Segments | 15 | Y ([324](https://www.intercom.com/help/en/articles/324-how-to-segment-your-contacts)) | N | Y | N | IC, FB |
| Surveys NPS/CSAT | 15 | Y | N | Y ([5099513](https://help.featurebase.app/articles/5099513)) | N | IC, FB |
| CES template | 15 | N | N | Y | N | FB |
| Survey branching | 15 | Y ([6209027](https://www.intercom.com/help/en/articles/6209027-using-survey-branching-logic)) | N | Y | N | IC, FB |
| Survey answer write-back to attributes | 15 | Y ([5970344](https://www.intercom.com/help/en/articles/5970344-8-ways-you-can-use-stored-survey-data)) | N | U | N | IC |
| CSAT after close | 15 | Y | N | Y | N | IC, FB |
| First-party mobile app readiness (API only) | 09, 06 reserved push kinds | Y native agent apps | N | Y native iOS/Android | N | IC, FB |
| Per-conversation pricing | 12 | N (seats + $0.99 outcome) | N (seats + $0.79) | N (seats + $0.49) | P ($149 flat + $0.05/AI conversation packs) | **None** |
| Self-host + open source | 11 | N | N | N | N | **None** |

PO channel note: assistant chose Slack, Discord, Telegram, WhatsApp. No channel is "none". Telegram is IC-only; Discord as a support channel is IC-only (FB only has a feedback bot). If PO wants channels at least two competitors offer: Slack and WhatsApp.

## 4. Full per-spec table

Out of scope for cutting (listed for completeness): 00 auth/2FA/audit/sessions/tokens/brute force, 00-conventions, 00-foundation, 11 packaging/ops, 12 billing mechanics, GDPR export/erase, rate limits, SSRF rules, realtime/Valkey, performance budgets, i18n basics, accessibility.

### 00 Accounts

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Signup password, magic link, GitHub, Google | Y (Google) | U | U | Y magic link | Platform, keep |
| Teams, roles, invitations | Y | Y | Y | Y seats | Keep |
| Workspace domain join | U | U | U | U | Borderline B16 |
| Audit log, 2FA, PATs, sessions | Y | P | Y audit API | P | Platform, keep |

### 01 Messenger widget

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Live chat widget, attachments, typing, read state | Y | Y | Y | N (no chat) | Keep |
| Anonymous visitors, lazy session | Y | Y | Y | Y | Keep |
| Offline reply by email | Y | Y 2 min ([live-chat](https://productlane.com/docs/channels/live-chat)) | Y 3 min ([6584063](https://help.featurebase.app/articles/6584063)) | N | Keep |
| Origin allowlist | Y trusted domains ([4418](https://www.intercom.com/help/en/articles/4418-list-trusted-domains-you-use-with-your-messenger)) | U | P audience rules | U | Keep (hygiene) |
| ALTCHA proof-of-work, bot score | N | N | N | N | **Cut C5** |
| Test sessions / test mode | Y test workspace ([188](https://www.intercom.com/help/en/articles/188-create-a-test-workspace-in-intercom)) | Y teammate testing | U | N | Keep |
| Install snippets per framework, install status | P npm SDK | P | Y `featurebase-js` | Y SDK + React example | Keep |
| Up to 10 widgets per workspace | Y multibrand (Expert) | P multiple portals | Y per brand | P workspaces | Keep |
| Help / What's new / Feedback tabs | Y Help + News | Y docs, changelog, roadmap tabs | Y | Y search + What's new | Keep |
| Realtime uni-SSE, CDN assets, size budget | | | | | Platform |

### 02 Widget identity

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| HS256 JWT identity, `logout()` | Y | Y | Y | Y JWT identify | Keep |
| RS256 / EdDSA public keys | N | N | N | U | **Cut C3** |
| Single-use nonce replay cache | N | N | N | N | **Cut C4** |
| Secret rotation with grace (2 active) | P | U | U | U | Quality, keep |
| Modes off/optional/required + rollout stats | P toggle | U | P secure install default | U | Borderline B7 |
| Validations endpoint + mistake detector | N | N | N | N | Borderline B7 (quality) |
| Helper packages for 9 stacks | N (snippets) | N | N | N | **Cut/trim C16** |
| Company claim, custom attributes | Y | Y `orgExternalId` | Y | U | Keep |
| Portal SSO surface (14) | n/a | Y | Y | Y reader SSO | Keep |

### 03 Shared inbox

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Inboxes, views, filters, sorts, statuses, snooze | Y | Y | Y | N | Keep |
| Assign, auto-assign on reply, teams | Y | Y | Y | N | Keep |
| Bulk by ids / by filter (1,000) | Y up to 10,000 by query ([6274899](https://www.intercom.com/help/en/articles/6274899-get-started-with-intercom-inbox)) | Y | Y | N | Keep |
| Spam view | Y | P blocklist | Y ([8590191](https://help.featurebase.app/articles/8590191-manage-spam-in-the-inbox)) | N | Keep |
| Conversation merge | Y ([8862676](https://www.intercom.com/help/en/articles/8862676-merging-conversations-and-tickets)) | Y ([merging](https://productlane.com/docs/inbox/merging-threads)) | U | N | Keep |
| Participants (CC) | Y | U | U | N | Keep |
| Internal notes + @mentions | Y | Y | Y | N | Keep |
| Message edit within window | N (delete only) | U | U | N | **Cut C2** |
| Message delete | Y | U | U | N | Keep |
| Collision: who is viewing | Y | U | U | N | Keep |
| Collision: teammate typing | N | U | U | N | **Cut C15** |
| Read state, unread counts | Y | Y | Y | N | Keep |
| Contacts, identify, block | Y | Y | Y | N | Keep |
| Contact duplicate suggestion + merge | Y ([16231774](https://www.intercom.com/help/en/articles/16231774-preview-what-happens-before-you-merge-two-contact-profiles)) | P | Y ([8458199](https://help.featurebase.app/articles/8458199)) | N | Keep |
| Companies | Y ([186](https://www.intercom.com/help/en/articles/186-group-your-users-by-company)) | Y | Y | N | Keep |
| Tags | Y | Y | Y | N | Keep |
| Saved replies with variables | Y macros | Y | Y | P Chrome `!!` | Keep |
| Cmd+K search with operators, FTS + trigram | Y search | Y Cmd+K | Y | N | Keep (operators = quality) |
| Sidebar context (attributes, page, browser) | Y | Y | Y | N | Keep |
| Soft delete + restore | P | U | U | N | Borderline B20 |

### 04 Email channel

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Forwarding address | Y | Y | Y | N | Keep |
| IMAP adapter | N | N | N | N | **Cut C1** |
| Self-host inbound adapters (Cloudflare worker, Postmark, Mailgun, SES) | N | N | N | N | Self-host packaging; Borderline B6 |
| Custom sending domain, DKIM/SPF/DMARC records + checker | Y | Y DNS verification | Y | N | Keep |
| Threading, quote stripping, HTML to Markdown | Y | Y | Y | N | Keep (quality) |
| Auto-mail classification (OOO, bounce, list) | U | Y (agent ignores) | Y to Spam | N | Keep |
| Loop prevention | | | | | Quality, keep |
| CC/BCC, reply-all | Y | U | U | N | Keep |
| Forwarded email detection | Y ([7339364](https://www.intercom.com/help/en/articles/7339364-detect-customers-in-forwarded-emails)) | P | U | N | Keep |
| Form-relay (Reply-To) detection | Y (same article) | N | U | N | Keep |
| Unverified sender flag | P | U | U | N | Borderline B5 |
| Per-member signatures | Y | U | U | N | Keep |
| Raw `.eml`, view original, reprocess | U | U | U | N | Quality |
| Bounce/complaint handling, suppression, caps | | | | | Platform |

### 05 Chat app channels

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Slack (channels, Slack Connect) | Y | Y | Y | N | PO keep |
| Discord (bot, forums, threads, DMs, `/support`) | Y | N | P | N | PO keep; sub-modes B3 |
| Telegram (private, groups, forum topics) | Y | N | N | N | PO keep; groups/topics B2 |
| WhatsApp Cloud API, 24 h window, templates | Y | P | Y | N | PO keep |
| Teammate echo from Slack/Discord | U | P | U | N | Borderline B4 |
| Edits and deletes both ways | U | U | U | N | **Cut C2** (edit); keep delete if cheap |
| Socket Mode / long polling for self-host | | | | | Self-host, keep |
| WhatsApp template management | Y | N | U | N | Keep with WhatsApp |
| CLI `channels:add`, MCP tools | | | | | PO agents, keep |

### 06 Notifications

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| In-app bell | Y | Y | Y | N | Keep |
| Email | Y ([187](https://www.intercom.com/help/en/articles/187-how-teammates-get-notifications)) | Y | Y | N | Keep |
| Web push (desktop + PWA) | Y browser | N | Y ([1656979](https://help.featurebase.app/articles/1656979)) | N | Keep |
| Slack (DM + channel destination) | Y ([12556780](https://www.intercom.com/help/en/articles/12556780-set-up-slack-notifications)) | Y | Y | N | Keep |
| Discord (personal + destination) | N | N | P new posts | N | Borderline B1 |
| Quiet hours, pause, burst digest, smart suppression | U | U | U | N | Quality |
| Delivery inspection API | N | N | N | N | Borderline B19 |

### 07 Dimmi Agent

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| AI answers from knowledge with citations | Y | Y | U | Y ([ufxew](https://ferndesk.com/help/how-ai-answers-work-ufxew)) | Keep |
| "I don't know", silence on thanks | Y | Y silent mode | Y fallback | Y | Keep |
| Handoff with summary | P (summary via workflow) | P | P | Y AI-drafted summary ([i02iu](https://ferndesk.com/help/ai-escalation-and-handoff-behavior-i02iu)) | Keep |
| Playground before live | Y preview ([8286630](https://www.intercom.com/help/en/articles/8286630-deploy-fin-ai-agent-over-chat)) | Y | U | N | Keep |
| Schedule vs office hours | Y via workflow | Y ([office-hours](https://productlane.com/docs/administration/office-hours)) | Y ([2109532](https://help.featurebase.app/articles/2109532)) | N | Keep |
| Guidance, tone, languages, max replies | Y | Y | Y | Y | Keep |
| BYOK, 6 drivers, fallback model | N | N | N | N | PO keep |
| Run traces, cost per conversation, spend cap | N | N | P cap | N | PO keep (BYOK) |
| Prompt injection, PII redaction, link allowlist | | | | | Hygiene |
| Streaming to widget | U | U | U | U | Quality |

### 08 Help center

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Collections, Markdown articles, editor | Y | Y | Y | Y | Keep |
| Version history, diff, restore | Y | Y ([version-history](https://productlane.com/docs/help-center/version-history)) | Y ([1314949](https://help.featurebase.app/articles/1314949)) | Y | Keep |
| Agent-only articles | P Fin content | Y | P | Y AI-only visibility | Keep |
| Redirects (auto + manual) | U | U | U | Y | Keep |
| SSR, SEO, sitemap, JSON-LD | Y | Y | Y | Y | Keep |
| Custom domain + auto TLS | Y | Y | Y | Y | Keep |
| Path mode `/hc/` | U | U | U | Y subfolder | Keep (B17) |
| Multiple help centers per workspace | Y up to 100 ([8170953](https://www.intercom.com/help/en/articles/8170953-create-and-manage-multiple-help-centers)) | Y multiple portals | Y | Y $39 each | Keep |
| Search + Ask AI | Y | Y | Y | Y | Keep |
| `llms.txt`, `.md` twins | N | P | Y ([0703677](https://help.featurebase.app/articles/0703677)) | Y | Keep |
| Public read-only MCP for readers | N | N | N | Y | Keep (FD only) |
| Website crawl source | P Fin knowledge | P | Y | Y cold start | Keep |
| Zip Markdown upload | U | Y import | U | Y 22 importers | Keep |
| Git source + push webhook | N | Y | N | Y | Keep |
| Review queue, per-hunk accept, +/- counts | N | Y | P | Y | Keep |
| Cold-start generator via MCP | P CLI `--content-dir` | Y | P importers | Y | Keep |
| Hybrid vector search, embeddings via BYOK | | | | | Platform/BYOK |
| Article thumbs, search log | Y | Y | Y | Y | Keep |

### 09 Agent-native

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| OAuth 2.1 server, PKCE, refresh, CIMD/DCR | P (MCP OAuth) | Y MCP OAuth | Y | Y | Platform for PO ask |
| Device flow | N | N | N | N | PO keep (CLI) |
| MCP server (22 tools, stdio + HTTP) | Y | Y | Y | Y | PO keep |
| Reply drafts + send modes | N | N | P | N | Borderline B8/B9 |
| CLI (`conversations`, `drafts`, `articles pull/push`, `tokens`, `api`) | Y Fin CLI | N | U | N | PO keep |
| `dimmi init` | P | N | N | N | PO keep |
| Install-by-prompt bundle | P | N | N | P | PO keep |
| First-party mobile app OAuth | Y apps | N | Y apps | N | PO keep |
| `dimmi listen` client | N | N | N | N | **Cut C6** |

### 10 Webhooks

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Endpoints CRUD via API, event catalog | P UI only | Y | Y | Y | Keep |
| HMAC signing (Standard Webhooks) | P SHA1 | Y SHA256 | Y | Y | Keep (quality) |
| Retries over 24 h, auto-disable + email | P 1-2 retries | Y + email | U | Y retried | Keep |
| Delivery log | Y | Y 30 days | U | U | Keep |
| Single redeliver | N | Y replay | N | U | Keep |
| Bulk recover since timestamp | N | N | N | N | **Cut C7** |
| Secret rotation with overlap | P | Y | Y ([1754068](https://help.featurebase.app/articles/1754068)) | U | Keep |
| Test event send | U | U | U | U | Quality, keep (cheap) |
| `dimmi listen` server side | N | N | N | N | **Cut C6** |
| `include_notes`, inbox filter, per-endpoint api_version, thin payload | | | | | Quality |

### 11 Self-host, 12 Cloud billing

PO-requested / out of scope. No competitor self-hosts or bills per conversation. 12 "Fin would have charged" comparator and calculator data: marketing, not a competitor feature; keep with pricing.

### 13 Changelog

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Entries, labels, cover, schedule, drafts | P News | Y (no schedule) | Y schedule ([6518299](https://help.featurebase.app/articles/6518299)) | Y tags | PO keep |
| Public page on help center host | P | Y | Y | Y | PO keep |
| RSS | N | Y | Y ([1346872](https://help.featurebase.app/articles/1346872)) | Y | Keep |
| Atom, JSON Feed | N | N | N | N | **Cut C9** |
| Dedicated changelog custom domain | N | P | P | P | Borderline B12 |
| Reactions | Y | Y | P | Y | Keep |
| Widget What's new + unread badge | Y News | Y | Y | Y | Keep |
| Email subscribers, double opt-in, per-entry send | N | Y | Y | N | Keep |
| Weekly digest cadence | N | N | U | N | **Cut C8** |
| 5-minute undo, RFC 8058 unsubscribe | | | | | Compliance/quality |
| GitHub release webhook + PR range drafts | N | Y | P | Y | Keep |
| BYOK AI rewrite | N | Y | Y | Y | Keep |
| Agent cites changelog | N | Y source | Y source | Y | Keep |
| Public MCP `dimmi_list_updates` | N | N | N | U | Borderline B13 |
| Replies land in inbox | N | N | N | N | Borderline B14 |
| Notify voters (14 seam) | N | Y | Y | N | Keep |
| Views/reactions/subscriber analytics | U | U | Y | Y | Keep |

### 14 Feedback and roadmap

All PO-requested. Sub-feature status:

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Boards, statuses, votes, comments | N | Y | Y | N | Keep |
| Public roadmap | N | Y | Y | N | Keep |
| Revenue weighting | N | P | Y | N | Keep |
| Customer requests from inbox (quote + link) | N | Y | Y | N | Keep |
| Merge duplicates | N | N | Y | N | Keep |
| Moderation, spam, ban | N | N | Y ([6982593](https://help.featurebase.app/articles/6982593)) | N | Keep |
| Internal comments + mentions | N | P | Y ([8216651](https://help.featurebase.app/articles/8216651)) | N | Keep |
| ETA field | N | U | U | N | Borderline B10 |
| Portal SSO JWT, magic link | N | Y | Y | N | Keep |
| One-click widget-to-portal hand-off | N | U | P (identified widget votes) | N | Quality |
| Anonymous voting | N | U | Y (blog only) | N | Keep (optional) |
| Close loop: notify on status/ship | N | Y | Y | N | Keep |
| AI duplicate suggestions | N | N | Y | N | Keep |
| AI capture from conversations | N | P | Y | N | Keep |
| AI per-post summary | N | N | P | N | Borderline B11 |
| CSV import | N | P | Y | N | Keep |
| Portal `.md` twins | N | N | N | N | **Cut C10** |

### 15 Announcements and surveys (was 16)

| Feature | IC | PL | FB | FD | Verdict |
|---|---|---|---|---|---|
| Segments | Y | N | Y ([9188570](https://help.featurebase.app/articles/9188570)) | N | Keep |
| Banner | Y | N | Y ([9083800](https://help.featurebase.app/articles/9083800)) | N | Keep |
| Modal (post) | Y | P | Y | P | Keep |
| Chat-bubble announcement | Y Chats | N | Y in-app chats | N | Keep |
| Client-side triggers, `track` event | Y | N | Y | N | Keep |
| Global throttle, frequency | U | N | U | N | Quality |
| Translations up to 10 locales | Y multilingual | N | Y auto-translate | N | Keep |
| NPS, CSAT templates | Y | N | Y | N | Keep |
| CES template | N | N | Y | N | Keep |
| PMF template | N | N | N | N | **Cut C14** |
| Branching | Y | N | Y ([2766909](https://help.featurebase.app/articles/2766909)) | N | Keep |
| Attribute write-back | Y | N | U | N | Keep |
| Hosted survey page / link | P | N | N | N | **Cut C12** |
| Anonymize responses | U | N | U | N | **Cut C13** |
| AI summary of open text | U | N | U | N | **Cut C11** |
| CSV export, per-question results | Y | N | U | N | Keep |
| CSAT on close (widget, email) | Y | N | Y | N | Keep |
| CSAT on chat apps (plain link) | U | N | U | N | Borderline B15 |

## 5. Gaps: the 4 offer it, Dimmi MVP lacks it

Ranked by how many of the 4 offer it and how central it is to their pitch. "Roadmap" = already on Dimmi post-MVP list.

| # | Gap | Who offers | Dimmi status |
|---|---|---|---|
| G1 | Agent actions (HTTP tools, data connectors, Stripe/Shopify) | IC Procedures + connectors, PL connectors, FB custom actions | Launch 18 (agent actions, 07 actions.md) |
| G2 | Importers (Intercom, Zendesk, Crisp, Canny, help centers with redirects) | FB many, PL Intercom/Zendesk/Productboard, FD 22 platforms ([enryy](https://ferndesk.com/help/migrate-your-help-center-to-ferndesk-enryy)), IC Zendesk articles | Launch 17 (`16-importers`) |
| G3 | Self-updating docs from merged PRs and resolved threads | PL, FD (core pitch of both) | Launch 16 (08 docs-sync.md) |
| G4 | Knowledge-gap report / audits / content suggestions | FD audits, PL weekly AI report, FB gap flags, IC content recommendations | post-MVP: knowledge-gap report; data collected now |
| G5 | Reports beyond counts (AI resolution rate, response times, CSAT dashboards) | IC, PL, FB (12 templates), FD AI analytics | post-MVP: reports |
| G6 | Agent assist / Copilot (draft, summarize, translate) | IC, PL, FB, FD (Chrome `!!` drafts) | post-MVP: agent assist |
| G7 | Tickets (customer, back-office, tracker), ticket portal, custom fields | IC, FB, PL support portal | post-MVP: ticket fields |
| G8 | Rules: office-hours auto-reply, round robin, SLAs, workflows | IC, FB, PL | post-MVP: rules lite |
| G9 | Linear integration (two-way, loop closing) | PL (core), FB, FD (source) | post-MVP: Linear |
| G10 | GitHub/Jira issue links, notify on fix | FB, IC Jira, PL (via Linear + GitHub deploy check) | post-MVP: GitHub loop |
| G11 | Multilingual help center, AI translation | IC, PL 47, FB 70+, FD 15 | post-MVP: auto-translation |
| G12 | Private help center / audience-restricted articles | IC, PL, FB, FD | 08 Out (later) |
| G13 | Internal (teammate-only) articles | IC, PL | 08 Out (with agent assist) |
| G14 | Claim-level article verification / stale detection | FD ([0mv7r](https://ferndesk.com/help/verify-your-help-center-articles-0mv7r)) | Standout #5 |
| G15 | Automated screenshots by logging into app | FD beta, PL via coding agent | Standout #12 |
| G16 | OpenAPI/GraphQL API reference pages in help center | FB, PL, FD | 08 Out (later) |
| G17 | Native mobile SDKs for end users | IC (iOS, Android, RN/Expo) | post-MVP: mobile SDKs |
| G18 | Native agent mobile apps for teammates | IC, FB | PO: API readiness only |
| G19 | Product tours, checklists, tooltips | IC, FB | 15 Out (upmarket) |
| G20 | Email campaigns / broadcasts | IC, FB | 15 Out |
| G21 | Microsoft Teams channel | PL, FB | post-MVP: more channels |
| G22 | SMS, Instagram, Facebook Messenger, voice | IC | post-MVP: more channels, upmarket set |
| G23 | Developer context: console + browser capture | PL automatic | Standout #2 (01 Out) |
| G24 | Simulations / evals before publish | IC Simulations | post-MVP: agent evals |
| G25 | QA scoring, CX score | IC | post-MVP: upmarket set |
| G26 | SSO SAML/SCIM | IC, PL, FB, FD Enterprise | post-MVP: upmarket set (never paywalled) |
| G27 | Multi-brand (separate messenger + help center per brand) | IC Expert, FB Enterprise | 01 Out (10 widgets, no brands) |
| G28 | Undo send | PL | 03 Out |
| G29 | Reply from Slack / Linear (team works outside dashboard) | PL Slack inbox + reply within Linear | post-MVP: chat replies |
| G30 | Knowledge sources beyond web/git/zip: Slack, Discord forums, tickets from other helpdesks, video, Notion/Confluence | FD many, IC Notion/Confluence/Guru | 08 Out partly |
| G31 | Chrome extension (docs sidebar, reply drafts) | FD, PL | None |
| G32 | AI image/attachment understanding | IC, PL (email attachments) | 07 Out |
| G33 | Feedback from call recorders (Gong, Fathom) and external tools | FB | None |
| G34 | Embeddable "Ask AI" spotlight bar for marketing pages | PL Spotlight | None |

Highest-signal gaps for launch (offered by 3+ of 4, central to their pitch): G1 agent actions, G2 importers, G3 self-updating docs, G4 gap report, G5 reports, G6 copilot, G11 translations, G12 private help center.

## Ferndesk summary (new research, `competitors/ferndesk.md`)

- AI help center, not a helpdesk. Agent "Fern" drafts articles from merged PRs, Linear, Slack, tickets from 13+ helpdesks, video. Never auto-publishes.
- Strong: PR drafts with sensitivity setting, claim-level verification, scheduled audits, screenshot recapture, 22-platform importer, workspace MCP + public reader MCP, `llms.txt`, subfolder hosting, changelog from PRs.
- Widget: search + AI answers + What's new + contact form; hands off to Intercom/Crisp/Help Scout/Zendesk/Pylon/Plain. No inbox, no live chat, no channels.
- No BYOK, no agent actions, no feedback board, roadmap or surveys, no CLI, not open source, no self-host.
- Pricing: Pro $149/month (5 seats, 1,000 AI conversations); early-stage $75; Enterprise from $399.
- Effect on this audit: FD alone justifies keeping 08 public MCP, cold-start, review queue, subfolder hosting, changelog PR drafts. FD also widens gaps G3, G4, G14, G15, G30.
