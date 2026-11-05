# Intercom gaps in what dimmi has built

What Intercom does, inside the features dimmi already has, that dimmi does not. Summarised in our own words. Nothing here has been changed in code.

## 1. What this was audited against

**Research date:** 2026-11-04. **Our code:** commit `b8eaecd` on `main`. Another agent was fixing bugs in the same tree during this audit, so a line may have moved or a gap may have closed since.

**Built features compared:**

- **Inbox:** conversations with open, snoozed and closed; assign to member or team (API); priority flag; spam flag (API); reply, note, relay; read state; counts; list search with phrases, `-word`, `OR` and `is:` filters; quick search API; neighbors API; views mine, unassigned, all, member, spam (API) and mentions (always empty); composer with R, N, O and Cmd+Enter; details panel; presence in the conversation header; realtime with recovery.
- **Inboxes and contacts:** inbox CRUD API with reopen window and assign on reply; contact CRUD API with block flag and custom attributes.
- **Email:** forwarding addresses, setup steps, forwarding test, Gmail confirmation code, threading, classification, DMARC check on inbound, quoted reply, bounce and complaint suppression, sender throttle.
- **Security and accounts:** password, magic link, Google and GitHub sign-in, TOTP, passkeys, recovery codes, workspace 2FA rule, three roles, invitations, teams, API tokens with abilities and expiry, audit log, personal security log, session list, workspace trash and restore, ownership handover.
- **Settings pages wired to real data:** email, workspace, members and invitations, teams, tokens, audit log, security (2FA, sessions, activity).
- **Settings pages that are still sample content:** inboxes, tags, saved replies, preferences, profile fields, notifications, sign-in methods, password. These are reported in section 3 where a built feature sits behind them.

**Rules of this report:**

- The 96 bugs in `research/competitors/edge-case-audit-chatwoot-zammad.md` are left out. Where a gap sits next to one, the entry names it (`F16`).
- Macros and saved replies are left out, as asked.
- "Spec" notes where `specs/03-shared-inbox/README.md` already places the item. Several items the spec calls MVP are unbuilt, and several it defers are recommended sooner here.
- Effort: small is days, medium is one to two weeks, large is more. These are estimates from reading the code.

## 2. Top recommendations

Ranked for a world-class MVP. "Why" is one line each.

1. **E-01 Send from the customer's own address, with domain authentication.** Today every reply reaches customers from our inbound forwarding address, which reads as not professional and hurts deliverability.
2. **I-01 Team, inbox and teammate views, with team assignment on screen.** The API has all of it, but the screen shows only four fixed views, so a team cannot organise its work.
3. **I-110 and I-111 Keyboard navigation and command menu.** Support is a keyboard job; we have R, N and O, and a neighbors API nothing uses.
4. **I-50 @mentions in notes with notifications.** The Mentions view exists and is always empty, which is a visible hole in collaboration.
5. **I-70 Bulk actions.** Without them, cleaning up after a spike or a spam wave means opening conversations one at a time.
6. **I-20 and I-30 Snooze picker, send and close, send and snooze.** The server supports all of it; the screen offers only "tomorrow" and plain send, so this is the cheapest high-value work here.
7. **I-60 Conversation tags.** The tables and queries exist but nothing can tag, and tags feed views, search and later reports.
8. **I-90 and I-91 Customer panel and past conversations.** A teammate today sees only a name, an address and a kind, with no history of the person.
9. **I-71 Merge conversations.** Email creates duplicates daily, and there is no way to join them.
10. **I-80 Manage participants, Cc and Bcc.** Email work needs it, and `F16` only fixes copying people already on the thread.
11. **S-01 Run the malware scan on attachments.** Scanner drivers exist and nothing calls them, and files are about to become downloadable through the `F14` fix.
12. **P-01 Rate limits on the public API.** The middleware exists but is not on any route, which leaves the API open to abuse and accidents.
13. **I-140 and I-141 Spam folder, not spam and block from the inbox.** The server can do these but teammates have no control for any of them.
14. **S-15 Erase and export one customer's data.** Needed to answer privacy requests; permission names exist and no action does.
15. **I-10 and I-11 Round robin and away status.** Teams past a few people need fair distribution; the availability column exists and nothing writes it.

Just outside the list, and all marked must have or close to it: I-03 sorting by waiting since, I-31 drafts that survive, I-36 delete a reply or note, I-40 start a conversation from the inbox, S-04 conversation access by team.

## 3. Gaps by area

### Inbox: views, list and sorting

