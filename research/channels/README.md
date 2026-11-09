# Chat app channels: competitor study and direction

Research date: 2026-11-08. Style: caveman. Subject: best support experience on Slack, Discord, Telegram. Checked against unbuilt plan `specs/05-chat-app-channels`.

| File | What |
|---|---|
| [slack.md](slack.md) | Pylon, Plain, Thena, Unthread, ClearFeed, Atlas, Fin, Front, Featurebase, Productlane, Zendesk. Slack platform 2026. 22 gaps |
| [discord.md](discord.md) | Fin, Plain, Pylon, Thena, Unthread, Mava, kapa, Inkeep, RunLLM, Answer Overflow, ticket bots. How dev communities run help. 17 gaps |
| [telegram.md](telegram.md) | Fin, Chatwoot, Crisp, respond.io, Zendesk, Mava, Pylon, Interchat, crypto long tail. Bot API 9.3 to 10.3. 15 gaps |

## TL;DR

- Spec 05 treats chat apps like email: one private thread per customer, bot DM, teammates reply only from inbox. Competitors treat chat apps as **places where humans and agent share one public space**. That is the big shift.
- Discord: spec MVP (bot DMs only) is least-used path. Every competitor starts from forum posts, mentions, threads. DM-only launch loses to free kapa-style bots and Mava free tier.
- Slack: table stakes now = teammate replies in Slack sync back, agent stands down when human speaks, ingestion modes with pleasantry filter, emoji actions, buttons. Spec defers or misses all.
- Telegram: helpdesks do bot DM only, community tools do groups only, nobody uses 2026 Bot API (streaming drafts, guest mode, business connections, managed bots). Cheap lead for dimmi inside scope already chosen.
- No one offers free, self-hosted, BYOK support across all three. Self-host Slack internal app also skips Marketplace history throttle. Positioning holds; experience must catch up.

## Cross-channel principles (apply to every driver)

1. **Humans in the place count.** Teammate (and trusted helper role) replies typed natively in Slack thread, Discord post, Telegram team topic ingest as replies. Spec "later: teammate-echo" moves to launch for shared spaces. Without it inbox lies, SLAs misfire, agent repeats humans.
2. **Agent knows when to shut up.** Per message decision: answer / stay silent / hand off, reason logged. Silent on pleasantries, member-to-member chat, messages aimed at teammate. Stops for good once a human replies or is @mentioned. Hand-off note says what it already searched.
3. **"Thanks" is never a conversation.** Pleasantry filter before conversation creation. Never billable. Marketing line against Fin "assumed resolution" billing.
4. **Every AI answer has an exit.** Buttons: Solved / Talk to a human, plus thumbs. Explicit solved = honest resolution signal for analytics.
5. **Visible work.** Reaction or typing while agent thinks (👀 / 💬), streaming where platform supports (Telegram `sendMessageDraft` now; Slack `chat.startStream` verify thread support).
6. **State visible in place.** Slack ✅ and reopen on reply; Discord forum tags Open / Answered / Solved / Needs team synced both ways, archive on close; Telegram reaction plus inline status.
7. **Audience-aware agent.** In public or multi-org spaces agent uses public knowledge only, no account-data tools. Linked company room (Slack Connect) or verified identity unlocks private context. Featurebase does this best; Intercom only warns. Privacy bug class, cheap to guard now.
8. **Public by default, private on request.** Discord private thread, Telegram deep link to bot DM, Slack ephemeral. Detect pasted secrets / PII in public posts, offer move to private, never echo back.
9. **Identity link.** Signed deep link from widget ("Continue in Telegram", Discord "Link account"), self-serve merge and unlink. Weakest point at every competitor.
10. **Solved threads feed knowledge.** Teammate or trusted answer in solved thread becomes knowledge source plus help-center draft for review. Fits self-updating help center story; nobody closes this loop end to end.
11. **Reply without opening dimmi.** Slack triage channel, Telegram admin supergroup with topic per conversation, Discord native reply. Solo devs live in chat apps.

## Recommended spec 05 scope changes

Launch (move in or add):

| Channel | Change |
|---|---|
| All | Teammate echo for shared spaces (03 X2 pulled forward); agent silence/hand-off policy (07 hook); pleasantry filter; Solved / Talk to human buttons; working indicator; audience guard (public knowledge only outside linked rooms) |
| Slack | Manifest with interactivity on, `reactions:read`, `app_mentions:read`, `commands` now (avoid self-host re-approval later). Ingestion modes: per-message, emoji-only, mention-only plus existing author window. "Create conversation from message" shortcut |
| Discord | Forum channel mode (post = conversation, `discord:t:{thread}`); mention / right-click "Ask dimmi" / `/ask` in text channels answered in thread (no privileged intent); forum tag status sync; role mapping (teammate / helper / escalation ping); per-room trigger auto / mention / off; App Verification and MESSAGE_CONTENT review started for cloud bot. Keep DMs |
| Telegram | `sendMessageDraft` streaming with Stop cancelling model call; inline buttons (quick replies, human, solved, CSAT); widget deep-link identity handoff (`?start=` signed nonce); 👀 reaction on receipt; QR and widget button |

Soon (first weeks after launch): Slack emoji actions, edit/delete sync, reply as own Slack user, triage channel, auto-join `#ext-*`, office hours line; Discord private tickets (panel, `/support`, private thread, intake modal), secret guard, knowledge capture, CSAT buttons, webhook teammate avatars, account link; Telegram groups (mention / reply / guest mode, 20 msg/min bucket), Business connection, admin-supergroup replies, Managed Bots one-tap create, edits as note. Chat-app CSAT for all.

Later: Slack broadcasts, forms, Work Objects (internal only), streaming; Discord SEO pages, GitHub issue from thread, forum history import as knowledge; Telegram private-chat topics, Rich Messages, local Bot API server.

Data model touch: `channel_rooms.kind` adds `discord_forum`, `discord_text`, `discord_ticket_panel` (later `telegram_group`); per-room `mode`, trigger, tag map, linked `company_id` in settings or columns.

## Status

Folded into `specs/05-chat-app-channels` on 2026-11-08 (plus 03 X2/X6, 07 stand-down and streaming, 12 echo waiver). Open questions below answered there (README "Open questions" 5-9).

## Open questions for owner

- Billing: pleasantry-only and AI-silent threads never billable? (recommended yes)
- Discord at launch: forum + mention modes in, DM kept. Accept bigger 05 scope or cut WhatsApp to soon?
- Telegram groups at launch only if a web3 design partner exists?
- Slack reply-as-user (user tokens) worth extra OAuth complexity at launch, or bot with custom name/avatar first?