**I-01. Team, inbox and teammate views, and team assignment on screen.**
- Intercom: each team has its own inbox in the sidebar, teammates are a folder, conversations can be assigned to a team. ([team inboxes](https://www.intercom.com/help/en/articles/197-organize-team-inboxes), [customise sidebar](https://www.intercom.com/help/en/articles/7911926-customize-the-inbox-to-suit-you-and-how-you-work-best))
- We do: API accepts `filter[team]`, `filter[inbox]`, `filter[view]=member` and `team_id` (`app/http/controllers/api/v1/conversation_list_query.go`, `app/actions/conversations/filters.go`). The sidebar has mine, unassigned, all, mentions (`resources/js/features/shell/navigation.ts`), and the details panel picks only a member (`resources/js/features/inbox/details-panel.tsx`).
- Plan: team inboxes are Advanced and Expert ([setup](https://www.intercom.com/help/en/articles/10223008-setting-up-the-inbox)).
- Work: medium. **Must have:** the server work is done and every team past two people needs it.

**I-02. Status filter on the list.**
- Intercom: a filter above the list switches between open, closed and snoozed. ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained), [snooze](https://www.intercom.com/help/en/articles/6564538-snooze-a-conversation))
- We do: API has `filter[status]`; the list always shows open unless a search is typed (`resources/js/lib/api/inbox.ts`, `resources/js/features/inbox/inbox-page.tsx`). `F31` proposes a snoozed view only.
- Plan: none stated.
- Work: small. **Must have:** closed conversations can be reached today only by searching.

**I-03. Sort control with waiting since, priority and snooze time.**
- Intercom: sort by date started, last activity, waiting since, priority, snooze time, SLA and custom attributes, either direction. Waiting since measures from the customer's first unanswered message. ([sorting](https://www.intercom.com/help/en/articles/6989006-inbox-sorting), [waiting](https://www.intercom.com/help/en/articles/9636510-prioritize-conversations-waiting-for-a-teammate-reply))
- We do: API sorts by `last_message_at` and `created_at` only (`conversation_list_query.go`). No control on screen. No waiting-since column. Spec lists waiting longest and snooze ends.
- Plan: none stated; SLA sorts need SLAs.
- Work: medium. **Must have:** oldest unanswered first is how a support queue is worked.

**I-04. Shared custom views.**
- Intercom: a saved set of filters with And/Or groups, live results, visible to the workspace, pin, duplicate, search inside a view. Needs a permission. ([views](https://www.intercom.com/help/en/articles/6588834-organize-your-inbox-with-custom-views-and-folders), [FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs))
- We do: nothing (`app/actions/conversations/filters.go` has fixed views). Spec defers it.
- Plan: not stated in the article.
- Work: large. **Should have:** valuable once tags and filters exist, so it comes after them.

**I-05. Personal folders and sidebar editing.**
- Intercom: personal folders grouping teams, teammates and views; pin and reorder. ([customise](https://www.intercom.com/help/en/articles/7911926-customize-the-inbox-to-suit-you-and-how-you-work-best))
- We do: fixed sidebar (`navigation.ts`).
- Plan: none stated.
- Work: small to medium. **Later:** few views exist to organise yet.

**I-06. Table layout with chosen columns and a preview.**
- Intercom: toggle with L; each row is a conversation, columns can be added, moved, sorted, hidden; selecting a row previews it. ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained))
- We do: one list style (`conversation-list.tsx`).
- Plan: none stated.
- Work: medium to large. **Later:** matters for managers at volume.

**I-07. More on the list row: channel, presence, assignee, priority.**
- Intercom: avatars on a row show who has it open, up to three then a count; rows carry channel indicators. ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained), [product page](https://www.intercom.com/helpdesk/inbox))
- We do: the row shows avatar, name, time, subject or excerpt, unread dot (`conversation-list.tsx`). Presence shows only inside the open conversation (`resources/js/features/realtime/collision.tsx`).
- Plan: presence follows a workspace setting.
- Work: small. **Should have:** presence on the row stops collisions before anyone opens the conversation.

### Inbox: assignment and availability

**I-10. Round robin as an inbox setting.**
- Intercom: per team inbox choose manual, round robin or balanced. Round robin rotates among active teammates once on arrival and skips those away unless a per-team toggle says otherwise. ([round robin](https://www.intercom.com/help/en/articles/6512504-round-robin-deep-dive), [team inboxes](https://www.intercom.com/help/en/articles/197-organize-team-inboxes))
- We do: only assign on reply (`specs/03-shared-inbox/README.md`, `app/actions/messages/reply.go`). Spec defers rules. No workflow engine is needed for this one.
- Plan: Advanced and Expert ([status](https://www.intercom.com/help/en/articles/6522874-manage-teammate-inbox-status)).
- Work: medium. **Should have:** the simple fair split, without waiting for workflows.

**I-11. Away status and reassign on reply.**
- Intercom: active, away, or away and reassigning; a customer reply then moves the conversation to the default assignee or back to the team inbox. Managers can change status with a permission. API too. ([status](https://www.intercom.com/help/en/articles/6522874-manage-teammate-inbox-status))
- We do: `workspace_members.availability` is read by `me_resource.go` and `member_resource.go`, and nothing writes it. The toggle in `resources/js/features/settings/pages/account.tsx` is static.
- Plan: status itself not gated; dashboard control is Expert.
- Work: medium. **Should have:** holidays and sick days otherwise leave customers waiting on one person. Pairs with I-10.

**I-12. Away reasons and automatic away.**
- Intercom: up to 100 reasons, optionally mandatory; automatic away after set thresholds. ([status](https://www.intercom.com/help/en/articles/6522874-manage-teammate-inbox-status), [auto away](https://www.intercom.com/help/en/articles/6899988-automatic-away-mode))
- We do: nothing.
- Plan: automatic away is Expert.
- Work: small to medium. **Later:** useful after I-11, and it needs reports to matter.

**I-13. Balanced assignment and capacity limits.**
- Intercom: gives work to the eligible teammate with fewest open conversations, with teammate, channel and inbox limits, primary and secondary inboxes, and a chosen queue order. Explains why a conversation waits. ([balanced](https://www.intercom.com/help/en/articles/6553774-balanced-assignment-deep-dive))
- We do: nothing.
- Plan: Expert.
- Work: large. **Later:** Intercom gates it to its top plan, so it is not expected at MVP.

**I-14. Per-teammate choice about self-assign.**
- Intercom: each teammate chooses whether replying assigns to them; with it on, reopening or unsnoozing an unassigned conversation also assigns. ([get started](https://www.intercom.com/help/en/articles/6274899-get-started-with-intercom-help-desk))
- We do: a per-inbox `assign_on_reply`, with no screen (`resources/js/features/settings/pages/inbox.tsx` is static).
- Plan: none stated.
- Work: small. **Later:** the inbox-level rule covers most teams once T-01 ships.

### Inbox: snooze, close and reopen

**I-20. Snooze picker with presets and custom time.**
- Intercom: Z opens a menu of options plus a custom date and time. ([snooze](https://www.intercom.com/help/en/articles/6564538-snooze-a-conversation))
- We do: the API takes four presets and a time (`app/actions/conversations/status.go`). The screen always sends `tomorrow` (`resources/js/features/inbox/case.ts`).
- Plan: none stated.
- Work: small. **Must have:** a single fixed snooze time makes the feature nearly useless.

**I-21. Admin-managed snooze lists per team.**
- Intercom: up to 10 options per list, each team one list. ([snooze](https://www.intercom.com/help/en/articles/6564538-snooze-a-conversation))
- We do: fixed presets (`status.go`).
- Plan: none stated.
- Work: small to medium. **Later:** presets suit most teams.

**I-22. More things wake a snooze, with admin switches.**
- Intercom: timer, customer or teammate reply, and an attribute change by someone other than the assignee. Optional switches for another person's note or assignment. ([snooze](https://www.intercom.com/help/en/articles/6564538-snooze-a-conversation))
- We do: a customer message wakes it (`app/actions/conversations/receive_inbound_message.go`); a note changes nothing (`app/actions/messages/notes.go`); a reply leaves status as is unless `then` says otherwise (`reply.go`). `F31` covers the timer.
- Plan: none stated.
- Work: small. **Later:** the customer reply and timer matter most.

### Inbox: composer and thread

**I-30. Send and close, send and snooze.**
- Intercom: Cmd+Shift+Enter and Cmd+Option+Enter; the first snooze option is used. ([shortcuts](https://www.intercom.com/help/en/articles/6272267-how-to-use-command-k-with-intercom-inbox), [snooze](https://www.intercom.com/help/en/articles/6564538-snooze-a-conversation))
- We do: API has `then` of keep, open, close, snooze (`app/actions/messages/reply.go`). The composer sends plain replies (`resources/js/features/inbox/composer.tsx`, `conversation-view.tsx`).
- Plan: none stated.
- Work: small. **Must have:** the most common move in support, and the server is ready.

**I-31. Drafts that survive leaving the conversation.**
- Intercom: lists composer drafts as shipped, and keeps reply and note drafts apart. ([product page](https://www.intercom.com/helpdesk/inbox), [changelog](https://www.intercom.com/changes/en/138906-never-lose-a-draft-when-switching-between-replies-and-notes))
- We do: drafts are React state, separate per mode, thrown away when the conversation route changes (`composer.tsx`, keyed by id in `resources/js/routes/d/inbox/$view/$conversation.tsx`). Spec says the client stores drafts locally, which is not built.
- Plan: none stated. Intercom's storage and scope are not documented.
- Work: small. **Must have:** clicking away must not lose a long answer.

**I-32. Undo send.**
- Intercom: a delay window of 5 to 30 seconds, behind a permission. Once sent, email cannot be recalled. ([conversations FAQ](https://www.intercom.com/help/en/articles/8838326-conversations-faqs), [permissions](https://www.intercom.com/help/en/articles/176-teammate-permissions-how-to-control-workspace-access))
- We do: nothing; `SendChannelMessage` is queued at commit. Spec defers it.
- Plan: not stated.
- Work: medium. **Should have:** cheap trust for email, where a wrong send cannot be taken back.

**I-33. Formatting toolbar and shortcuts.**
- Intercom: rich composer with Markdown shortcuts as typed, Cmd+B and Cmd+I, a toolbar; notes and email replies add underline, size, colour, alignment. ([composing](https://www.intercom.com/help/en/articles/319-composing-a-message))
- We do: a plain textarea, Markdown typed by hand (`composer.tsx`).
- Plan: none stated. The source describes outbound composing and mentions inbox replies; precise inbox coverage is not confirmed.
- Work: medium. **Should have:** teammates who do not know Markdown cannot format.

**I-34. Emoji and GIF insert.**
- Intercom: `:` for emoji, Cmd+Shift+G for GIF. ([shortcuts](https://www.intercom.com/help/en/articles/6272267-how-to-use-command-k-with-intercom-inbox))
- We do: nothing in `composer.tsx` or `add-menu.tsx`.
- Plan: none stated.
- Work: small. **Later:** nice, not blocking.

**I-35. Inline images in replies.**
- Intercom: Cmd+Shift+I finds an image to upload into the message. ([shortcuts](https://www.intercom.com/help/en/articles/6272267-how-to-use-command-k-with-intercom-inbox))
- We do: nothing; `F15` covers files, not inline images in sent mail.
- Plan: none stated.
- Work: medium. **Should have:** screenshots are how support explains things. Do it with the `F15` fix.

**I-36. Delete a reply or note, and edit a note.**
- Intercom: both behind permissions; authors edit their own notes; deleted messages cannot be recovered. ([conversations FAQ](https://www.intercom.com/help/en/articles/8838326-conversations-faqs), [mentions](https://www.intercom.com/help/en/articles/6525765-loop-teammates-or-teams-into-conversations))
- We do: no route (`routes/inbox.go`), though events `message.updated` and `message.deleted` are named. Spec has it in MVP.
- Plan: none stated.
- Work: medium. **Must have:** the only way to remove a pasted password or a card number.

**I-37. Emoji reactions on notes.**
- Intercom: internal reactions; the author gets a mention. ([mentions](https://www.intercom.com/help/en/articles/6525765-loop-teammates-or-teams-into-conversations))
- We do: nothing.
- Plan: none stated.
- Work: small. **Later:** depends on I-50.

**I-38. Translation of conversations.**
- Intercom: customer messages show in the teammate's language, replies go back in the customer's. Ten free translated conversations a month without a Copilot seat. ([translations](https://www.intercom.com/help/en/articles/10545610-how-to-use-ai-inbox-translations))
- We do: nothing. Spec places it with agent assist.
- Plan: Copilot seat for unlimited use.
- Work: medium, needs a real LLM. **Later:** comes with the agent work.

**I-39. Thread viewing extras.**
- Intercom: image lightbox with zoom; code blocks with highlighting, copy and full screen; customer time zone toggle; hide events with Cmd+E; date on hover. ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained), [FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs), [shortcuts](https://www.intercom.com/help/en/articles/6272267-how-to-use-command-k-with-intercom-inbox))
- We do: none in `resources/js/features/inbox/thread.tsx` or `message-body.tsx`.
- Plan: none stated.
- Work: small each. **Later:** code block copy is the first one worth doing for a developer audience.

**I-40. Start a conversation from the inbox.**
- Intercom: C or Create new; To, From, Cc, Bcc; optional send separately per recipient. ([start a conversation](https://www.intercom.com/help/en/articles/6433002-start-a-conversation-from-the-inbox))
- We do: `POST /conversations` with `author=member` exists (`app/actions/conversations/create_conversation.go`), and no screen calls it.
- Plan: custom sender needs a permission.
- Work: medium. **Must have:** following up with a customer by email is basic. This is one person at a time, not outbound campaigns.

### Inbox: mentions and collaboration

**I-50. @mentions in notes, with notifications.**
- Intercom: `@` in a note suggests teammates and teams; team mentions create one per member; email and mobile push; Mentions inbox filtered by unread; mark read in bulk; workspace switch for team mentions. ([mentions](https://www.intercom.com/help/en/articles/6525765-loop-teammates-or-teams-into-conversations))
- We do: `listParams` returns an empty list for `ViewMentions` (`app/actions/conversations/filters.go`). No parser or storage (`app/actions/messages/notes.go`). Notifications are unbuilt (`resources/js/routes/d/notifications.tsx` is a placeholder, spec 06).
- Plan: none stated.
- Work: medium for the mention, large with email and push. **Must have:** ship mention and in-app first, then mail.

**I-51. Side conversations by email.**
- Intercom: email an outside partner from a conversation, replies post to the parent and customers never see them; assignee and state follow the parent; a reply wakes it. Slack too. ([side conversations](https://www.intercom.com/help/en/articles/8398956-side-conversations))
- We do: nothing.
- Plan: full seat and a permission; plan tier not stated.
- Work: large. **Later:** forwarding by hand covers early teams.

**I-52. Similar conversations.**
- Intercom: past conversations with similar text, last 6 months. ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained))
- We do: nothing; the "past conversations" in the add menu are sample data (`resources/js/features/inbox/library.ts`).
- Plan: none stated.
- Work: medium. **Later:** search covers it for now.

### Inbox: tags, priority and attributes

**I-60. Conversation tags.**
- Intercom: tag conversations, tag a message with Shift+T, manage tags, filter and build views. ([shortcuts](https://www.intercom.com/help/en/articles/6272267-how-to-use-command-k-with-intercom-inbox), [search](https://www.intercom.com/help/en/articles/6516006-inbox-search-and-filter))
- We do: tables and queries exist (`database/queries/tags.sql`). No action, route or filter; the settings page is sample data (`settings/pages/inbox.tsx`). Spec has it in MVP.
- Plan: none stated.
- Work: medium. **Must have:** the first thing teams use to organise, and I-04 and reports rely on it.

**I-61. Tags on people and companies.**
- Intercom: person tags follow a person across conversations. ([search result summary](https://www.intercom.com/help/en/articles/6516006-inbox-search-and-filter))
- We do: nothing.
- Plan: none stated.
- Work: small after I-60. **Later:** conversation tags first. The exact source wording for this was only seen in search results.

**I-62. Priority levels.**
- Intercom: five levels, none to urgent, for all workspaces, replacing the old star; P sets it. ([priority](https://www.intercom.com/help/en/articles/6545655-prioritize-your-most-important-conversations))
- We do: a boolean (`app/actions/conversations/attributes.go`, `case.ts`).
- Plan: all workspaces.
- Work: small to medium, a migration. **Should have:** cheaper now than after integrations rely on a boolean.

**I-63. Custom conversation attributes, and required to close.**
- Intercom: text, list, number, decimal, boolean, date, reference and file; can be required before closing in the inbox; conditional; team-limited; filter and sort by them. ([attributes](https://www.intercom.com/help/en/articles/6546210-using-conversation-data-in-the-inbox))
- We do: nothing on conversations. Spec defers custom fields.
- Plan: not stated.
- Work: large. **Later:** mainly feeds reports and workflows, which do not exist yet.

### Inbox: bulk, merge and duplicates

**I-70. Bulk actions.**
- Intercom: X or checkboxes, then reply, note, assign, tag, snooze, close or set priority. Up to 1,000 at once, and "select all matching" up to 10,000 with a background job past 1,000. ([get started](https://www.intercom.com/help/en/articles/6274899-get-started-with-intercom-help-desk), [FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs), [search](https://www.intercom.com/help/en/articles/6516006-inbox-search-and-filter))
- We do: no bulk route (`routes/inbox.go`). Spec has 100 ids or a filter up to 1,000 in MVP.
- Plan: none stated.
- Work: medium to large. **Must have:** needed for any spam wave or team handover.

**I-71. Merge conversations.**
- Intercom: the secondary closes and turns read-only with a link and summary note; the primary carries on; irreversible except through support. Conversations with several participants cannot merge. Cross-user merging is off unless enabled in security settings. ([merge](https://www.intercom.com/help/en/articles/8862676-merging-conversations-and-tickets))
- We do: nothing but an event name `conversation.merged` (`app/events`). Spec has it in MVP.
- Plan: not stated.
- Work: medium to large. **Must have:** email produces duplicates all the time. Copy the off-by-default rule for different customers.

**I-72. Merge contacts and flag potential duplicates.**
- Intercom: the panel flags leads or users matching on phone, WhatsApp or email and merges after a preview. ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained))
- We do: the directory hides merged rows (`app/actions/contacts/list_contacts.go`) but no merge action exists. `F36` covers identity bugs only. Spec has it in MVP.
- Plan: none stated.
- Work: medium. **Should have:** one customer on two addresses splits their history.

### Inbox: participants and customer context

**I-80. Manage participants, with To, Cc and Bcc in the composer.**
- Intercom: add or remove from the header or the composer; up to 10 participants, 50 Bcc on a reply; changes apply on send; flags outsiders replying; removed people stop receiving replies. Behind a permission. ([group conversations](https://www.intercom.com/help/en/articles/6435588-manage-group-conversations-in-the-inbox))
- We do: inbound stores participants (`database/queries/conversation_participants.sql`); no route or control. `F16` only fixes copying those already stored. Spec has it in MVP.
- Plan: not stated.
- Work: medium. **Must have:** email support needs to bring in a customer's manager.

**I-90. Customer details panel with real attributes.**
- Intercom: plan, browser, company, location, last active, custom attributes and pinned attributes in the column beside the thread; click for the full profile. ([profiles](https://www.intercom.com/help/en/articles/6988783-get-context-fast-with-user-and-company-profiles))
- We do: name, address, kind, inbox, channel (`details-panel.tsx`). The API already holds attributes and `conversations.source` per the spec. Links and articles sections are placeholder text.
- Plan: none stated.
- Work: medium. **Must have:** answers depend on knowing who is asking.

**I-91. Past conversations of this person.**
- Intercom: three most recent, expandable to 20. ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained))
- We do: API has `filter[contact]`; nothing on screen.
- Plan: none stated.
- Work: small. **Must have:** a day's work that stops customers repeating themselves.

**I-92. Contact directory.**
- Intercom: people and company lists, filter, export. ([people list](https://www.intercom.com/help/en/articles/318-the-people-list-and-company-list-explained), [permissions](https://www.intercom.com/help/en/articles/176-teammate-permissions-how-to-control-workspace-access))
- We do: API lists and edits contacts (`app/actions/contacts`); `resources/js/routes/d/customers.tsx` is a placeholder.
- Plan: Lite seats excluded.
- Work: medium. **Should have:** there is no way to see or edit a customer otherwise.

**I-93. Company profile.**
- Intercom: users at the company, plan, past conversations, notes, tags; a company filter in search. ([profiles](https://www.intercom.com/help/en/articles/6988783-get-context-fast-with-user-and-company-profiles))
- We do: a `companies` table with no action or route. Spec has it in MVP.
- Plan: none stated.
- Work: medium to large. **Should have:** B2B teams think in accounts, though it can follow I-90.

### Inbox: search

**I-100. Filters and operators in search.**
- Intercom: assigned to, team, tag, topic, user, company, status, priority, title, started, ID with numeric operators, match all or any. ([React Inbox search](https://www.intercom.com/help/en/articles/16393491-search-and-filter-conversations-in-the-react-inbox))
- We do: words, phrases, `-word`, `OR`, `is:open`, `is:unread`, `is:mine` and the like (`app/support/searchquery/searchquery.go`). No `from:`, `tag:`, `assignee:` or dates, and no filter screen. Spec lists these operators.
- Plan: Lite seats cannot search.
- Work: medium. **Should have:** "finding that one thread from March" needs a date and a person.

**I-101. Search everywhere, conversations and contacts.**
- Intercom: a separate search reaches the whole inbox, with email and ID lookups at the top. ([search](https://www.intercom.com/help/en/articles/6516006-inbox-search-and-filter))
- We do: `/search` exists and no screen calls it (`app/actions/conversations/search.go`); the visible box searches the current view only (`inbox-page.tsx`).
- Plan: none stated.
- Work: small. **Should have:** the server is already built. Serves I-111.

**I-102. Sort by relevance.**
- Intercom: relevance by default, then newest or oldest. ([search](https://www.intercom.com/help/en/articles/6516006-inbox-search-and-filter))
- We do: newest activity only (`search.go`). `F29`, `F30`, `F57` cover matching, not ranking.
- Plan: none stated.
- Work: small to medium. **Should have:** the first page should hold the best match.

### Inbox: keyboard and command menu

**I-110. Navigation and action shortcuts.**
- Intercom: J and K for previous and next, Z snooze, P priority, A assign, C new, G go to inbox, X select, L layout, `[` and `]` panels, Cmd+Shift+Y close, Cmd+Shift+O reopen. ([shortcuts](https://www.intercom.com/help/en/articles/6272267-how-to-use-command-k-with-intercom-inbox))
- We do: R, N, O and Cmd+Enter (`thread.tsx`, `composer.tsx`). `/neighbors` is unused. Spec calls for J and K without a spinner.
- Plan: none stated.
- Work: small to medium. **Must have:** the cheapest way to feel fast.

**I-111. Command menu.**
- Intercom: Cmd+K lists reply, note, close, snooze, assign, go to, upload, merge, theme, language, new conversation, tags and more. ([Command-K](https://www.intercom.com/help/en/articles/6272267-how-to-use-command-k-with-intercom-inbox))
- We do: nothing. Cmd+K has no handler; `dashboard-sidebar.tsx` handles only Cmd+L.
- Plan: none stated.
- Work: medium. **Must have:** one place to reach every move.

**I-112. Shortcut list and practice.**
- Intercom: a cheat sheet in the menu and an interactive practice page. ([get started](https://www.intercom.com/help/en/articles/6274899-get-started-with-intercom-help-desk))
- We do: `Kbd` hints only in the composer.
- Plan: none stated.
- Work: small. **Should have:** shortcuts nobody knows do not help.

### Inbox: spam and blocking

**I-140. Spam folder with not spam and delete.**
- Intercom: filtered mail sits in a folder for 2 months and is deleted after 6; not spam returns it and offers to trust the sender; marking spam closes the conversation. ([FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs), [conversations FAQ](https://www.intercom.com/help/en/articles/8838326-conversations-faqs))
- We do: API view and `spam` field (`filters.go`, `attributes.go`), but no spam item in the sidebar and no control, and no spam retention. `F37` and `F61` cover wake and stickiness.
- Plan: none stated.
- Work: small to medium. **Must have:** a false positive today can only be rescued through the API.

**I-141. Block a contact from the inbox, and a blocked list.**
- Intercom: block from the profile archives the person, rejects new mail from the address; a Blocked tab undoes it. API too. ([FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs), [API](https://developers.intercom.com/docs/references/rest-api/api.intercom.io/contacts/blockcontact))
- We do: `blocked` on `PATCH /contacts` and a directory filter (`app/actions/contacts/update_contact.go`, `list_contacts.go`); no control. New mail from a blocked sender is kept as spam rather than refused (`app/actions/email/ingest_email.go`).
- Plan: none stated.
- Work: small. **Must have:** abuse handling needs a button.

**I-142. Trusted senders and a visible rate-limit label.**
- Intercom: trust an address or domain to skip spam filter, rate limit and auto-reply detection, unless a header says auto-reply. Per-sender limit shows a "too many emails" label on new conversations with a way to recover. ([FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs))
- We do: a sender throttle of 20 per 10 minutes (`config/channels.go`) recorded as metadata (`app/actions/email/inbound_message.go`), with no allow list and no label.
- Plan: none stated.
- Work: small to medium. **Should have:** a form relay or a partner system will hit the throttle and vanish from view.

### Inbox: exports and read receipts

**I-120. Export a conversation, a selection or a view.**
- Intercom: text or PDF from a conversation or selection (200 to 300 at once), CSV of a view, notes excluded and card numbers redacted. Behind a permission. ([export](https://www.intercom.com/help/en/articles/2046229-export-your-conversations-data))
- We do: nothing. `exports:*` permission names exist (`app/auth/permissions.go`); the page is sample data (`settings/pages/admin.tsx`).
- Plan: view CSV on all plans with Inbox; others not stated.
- Work: small to medium. **Should have:** legal and escalation requests need a transcript.

**I-121. Scheduled export to cloud storage.**
- Intercom: JSON to S3, teammates and activity logs to S3, GCS or Azure. ([export](https://www.intercom.com/help/en/articles/2046229-export-your-conversations-data), [activity export](https://www.intercom.com/help/en/articles/16528353-export-teammate-and-activity-log-data-to-cloud-storage))
- We do: nothing.
- Plan: not stated.
- Work: large. **Later:** self-host users already own the database.

**I-130. Seen indicator on replies.**
- Intercom: shows when a participant saw a message, including on email, and when an automated scanner did. ([inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained))
- We do: nothing for email; `seen_receipt` is a static toggle (`settings/pages/inbox.tsx`).
- Plan: none stated.
- Work: medium, needs a tracking pixel and a privacy decision. **Later:** matters more on the widget, which is unbuilt.

### Email

**E-01. Send from the customer's own address, with domain authentication.**
- Intercom: add an address, verify it by a mailed link, then add two CNAMEs (DKIM, return path) and a DMARC TXT, with a validate button and optional automatic DNS. Only authenticated addresses may send. ([connect email](https://www.intercom.com/help/en/articles/9744849-connect-your-email-support-channel), [troubleshooting](https://www.intercom.com/help/en/articles/11183745-troubleshooting-email-authentication-issues-dkim-dmarc-and-dns-setup), [FAQ](https://www.intercom.com/help/en/articles/8797915-email-sending-faqs))
- We do: From is always the forwarding address on our inbound domain (`app/actions/email/compose_reply.go` line 80, shown as "Sends as" in `settings/pages/email.tsx`). `email_domains` and `email_sending_profiles` tables exist with no action using them; `framework/customdomain` exists.
- Plan: none stated.
- Work: large. **Must have:** a customer emailing `support@acme.com` should hear back from it. Also adds real spoof protection, since an unproven address can never send.

**E-02. Choose the sending address.**
- Intercom: a workspace default, a From field on new conversations, and replies follow the address the mail came in on. ([start a conversation](https://www.intercom.com/help/en/articles/6433002-start-a-conversation-from-the-inbox), [brands](https://www.intercom.com/help/en/articles/9744860-configure-your-email-channel-to-support-multiple-brands))
- We do: replies follow the conversation's address, else the default (`send_email_message.go`). `from_address_id` is read in `compose_reply.go` but `F16` notes `Options` is never set. No default control on the settings page.
- Plan: custom senders need a permission.
- Work: small after E-01. **Should have:** needed by any workspace with two addresses.

**E-03. Signature on replies.**
- Intercom: one per brand with logo, position, size and text; made automatically. ([brands](https://www.intercom.com/help/en/articles/9744860-configure-your-email-channel-to-support-multiple-brands))
- We do: `email_signatures` table and permission names only; `account.tsx` shows a static form; no action.
- Plan: multi-brand gated. Personal signatures not confirmed.
- Work: small to medium. **Should have:** expected by every team that sends email.

**E-04. Ignored addresses.**
- Intercom: a list of company addresses never treated as participants or auto-assigned. ([group conversations](https://www.intercom.com/help/en/articles/6435588-manage-group-conversations-in-the-inbox), [delivery](https://www.intercom.com/help/en/articles/9465187-why-did-a-message-delivery-fail-from-the-inbox))
- We do: only the workspace's own forwarding and public addresses are removed (`app/actions/email/compose_reply.go`, `inspect.go`). `F23` covers own-address mail, not a custom list.
- Plan: none stated.
- Work: small. **Should have:** stops `sales@` or `billing@` becoming a customer or a copied party.

**E-05. See and lift a suppression from the customer.**
- Intercom: spam-report block removable on the profile with the contact's permission; bounce list needs support. ([delivery](https://www.intercom.com/help/en/articles/9465187-why-did-a-message-delivery-fail-from-the-inbox))
- We do: `email_suppressions` with permission names and no screen (`database/queries/email_suppressions.sql`). `F51` covers expiry and a lift action; showing it to teammates is the addition.
- Plan: none stated.
- Work: small. **Should have:** ahead of Intercom if done, since a teammate could lift it themselves.

**E-06. Strip links from inbound mail.**
- Intercom: an option that makes links non-clickable in email, with raw mail reachable for 14 days. ([FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs))
- We do: nothing beyond `F18`'s proposed raw view.
- Plan: none stated.
- Work: small. **Later:** see S-12 for the better answer.

**E-07. Send files hosted or attached.**
- Intercom: choose links or true attachments for email, 20 MB total. ([attachments](https://www.intercom.com/help/en/articles/2339426-control-how-attachments-are-uploaded-and-used))
- We do: nothing, outbound files are `F15`.
- Plan: none stated.
- Work: small after `F15`. **Later:** choose one default for now.

**E-08. Email template design.**
- Intercom: standard templates are recommended, HTML mode exists for outbound. ([FAQ](https://www.intercom.com/help/en/articles/8797915-email-sending-faqs))
- We do: one composed format (`app/integrations/email/composer.go`).
- Plan: not stated. How far this reaches inbox replies is not confirmed.
- Work: small to medium. **Later:** plain looking support mail reads fine.

**E-09. Guided DNS setup.**
- Intercom: Entri fills DNS records automatically. ([connect email](https://www.intercom.com/help/en/articles/9744849-connect-your-email-support-channel))
- We do: nothing.
- Plan: none stated.
- Work: medium, a paid third party. **Later:** plain records and a checker come first with E-01.

### Security

**S-01. Attachment scanning.**
- Intercom: scans on upload, removes confirmed malware, shows inconclusive, not fully scanned, mismatch and likely malicious with a warning; executables always blocked. ([scanning](https://www.intercom.com/help/en/articles/9222413-attachment-safety-how-we-scan-files-for-malware))
- We do: `framework/malware` has clamav and guardduty drivers; no code in `app/` calls a scanner. `attachments.scan_status` stays `pending`; the migration says a file is served only after its scan. Executables are refused on ingest (`app/actions/email/inbound_message.go`, `F47`).
- Plan: none stated.
- Work: medium. **Must have:** must land with the `F14` download route. Otherwise files are either unsafe or stuck.

**S-02. Attachment policy.**
- Intercom: turn customer uploads off, allow extra types; default allowed list of common files. ([attachments](https://www.intercom.com/help/en/articles/2339426-control-how-attachments-are-uploaded-and-used), [scanning](https://www.intercom.com/help/en/articles/9222413-attachment-safety-how-we-scan-files-for-malware))
- We do: install config only, 50 files and 25 MB (`config/channels.go`).
- Plan: audience rules depend on the messenger.
- Work: small. **Later:** the default refusal list covers most risk.

**S-03. Expiring attachment links.**
- Intercom: messenger and chat files expire after 30 minutes, email files after about six months; the API hands out a fresh signed link. An older community complaint reported unguarded links. ([attachments](https://www.intercom.com/help/en/articles/2339426-control-how-attachments-are-uploaded-and-used), [community](https://community.intercom.com/settings-security-permissions-22/image-attachment-url-are-not-secured-in-our-instance-2924))
- We do: no download route yet (`F14`).
- Plan: none stated.
- Work: small. **Must have:** decide this in the `F14` fix, and do better than Intercom with a member check plus short links.

**S-04. Limit which conversations a teammate may see.**
- Intercom: all, assigned to them, their teams, or all except named teams; optionally unassigned. Applies to inbox, reports, profiles, mobile; hides content while counts remain; OAuth tokens follow the authorizing teammate. ([limit access](https://www.intercom.com/help/en/articles/4707721-limit-teammates-access-to-conversations))
- We do: every member sees everything (`app/auth/roles.go`, `app/actions/conversations`). Realtime authorizers check membership only (`app/broadcasting/authorizers.go`). Spec defers it.
- Plan: depends on plan, tier not named.
- Work: large; touches every list, search, count and channel. **Should have:** earlier than the spec says, because retrofitting grows harder with each query added.

**S-05. Custom roles and fine permissions.**
- Intercom: per-teammate and role-based permissions for reassign, merge, export, delete notes, manage views and so on; only one role each. ([permissions](https://www.intercom.com/help/en/articles/176-teammate-permissions-how-to-control-workspace-access), [roles](https://www.intercom.com/help/en/articles/7054403-setting-up-roles-and-permissions-for-support))
- We do: owner, admin, member, set in one file (`roles.go`). The catalog design makes finer permissions cheap later.
- Plan: role-based bulk on some plans.
- Work: large. **Later:** three roles suit small teams and keep things simple.

**S-06. Workspace session length.**
- Intercom: force re-authentication after a chosen period; shortest wins across workspaces; overrides "keep me signed in"; default active within 4 days. ([session length](https://www.intercom.com/help/en/articles/8931217-setting-a-custom-session-length-for-teammates-on-your-workspace))
- We do: install-wide 14 days idle, 30 absolute (`config/sessions.go`), and sessions a user ends themselves (`app/actions/auth/end_session.go`).
- Plan: not stated.
- Work: small to medium. **Should have:** an access-policy rule fits the existing `app/auth/access_policy.go`, and a security review will ask.

**S-07. Require a sign-in method, and restrict Google to a domain.**
- Intercom: enforce 2FA, Google or SAML, turn off passwords, restrict Google to a domain. On all plans except SAML. ([2FA and SSO](https://www.intercom.com/help/en/articles/181-protect-your-account-with-2fa-google-sign-on-or-saml-sso))
- We do: 2FA rule yes. `app/auth/login_policy.go` is a hook that today allows everything.
- Plan: all plans for Google.
- Work: medium. **Should have:** the hook already exists, and it is the affordable step before SAML.

**S-08. SAML SSO with just-in-time provisioning.**
- Intercom: IdP sign-in, JIT accounts with default permissions, lock out other methods once confirmed. ([SAML](https://www.intercom.com/help/en/articles/3974587-integrate-with-an-identity-provider-and-log-in-with-saml-sso))
- We do: nothing but the login policy hook.
- Plan: Expert in one article, Enterprise in another.
- Work: large. **Later:** an upmarket feature for Intercom too.

**S-09. SCIM provisioning.**
- Intercom: IdP creates, updates and removes teammates, maps groups to roles; removal deletes and reassigns. ([SCIM](https://www.intercom.com/help/en/articles/5798401-system-for-cross-domain-identity-management-scim-provisioning))
- We do: nothing.
- Plan: limited plans, needs SAML.
- Work: large. **Later:** comes after S-08.

**S-10. IP allowlist for the dashboard.**
- Intercom: IPv4 and CIDR, must include your current address; web and mobile only. ([restrictions](https://www.intercom.com/help/en/articles/8931206-restricting-access-to-your-intercom-workspace-using-ip-restrictions))
- We do: nothing; `access_policy.go` could carry it.
- Plan: not stated.
- Work: medium. **Later:** self-host installs can do this at the network edge.

**S-11. Redact sensitive data before storing.**
- Intercom: card numbers keep the last four, SSNs are masked, up to 10 custom regex rules, applied before saving and irreversible, with a timeout per rule, on messenger, SDKs, email and call transcripts. ([redaction](https://www.intercom.com/help/en/articles/13925174-redacting-sensitive-data-in-conversations-with-custom-rules), [PAN](https://www.intercom.com/help/en/articles/8524626-redacting-primary-account-numbers-pan-in-conversations))
- We do: nothing for messages; `app/actions/audit/redact.go` serves audit entries only.
- Plan: not stated.
- Work: medium. **Should have:** customers paste cards into email, and it reduces what an agent ever reads. A card rule alone is small.

**S-12. Link warnings and blocked domains.**
- Intercom: scans incoming links, warns on untrusted, stronger warning on malicious, allow and block lists, previews on hover, on by default. ([link security](https://www.intercom.com/help/en/articles/9018904-link-security-settings))
- We do: unsafe schemes stripped in `message-body.tsx` (working, per the audit). No warning or list.
- Plan: not stated.
- Work: medium. **Later:** a hover showing the real URL is a cheap first step.

**S-13. Export the audit log and filter by date.**
- Intercom: CSV download, date range, teammate and action filters, IP shown, one year, API and webhooks. ([activity logs](https://www.intercom.com/help/en/articles/4667982-review-actions-taken-in-your-workspace-with-teammate-activity-logs))
- We do: event and member filters on screen, no CSV, no date control (`settings/pages/audit-log.tsx`). The API supports time and resuming (`app/actions/audit/list_audit_logs.go`). Same retention on cloud (`config/retention.go`).
- Plan: all plans.
- Work: small. **Should have:** auditors want a file.

**S-14. Wider audit scope.**
- Intercom: logs login failures, 2FA changes, exports, bulk deletions, secrets viewed, OAuth revocations, webhooks, macro changes, inbox configuration. ([event types](https://www.intercom.com/help/en/articles/12323918-event-types-in-teammate-activity-logs))
- We do: workspace, members, tokens, invitations and teams (`resources/js/features/settings/log-labels.ts`). Sign-in and 2FA events are in a personal log an admin cannot see (`app/actions/auth/list_security_events.go`). `F71` covers a few missing entries.
- Plan: all plans.
- Work: medium. **Should have:** admins investigating an incident cannot see a colleague's failed sign-ins today.

**S-15. Erase and export one customer's data.**
- Intercom: per-person export, permanent deletion that removes conversations and tickets, recoverable for 7 days. ([GDPR](https://www.intercom.com/help/en/articles/1385437-how-intercom-complies-with-gdpr), [export](https://www.intercom.com/help/en/articles/1636975-how-to-export-your-intercom-data-for-gdpr), [contacts FAQ](https://www.intercom.com/help/en/articles/8827723-contacts-faqs), [FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs))
- We do: `contacts:erase` is only a name (`app/auth/permissions.go`); no action or route. Spec has it in MVP.
- Plan: none stated.
- Work: medium to large; covers attachments, raw mail and the search index. **Must have:** a legal requirement for EU customers.

**S-16. Data retention controls.**
- Intercom: deleted contacts destroyed after 7 days; spam deleted after 6 months; activity logs mention a conversation auto-deletion schedule, and call recordings have a day setting. ([contacts FAQ](https://www.intercom.com/help/en/articles/8827723-contacts-faqs), [event types](https://www.intercom.com/help/en/articles/12323918-event-types-in-teammate-activity-logs), [recordings](https://www.intercom.com/help/en/articles/8488923-call-recordings-and-transcripts))
- We do: retention only for audit and security logs (`config/retention.go`). Conversations and raw mail stay forever.
- Plan: not stated. The general schedule's documentation was not found (section 6).
- Work: medium. **Should have:** raw mail kept forever is a growing liability.

### API and tokens

**P-01. Rate limits on the public API.**
- Intercom: per-app and per-workspace limits with `X-RateLimit-*` headers and 429s, spread across 10-second windows. ([rate limiting](https://developers.intercom.com/docs/references/rest-api/errors/rate-limiting))
- We do: `framework/http/middleware/throttle.go` exists; nothing in `routes`, `app/http`, `bootstrap` names it. Sign-in routes are limited in actions (`app/auth/throttle.go`). No limit found on conversation routes.
- Plan: limits differ by page, see section 6.
- Work: small. **Must have:** abuse guard and a safety net for scripts and agents.

**P-02. IP allowlist for API tokens.**
- Intercom: IPv4 and CIDR, 403 with a clear message, applies to OAuth and keys, changes logged. ([API allowlist](https://www.intercom.com/help/en/articles/13941009-restrict-rest-api-access-by-ip-allowlist))
- We do: nothing (`app/actions/tokens`).
- Plan: not stated.
- Work: small to medium. **Later:** token abilities and expiry already reduce exposure.

**P-03. Delete a conversation through the API.**
- Intercom: single conversations can be deleted by API; not in the inbox. ([conversations FAQ](https://www.intercom.com/help/en/articles/8838326-conversations-faqs))
- We do: no route; `conversations:delete` is only a name. Spec has trash in MVP.
- Plan: none stated.
- Work: small to medium. **Should have:** a mistake or a privacy request needs a way out.

**P-04. Structured conversation search.**
- Intercom: nested filter queries over many fields, with `waiting_since` sort on the list endpoint. ([search API](https://developers.intercom.com/docs/references/2.7/rest-api/api.intercom.io/conversations/searchconversations), [list API](https://developers.intercom.com/docs/references/2.0/rest-api/conversations/list-conversations))
- We do: `q` and fixed filters (`conversation_list_query.go`).
- Plan: none stated.
- Work: medium. **Later:** current filters serve dashboards and agents.

### Accounts and teams

**A-01. Edit your own profile.**
- Intercom: permission to edit own profile, and others'. ([permissions](https://www.intercom.com/help/en/articles/176-teammate-permissions-how-to-control-workspace-access))
- We do: name, photo and email controls are static (`settings/pages/account.tsx`), and `routes/api.go` has no user update route. A name can be set only at sign-up.
- Plan: none stated.
- Work: small to medium; email change needs a confirmation flow. **Must have:** the sender's name shows on every email.

**A-02. Change password from settings.**
- Intercom: changing a password ends all sessions on all devices. ([session length](https://www.intercom.com/help/en/articles/8931217-setting-a-custom-session-length-for-teammates-on-your-workspace))
- We do: reset by mail only; the "Change password" control is static (`account.tsx`), no route. `F11` covers tokens surviving a reset.
- Plan: none stated.
- Work: small. **Must have:** basic account hygiene, and the only local way to set a password when signed in another way.

**A-03. Personal time zone.**
- Intercom: snooze times use each teammate's time zone. ([snooze](https://www.intercom.com/help/en/articles/6564538-snooze-a-conversation))
- We do: `users.timezone` is read by `Location` in `status.go`, with no way to set it (`account.tsx` static). In practice presets use the workspace's zone.
- Plan: none stated.
- Work: small. **Should have:** the "9am tomorrow" preset is wrong for remote teammates.

**A-04. Sign-in methods page.**
- Intercom: Google linking and related settings per account. ([2FA and SSO](https://www.intercom.com/help/en/articles/181-protect-your-account-with-2fa-google-sign-on-or-saml-sso))
- We do: `/user/identities` list and unlink exist, and the page is static (`account.tsx`).
- Plan: none stated.
- Work: small. **Should have:** the server work is done.

**A-05. Admin helps a locked-out teammate with 2FA.**
- Intercom: a "2FA Recovery" button mails a code, and failed attempts can be reset. ([2FA and SSO](https://www.intercom.com/help/en/articles/181-protect-your-account-with-2fa-google-sign-on-or-saml-sso))
- We do: an operator command only (`app/actions/factors/remove_all_auth_factors.go`), so cloud users need our support.
- Plan: all plans.
- Work: small to medium. **Should have:** the workspace 2FA rule makes lockouts more likely. Protect with a recent confirmation and an audit entry.

**A-06. Members list shows 2FA and last active, with CSV.**
- Intercom: columns for last active, 2FA, roles, inbox seat, and export. ([teammates](https://www.intercom.com/help/en/articles/280-add-remove-delete-or-export-a-teammate))
- We do: no such column (`settings/pages/workspace.tsx`), although `last_accessed_at` and `ListMembersWithoutTwoFactor` exist.
- Plan: none stated.
- Work: small. **Should have:** the admin needs to know who has not set up 2FA before requiring it.

**A-07. Invitation extras.**
- Intercom: copy link always; reminder after 3 days to the inviter; invite configuration for team inboxes. ([teammates](https://www.intercom.com/help/en/articles/280-add-remove-delete-or-export-a-teammate), [product page](https://www.intercom.com/helpdesk/inbox))
- We do: batch invite with role, resend, withdraw, link only without mail (`settings/invitations.tsx`).
- Plan: none stated.
- Work: small. **Later:** invites work; add teams on invite first.

**A-08. Bulk member updates by CSV.**
- Intercom: add or update roles, team inboxes and limits by CSV. ([teammates](https://www.intercom.com/help/en/articles/280-add-remove-delete-or-export-a-teammate))
- We do: nothing beyond batch invite.
- Plan: Expert; updates need support to enable.
- Work: medium. **Later:** few large teams at MVP.

**A-09. Reassign when deleting a team.**
- Intercom: choose a new holder for open, snoozed and closed conversations. ([team inboxes](https://www.intercom.com/help/en/articles/197-organize-team-inboxes))
- We do: conversations lose their team with a warning (`settings/pages/team.tsx`, `app/actions/teams/delete_team.go`).
- Plan: none stated.
- Work: small. **Should have:** silently emptied team queues are easy to miss.

**A-10. Tell a teammate they joined a team.**
- Intercom: an email on being added. ([team inboxes](https://www.intercom.com/help/en/articles/197-organize-team-inboxes))
- We do: an audit entry only (`app/actions/teams/team_members.go`).
- Plan: none stated.
- Work: small. **Later:** wait for the notifications work.

### Settings

**T-01. Inbox settings page.**
- Intercom: settings for team inboxes, assignments, and snooze options. ([team inboxes](https://www.intercom.com/help/en/articles/197-organize-team-inboxes), [snooze](https://www.intercom.com/help/en/articles/6564538-snooze-a-conversation))
- We do: `routes/inbox.go` manages inboxes with reopen window and assign on reply, but `settings/pages/inbox.tsx` is sample data with a fake Billing inbox.
- Plan: team inboxes Advanced and Expert.
- Work: small. **Must have:** the page lies today and the server is ready.

**T-02. Preferences that take effect.**
- Intercom: personal theme, language and pins, and each teammate sets their own away status. ([customise](https://www.intercom.com/help/en/articles/7911926-customize-the-inbox-to-suit-you-and-how-you-work-best), [status](https://www.intercom.com/help/en/articles/6522874-manage-teammate-inbox-status))
- We do: Available, Send with Enter, signature, language and time zone do nothing (`account.tsx`). The composer always uses Cmd+Enter.
- Plan: none stated.
- Work: small each. **Should have:** controls that do nothing erode trust. Hide those not ready.

**T-03. Notification settings tied to mentions and assignment.**
- Intercom: team members are notified of assigned conversations if switched on; mentions can be muted. ([team inboxes](https://www.intercom.com/help/en/articles/197-organize-team-inboxes), [mentions](https://www.intercom.com/help/en/articles/6525765-loop-teammates-or-teams-into-conversations))
- We do: `account.tsx` shows a static matrix, and `/d/notifications` is a placeholder.
- Plan: none stated.
- Work: large, with I-50. **Should have:** ships with I-50.

**T-04. Workspace name for customers.**
- Intercom: an optional customer-facing name. ([workspace settings](https://www.intercom.com/help/en/articles/15432088-your-workspace-settings))
- We do: `email_addresses.display_name` per address, not workspace-wide, with no control for it on `settings/pages/email.tsx`.
- Plan: none stated.
- Work: small. **Later:** the per-address name does the job for email.

**T-05. Interface language.**
- Intercom: five inbox languages, via command menu or profile. ([customise](https://www.intercom.com/help/en/articles/7911926-customize-the-inbox-to-suit-you-and-how-you-work-best))
- We do: English only (`settings/pages/workspace.tsx`, `lang/`).
- Plan: none stated.
- Work: large. **Later:** after product copy settles.

## 4. Where we are already ahead or equal

Ahead, if the known bug fixes land:

- **Passkeys:** ours has them. Intercom's guide describes authenticator apps and does not mention passkeys ([2FA and SSO](https://www.intercom.com/help/en/articles/181-protect-your-account-with-2fa-google-sign-on-or-saml-sso)).
- **Workspace 2FA rule covers tokens:** `app/auth/access_policy.go` applies it to every request and every credential. Intercom describes a prompt at next login (same source).
- **API tokens:** abilities, optional expiry, single display, owner mail, cap per member (`app/actions/tokens`). Intercom's token docs do not describe scoping for private apps ([authentication](https://developers.intercom.com/docs/build-an-integration/learn-more/authentication)).
- **Deactivate and reactivate:** we keep the history and restore the member. Intercom says a removed teammate cannot be restored, only re-invited ([teammates](https://www.intercom.com/help/en/articles/280-add-remove-delete-or-export-a-teammate)).
- **Workspace trash and restore:** grace period, reminders, tokens revoked (`app/actions/workspaces/trash_workspace.go`).
- **Batch invites on every plan:** Intercom limits bulk invites to Expert (same source).
- **Search grammar:** phrases, `-word`, `OR`, prefix. Intercom says boolean logic is not supported ([search](https://www.intercom.com/help/en/articles/6516006-inbox-search-and-filter)). See `F29` and `F57`.
- **Views across statuses:** the API lets a list hold several states, which Intercom says a view cannot ([FAQ](https://www.intercom.com/help/en/articles/8838656-inbox-faqs)).
- **Mark any conversation unread:** ours works on any message from someone else (`app/actions/conversations/read_state.go`), with no screen yet. Intercom says a customer reply cannot be marked unread ([mentions](https://www.intercom.com/help/en/articles/6525765-loop-teammates-or-teams-into-conversations)).
- **Forwarding setup:** steps per provider, a test, and the Gmail code shown in settings. Intercom's guide also supports forwarding ([connect email](https://www.intercom.com/help/en/articles/9744849-connect-your-email-support-channel)), though it refuses personal mail domains and Google Groups (same source). Whether to allow groups is `F22`.
- **Sender name style:** ours offers two; Intercom says the name cannot be changed ([brands](https://www.intercom.com/help/en/articles/9744860-configure-your-email-channel-to-support-multiple-brands)).
- **Inbound authentication:** we verify DKIM and DMARC ourselves (`app/integrations/email/auth.go`).
- **Collision guard:** `expected_last_message_id` refuses a stale reply at the server. Not reachable from the composer until `F38`. No public Intercom equivalent found.
- **Idempotent writes and self-host:** retried replies replay; the product runs from one binary.

Equal:

- Reply and note drafts kept apart, in a single session. R, N, Cmd+Enter. Assign to me against "Pull conversation". Notes and closing do not assign ([get started](https://www.intercom.com/help/en/articles/6274899-get-started-with-intercom-help-desk)).
- Presence with up to three avatars, in the conversation header; Intercom shows it on rows (I-07).
- Snooze presets read in the teammate's own time zone, once A-03 lands ([snooze](https://www.intercom.com/help/en/articles/6564538-snooze-a-conversation)).
- Email threading by headers, quoted history toggle, forwarded customer detection ([threading](https://www.intercom.com/help/en/articles/7996715-email-threading), [forwarded](https://www.intercom.com/help/en/articles/7339364-detect-customers-in-forwarded-emails)).
- Hard bounce and complaint suppression ([delivery](https://www.intercom.com/help/en/articles/9465187-why-did-a-message-delivery-fail-from-the-inbox)), with `F51` for accuracy.
- Enforced 2FA and recovery codes ([2FA and SSO](https://www.intercom.com/help/en/articles/181-protect-your-account-with-2fa-google-sign-on-or-saml-sso)). Audit retention of a year on cloud ([activity logs](https://www.intercom.com/help/en/articles/4667982-review-actions-taken-in-your-workspace-with-teammate-activity-logs)).
- Inbound size: ours 25 MB per file, Intercom 20 MB per email ([attachments](https://www.intercom.com/help/en/articles/2339426-control-how-attachments-are-uploaded-and-used)).
- Light, dark and system themes ([customise](https://www.intercom.com/help/en/articles/7911926-customize-the-inbox-to-suit-you-and-how-you-work-best)).

## 5. Depends on unbuilt features

- **Macros and saved replies:** `\` in the composer, bulk reply with macro, snooze in a macro.
- **Workflows:** assignment rules, auto-close, auto-reassign, priority by rule, routing by channel.
- **SLAs:** SLA sorts, SLA filters, SLA in merge behaviour.
- **Tickets:** back-office and tracker tickets, linked tickets, ticket states, the Links panel.
- **Reports and real-time dashboard:** status control, aggregates, topic and CSAT data.
- **Fin and Copilot:** AI answers, autofill, AI topics, Copilot tab, summaries in transcripts.
- **Messenger:** customer-visible status, seen on chat, identity verification with JWT, trusted domains, conversation-started filters, audience upload rules, typing to customers.
- **Webhooks:** subscribing to activity logs, `api.request.completed`, signed delivery (`X-Hub-Signature` is HMAC-SHA1 with the client secret, per a search summary of [webhook topics](https://developers.intercom.com/docs/references/webhooks/webhook-models)). `framework/webhooks` exists; no routes are built.
- **OAuth apps:** token scopes, regeneration on access change.
- **Other channels:** Slack side conversations, phone, WhatsApp merge rules, voice notes.
- **Brands and office hours:** per-brand address and signature, team hours.
- **Custom objects and apps:** right sidebar apps, data connectors.
- **Tours and outbound:** bulk messaging, scheduled email.

## 6. Not verified

Page text was read through a fetch tool that summarises pages, so short details may differ from the original. Search result summaries were used for a few items, marked below.

- **Plan names disagree.** SAML is Expert in one article and Enterprise in another. Round robin's plan comes from the status article, not its own. Many articles name no plan, so "none stated" means the page is silent, not that it is free.
- **Rate limits.** One page says 10,000 a minute per app and 25,000 per workspace; an older one says 1,000. Taken from search summaries, the page itself was not read.
- **Undo send.** The 5 to 30 second window comes from the conversations FAQ. Who sets it, and whether it applies per teammate, was not found.
- **Composer drafts.** Listed as shipped, with no public detail on persistence or sharing.
- **Formatting in inbox replies.** The toolbar detail comes from an article on outbound composing, seen through a search summary.
- **Forwarding a conversation by email.** No document found either way, so it is not listed as a gap.
- **Split conversation.** Merge docs do not describe it. In Messenger, removing a user during a reply splits it into a new email thread ([group conversations](https://www.intercom.com/help/en/articles/6435588-manage-group-conversations-in-the-inbox)). No manual split found, so not listed.
- **Teammate-to-teammate typing.** Presence is documented; a typing indicator between teammates was not found. `F88` covers ours.
- **Personal email signatures.** Only per-brand signatures were documented.
- **Teammate notification settings.** The specific article was not found; I-50 and T-03 rely on the mention and team articles.
- **Conversation auto-deletion.** An activity log event name implies it exists; no article was found (S-16).
- **Forced sign-out of a teammate.** The docs recommend changing their password; no dedicated button was found, so it is not listed separately.
- **Workspace ownership transfer.** No Intercom article found; we have a handover.
- **Private-app token regeneration and scoping.** The developer pages read do not cover them.
- **Inbox performance limits.** Intercom warns complex views may time out ([views](https://www.intercom.com/help/en/articles/6588834-organize-your-inbox-with-custom-views-and-folders)). No numeric target found to compare with our spec.
- **Person tags (I-61).** The wording came from a search summary of the search help page, not a dedicated article read here.
- **Our side.** All "we do not do it" claims come from reading and grepping the working tree on 2026-11-04. Nothing was run, as builds and servers were off limits. The `F` bug fixes under way may change a few details, chiefly P-01, S-01 and the sample settings pages.
