# Edge case audit against Chatwoot and Zammad

Behaviours were learned from both products and described in our own words. No code, regex or fixture was copied. Nothing here has been fixed.

## 1. Scope

| Repo | Commit |
|---|---|
| dimmi | `b8eaecdd7d5b43262d83b452926c03efaaa5b448` |
| chatwoot | `656d603a3cde9c2a3ab09c26cdfe1181bad444be` |
| zammad | `3a97440410292238905178f39a1e306f044baf98` |

Features audited:

- **Email:** inbound routing and parsing, HTML and quote handling, threading, classification (auto-reply, bounce, complaint, loop, forward, form relay, list, spam), attachments, outbound replies and headers, forwarding addresses.
- **Markdown:** thread rendering (`message-body.tsx`), list excerpts, outbound rendering.
- **Inbox:** conversations, messages, contacts, inboxes, search, filters, counts, read state, snooze, assignment, statuses, notes, composer, thread, list.
- **Realtime:** `app/broadcasting`, `framework/broadcasting`, `resources/js/lib/realtime`.
- **Accounts:** sign in, sessions, 2FA, passkeys, recovery codes, workspaces, members, invitations, teams, API tokens, audit log.

Out of scope: workflow builder, widget, help center content, SES, custom domains, tags UI, saved replies UI, merge, bulk.

How it was checked:

- Our parser and body builder ran over all 122 Zammad mails (`test/data/mail/*.box`, compared with their `.yml` expectations) and 43 Chatwoot fixtures (`spec/fixtures/files/*.eml`, read by eye). Their files were input only; their code was never run.
- Synthetic batteries ran against our parser, classifier and renderer, including `react-markdown` with our real plugins.
- End to end checks used virtual test files through `go test -overlay` against the real harness and services, plus read-only `to_tsquery` checks in PostgreSQL.
- No real LLM was called. The repo gained only this file.
- Evidence "test" means output was observed. Proof programs and output are in the session scratchpad, under `/private/tmp/claude-501/-Users-bahdcoder-code-trydimmi-com/66ae38c8-6a72-4473-ab4e-17a932565aa2/scratchpad/edge-audit/` (`scratch/`, `out/`). That folder is temporary.

Critical here follows the brief: data loss, security or mail loop. A mail counted as lost when no teammate can reach the words in the product, even though the raw `.eml` stays on disk. No screen shows raw mail or stored HTML (F18), and the ignored and failed rows do not appear in the thread or inbox lists.

Where neither product handles a case and it surfaced while testing, the finding says so.

## 2. Summary

| Category | Critical | High | Medium | Low | Total |
|---|---|---:|---:|---:|---:|
| Email parsing | 6 | 5 | 8 | 5 | 24 |
| Threading | 0 | 0 | 1 | 1 | 2 |
| Classification | 2 | 4 | 6 | 1 | 13 |
| Outbound | 0 | 3 | 1 | 1 | 5 |
| Markdown/rendering | 0 | 3 | 1 | 2 | 6 |
| Inbox logic | 0 | 9 | 7 | 5 | 21 |
| Realtime | 1 | 2 | 1 | 2 | 6 |
| Accounts | 4 | 1 | 8 | 6 | 19 |
| **Total** | **13** | **27** | **33** | **23** | **96** |

By type: **Incorrect** 59, **Missed** 37. IDs are stable labels; a few sit out of number order after grading.

## 3. Findings

### Critical

#### F01. A display name the parser dislikes drops the whole email
- **Category:** email parsing. **Type:** Incorrect. **Severity:** critical.
- **Input:** `From: Mueller, Hans <hans@customer.test>`. Same result for `=?iso-8859-1?Q?B=FCrling,=20Andreas?= <a@x>`, `Foo [Bar] <a@x>`, `Foo: Bar <a@x>`, `Foo @ Bar <a@x>`, `Bob <bob@yahoo.com.>`, `"A" <>, "B" <b@x>`, `"hans@x.de"`, `John Doe john@x.com`, two `From:` lines, or a stray header line with no colon after `From:`.
- **Now:** the address list fails to parse, From is empty, and the row ends `ignored`, reason `unparseable`, with no conversation. The envelope sender was available and unused.
- **Should:** recover the address from the angle brackets, last valid address, `Sender` or the envelope sender.
- **Ours:** `framework/mailbox/parse/parse.go:206`, `app/actions/email/ingest_email.go:80`.
- **Theirs:** Zammad `app/models/channel/email_parser.rb:413` with corpus `mail044`, `mail059`, `mail065`, `mail017`; Chatwoot `app/presenters/mail_presenter.rb` tries several headers.
- **Evidence:** test. `[comma] inbound status=ignored class=normal ignore="unparseable" conversations=0`.
- **Fix:** add a tolerant fallback address reader, then envelope sender. Never ignore mail that has a recoverable sender.

#### F02. Deeply nested HTML fails the email for good
- **Category:** email parsing. **Type:** Incorrect. **Severity:** critical.
- **Input:** HTML nested over 512 levels. It occurs in real mail: Chatwoot's `mail_with_html_part_no_html_content.eml` and Zammad's `mail113.box` both hit it.
- **Now:** `BuildBody` returns an error, the job fails, and the row ends `failed`. Retrying cannot help. The text part was there and unused.
- **Should:** keep the mail: use the text part, or strip tags from the HTML.
- **Ours:** `framework/dom/dom.go:47`, `app/integrations/email/htmlbody.go:87`, `app/actions/email/inspect.go:73`.
- **Theirs:** Zammad `lib/core_ext/string.rb:359` falls back to text conversion; Chatwoot has the fixture above in its mailbox specs.
- **Evidence:** test. `status=failed err="build body: parse html: html: open stack of elements exceeds 512 nodes"`.
- **Fix:** treat an HTML parse failure as a warning and fall back to the text part.

#### F03. A real question whose subject starts like an auto-reply is dropped
- **Category:** classification. **Type:** Incorrect. **Severity:** critical.
- **Input:** first mail, subject `Automatic reply not working on my account`, or `Out of office hours support question`, under 1000 characters.
- **Now:** classified `auto_reply`, then ignored as `auto_reply_no_thread`. Our header says the subject counts only beside a header signal or on a short message, and a customer's question is a short message.
- **Should:** subject alone never discards first contact.
- **Ours:** `app/integrations/email/classifier.go:35`, `:119`; `app/actions/email/ingest_email.go:156`.
- **Theirs:** Chatwoot `mail_presenter.rb` `auto_reply?` and Zammad `app/models/channel/filter/auto_response_check.rb` use headers only.
- **Evidence:** test. `[subject-ooo] status=ignored class=auto_reply ignore="auto_reply_no_thread"`.
- **Fix:** require a colon-style exact prefix plus a header, or keep the message as a quiet automatic one.

#### F04. Any first mail with `Auto-Submitted: auto-generated` is dropped
- **Category:** classification. **Type:** Incorrect. **Severity:** critical.
- **Input:** a form service, monitor or ticket system mails `no-reply@webflow.com` style, with `Auto-Submitted: auto-generated` and `Reply-To: visitor@…`.
- **Now:** the auto-reply rule runs before the form relay and bulk rules, so it is ignored. The bulk handling setting never applies.
- **Should:** store it and never answer it automatically. An auto-reply is only `auto-replied`.
- **Ours:** `app/integrations/email/classifier.go:92`, `:127`; `app/actions/email/ingest_email.go:156`.
- **Theirs:** Zammad `auto_response_check.rb` only blocks automatic responses and the ticket is still created; Chatwoot stores it flagged, `auto_reply` in `mail_presenter.rb`.
- **Evidence:** test. `[form-autosub] status=ignored class=auto_reply ignore="auto_reply_no_thread"`.
- **Fix:** split `auto-generated` from `auto-replied`, route the former to bulk or form relay handling.

#### F05. Text after an attachment in the body is lost
- **Category:** email parsing. **Type:** Incorrect. **Severity:** critical.
- **Input:** Apple Mail with a PDF mid-message: `multipart/mixed` holding html, pdf, html. Or `multipart/mixed` with text, pdf, text.
- **Now:** only the first HTML part is used. Two text parts are joined with a `--` line by the parser, which our signature rule then cuts.
- **Should:** join all body parts in order.
- **Ours:** `framework/mailbox/parse/parse.go:141`, `app/integrations/email/quotes.go:185`.
- **Theirs:** Zammad `mail085.box` keeps all the text; its expected body has the words ours lost.
- **Evidence:** test. `apple html+pdf+html md="First para"`, `mixed text+pdf+text text="First para\n--\nSecond para" md="First para"`, `mail085 MD: '123'`.
- **Fix:** walk the part tree for inline text and HTML parts and concatenate with a neutral separator.

#### F06. An empty or stub HTML part hides a real text part
- **Category:** email parsing. **Type:** Incorrect. **Severity:** critical.
- **Input:** `multipart/alternative` with real text and `<html><body></body></html>`, or HTML whose content is all hidden.
- **Now:** the Markdown is empty and the message is blank. The text part is never consulted once HTML is non-blank.
- **Should:** use text when HTML yields nothing.
- **Ours:** `app/integrations/email/htmlbody.go:83`, `:108`, `:144`.
- **Theirs:** Zammad `mail026.box` is such a mail; its expected result was not compared word for word.
- **Evidence:** test. `alt text + empty html text="Real text" md=""`; `mail026` text 279, html 537, md `''`.
- **Fix:** `BuildBody` falls back to `textBody` when the HTML Markdown is empty.

#### F07. Plain text stripping deletes real content for good
- **Category:** email parsing. **Type:** Incorrect. **Severity:** critical.
- **Input and result:**

| Mail body | Stored as |
|---|---|
| `Hi team,` / `On Monday my manager wrote:` / `The invoice must be paid.` / `Can you confirm?` | `Hi team,` |
| `On Monday I wrote` (no colon) then text | first line only |
| `Order details` / 22 underscores / `Order 123 is missing` | `Order details` |
| `New contact form` / `From: Jane` / `Date: 2024-01-05` / `Message: my order never arrived` | `New contact form` |
| `Our server shows:` / `From: billing@…` / `Date: Jan 3` / `To: me` / `Why did it fail?` | first line only |
| `Steps:` / `--` / `run the installer` / `reboot` / `it crashes` | `Steps:` |
| `Run this:` / `> rm -rf build` / `> make install` | `Run this:` |

- **Now:** plain text mail has no stored HTML, so what is cut survives only in the raw file.
- **Should:** a quote header needs corroboration, such as quoted lines after it. Cut text stays reachable.
- **Ours:** `app/integrations/email/quotes.go:44`, `:142`, `:185`; `htmlbody.go:123`.
- **Theirs:** Chatwoot `mail_presenter.rb` `text_content` stores `full` beside the trimmed copy; Zammad keeps the whole article and does not cut.
- **Evidence:** test (`out/b2.txt`).
- **Fix:** require a date or address in the header line, require `>` lines below it, and store the full text beside the visible text.

#### F08. A malformed top level drops the email too
- **Category:** email parsing. **Type:** Incorrect. **Severity:** critical.
- **Input:** `Content-Type: multipart/mixed` with no boundary, a leading blank line, or a junk first line (`---- Original email message ----`).
- **Now:** the parser falls back to a plain text body and never reads the header, so From is empty and the mail is ignored `unparseable`. With a boundary that does not match the body, From survives but the body is empty.
- **Should:** read the header block regardless; a broken boundary becomes raw text.
- **Ours:** `framework/mailbox/parse/parse.go:137`.
- **Theirs:** Zammad `mail068.box` expects the sender and subject recovered.
- **Evidence:** test. `no boundary param from=""`, `leading blank line from=""`, `boundary mismatch md=""`.
- **Fix:** parse headers independently of the MIME body, skipping junk before the first header line.

#### F09. A stranger's linked OAuth login survives the account claim
- **Category:** accounts. **Type:** Incorrect. **Severity:** critical.
- **Input:** attacker registers `victim@x` and links their own Google (no verified address required). The victim proves the inbox by magic link or reset.
- **Now:** the claim removes password, sessions, grants and tokens but not `oauth_identities`. The attacker signs in with Google: `302 loc=/`, victim's user has 2 sessions. The attacker's workspace stays too.
- **Should:** delete identities linked before the address was proven.
- **Ours:** `app/actions/auth/claim_account.go:62`, `app/actions/auth/reset_password.go:254`.
- **Theirs:** Chatwoot `app/controllers/devise_overrides/omniauth_callbacks_controller.rb:153`, `app/models/user.rb:65` (confirmable).
- **Evidence:** test.
- **Fix:** delete OAuth identities and factors in ClaimAccount, and run the claim from reset on unverified accounts.

#### F10. A stranger's TOTP survives the claim and locks the owner out
- **Category:** accounts. **Type:** Incorrect. **Severity:** critical.
- **Input:** attacker registers `victim2@x`, activates TOTP while unverified. The owner consumes a magic link.
- **Now:** `200 two_factor_challenge`, `hasPassword=false verified=true 2fa=true`. Only the attacker holds codes.
- **Should:** the claim wipes factors, or enrolment needs a verified address.
- **Ours:** `app/actions/auth/claim_account.go:62`, `app/actions/auth/complete_primary_auth.go:104`.
- **Theirs:** Chatwoot's confirmable keeps unconfirmed users out, `app/models/user.rb:65`.
- **Evidence:** test.
- **Fix:** as F09.

#### F11. Personal access tokens survive a password reset
- **Category:** accounts. **Type:** Incorrect. **Severity:** critical.
- **Input:** user mints a token, then resets their password after a compromise.
- **Now:** the token returns 200 before and after. A token minted on an unverified account survives too.
- **Should:** revoke, or at least tell the user how many remain.
- **Ours:** `app/actions/auth/reset_password.go:254-268`.
- **Theirs:** only partly handled: Chatwoot rotates its pubsub token on password change (`app/models/concerns/pubsubable.rb:12`), not the API token.
- **Evidence:** test.
- **Fix:** call `RevokeUserTokens` in `endEverySession` and log the count.

#### F12. Live sockets keep streaming after sign-out, reset and token revoke
- **Category:** realtime. **Type:** Missed. **Severity:** critical.
- **Input:** an open socket, then sign-out, password reset, session delete or token revoke.
- **Now:** the socket still subscribes to new `conversation:` channels and receives full conversation payloads. Only `MemberDeactivated` disconnects.
- **Should:** end connections when their credential ends.
- **Ours:** `app/broadcasting/accounts.go:62-64`, `app/actions/auth/end_session.go:88`, `framework/broadcasting/connect.go`.
- **Theirs:** Zammad `app/models/user/terminates_sessions.rb`; Chatwoot `pubsubable.rb:12`, `spec/models/user_spec.rb:37`.
- **Evidence:** test, by two separate probes.
- **Fix:** add listeners that call `Disconnect`, tracking credential per connection, and re-validate in the channel authorizers.

#### F13. Owner deactivation skips the token and recent-confirmation guards
- **Category:** accounts. **Type:** Incorrect. **Severity:** critical.
- **Input:** an owner's token with `members:write` deactivates a co-owner: 200, and the co-owner's tokens are revoked. The same token demoting gets 403. A session idle 2h: demote 423, deactivate 200.
- **Should:** anything touching an owner needs first party and a recent confirmation.
- **Ours:** `app/actions/members/update_member.go:222`, `:278`.
- **Theirs:** neither has an equivalent; surfaced while testing.
- **Evidence:** test.
- **Fix:** apply both checks to any status change on an owner.

### High

#### F14. Inbound attachments are stored but no one can see them
- **Category:** email parsing. **Type:** Missed. **Severity:** high.
- **Input:** any email with a file, or with only a file.
- **Now:** `TimelineEntry` has no attachments field and the thread renders none. An attachment-only mail shows an empty bubble. `skipped_attachments` metadata is not shown either.
- **Should:** list files in the thread with type, size and scan state.
- **Ours:** `app/http/resources/message_resource.go:38`, `resources/js/features/inbox/thread.tsx:81`.
- **Theirs:** Chatwoot `app/mailboxes/mailbox_helper.rb`; Zammad article attachments.
- **Evidence:** read only, plus grep over `resources/js` and `app/http`.
- **Fix:** return attachments on the timeline and render them; add a download route with safe headers.

#### F15. Files added to a reply look attached and are silently thrown away
- **Category:** outbound. **Type:** Incorrect. **Severity:** high.
- **Input:** teammate drops a PDF in the composer, sees it land at 100%, and sends.
- **Now:** upload is simulated and `attachments.clear()` runs on send. `outboundMessage` never sets attachments. The customer gets no file and nothing tells the teammate. Reported because a built feature misbehaves, not for missing uploads.
- **Should:** upload for real, or refuse to pretend.
- **Ours:** `resources/js/features/inbox/use-attachments.ts:2`, `composer.tsx:187`, `app/jobs/channel_outbound.go:84`.
- **Theirs:** both send attachments.
- **Evidence:** read only.
- **Fix:** hide the control until uploads exist.

#### F16. People copied on a thread never receive replies
- **Category:** outbound. **Type:** Incorrect. **Severity:** high.
- **Input:** customer writes with `Cc: boss@…`. Teammate replies.
- **Now:** the boss is stored as a CC participant, but nothing fills `OutboundMessage.CC`, so `compose_reply.go:121` loops over nothing. `Options` and `ReplyTo` are never set either, so the capability `ParticipantsCC: true` is false in practice. Our own feature test sets the field by hand, so it passes.
- **Should:** keep copies, as the code comment promises.
- **Ours:** `app/jobs/channel_outbound.go:84-87`, `app/actions/email/compose_reply.go:120`.
- **Theirs:** Chatwoot `spec/fixtures/files/reply_cc.eml` and its reply mailer (not traced in full).
- **Evidence:** test. `customer mails=1 boss mails=0`, `cc=[]`.
- **Fix:** load participants in `outbound`, and decide about To recipients when the workspace was blind-copied.

#### F17. Reply-To is ignored for ordinary mail
- **Category:** outbound. **Type:** Missed. **Severity:** high.
- **Input:** `From: bob@a.test`, `Reply-To: bob-real@b.test`.
- **Now:** contact and reply use From. Only form relays read Reply-To.
- **Should:** answer where the sender asked, as every mail client does.
- **Ours:** `app/actions/email/ingest_email.go:307`, `app/actions/email/compose_reply.go:115`.
- **Theirs:** Chatwoot `mail_presenter.rb` `original_sender`; Zammad `app/models/channel/filter/reply_to_based_sender.rb`.
- **Evidence:** test. `from-mailbox=1 replyto-mailbox=0`.
- **Fix:** store Reply-To on the conversation and send to it, keeping contact identity on From.

#### F18. Quoted text, full HTML and "View original" are unreachable
- **Category:** email parsing. **Type:** Missed. **Severity:** high.
- **Input:** any stripped or over-64KB mail.
- **Now:** `body_html` is stored but no API or screen returns it. A truncated body says `[Message truncated. View original.]` with nothing to click. Our quotes header says the full message stays available.
- **Should:** a "show full message" control and a raw download.
- **Ours:** `app/http/resources/message_resource.go:38`, `app/integrations/email/htmlbody.go:54`.
- **Theirs:** Chatwoot keeps `full` and `reply`, in `mail_presenter.rb`; Zammad's articles keep the full body (`test/data/mail/*.yml`).
- **Evidence:** read only, grep.
- **Fix:** expose sanitized HTML in a sandboxed view.

#### F19. Answers written inside a quote, and forwarded mail, are hidden
- **Category:** email parsing. **Type:** Incorrect. **Severity:** high.
- **Input:** Gmail `See my answers inline` then a `gmail_quote` holding `Order 555` between quotes. Or a customer forwards an invoice from Gmail, Outlook web or Apple Mail with `Is this right?` on top.
- **Now:** Markdown is only the top line. The inline-reply check covers only `blockquote[type=cite]`, and a non-teammate's forward is treated as a quote.
- **Should:** own text inside a quote keeps the quote; a forward is content, shown collapsed at worst.
- **Ours:** `app/integrations/email/htmlbody.go:201`, `:225`; `app/actions/email/inspect.go:84`.
- **Theirs:** Zammad keeps the whole body; Chatwoot keeps `full`.
- **Evidence:** test. `gmail top+inline md="See my answers inline"`, `gmail fwd customer md="Is this invoice right?"`.
- **Fix:** look for non-quote text inside the marked container, and exempt forward markers from cutting.

#### F20. HTML tables lose their separators
- **Category:** email parsing. **Type:** Incorrect. **Severity:** high.
- **Input:** `<tr><td>Name</td><td>Jane Doe</td></tr><tr><td>Email</td><td>jane@x.com</td></tr>`, common in forms and order mail.
- **Now:** `NameJane DoeEmailjane@x.comAmount$50`. This also defeats `BodyEmail` on HTML forms.
- **Should:** cells and rows stay apart.
- **Ours:** `framework/dom/convert.go:43` (no table plugin).
- **Theirs:** Zammad `lib/core_ext/string.rb:102` keeps rows on lines.
- **Evidence:** test.
- **Fix:** enable the library's table plugin in its small module, with a fallback of one line per row.

#### F21. Email images never display
- **Category:** email parsing. **Type:** Missed. **Severity:** high.
- **Input:** a screenshot inline (`cid:`), embedded (`data:`) or remote.
- **Now:** the Markdown holds `![](cid:…)`. `react-markdown` strips `cid:` and `data:`, leaving `<img alt>` with no source. Remote images are blocked by `img-src 'self' data: blob:`, which also logs a CSP violation. The attachment is unreachable as well (F14).
- **Should:** rewrite `cid:` to the stored file; offer remote images on click.
- **Ours:** `resources/js/features/inbox/message-body.tsx:55`, `app/http/middleware/content_security_policy.go:82`.
- **Theirs:** Chatwoot `app/mailboxes/mailbox_inline_attachment_helper.rb`; Zammad `lib/html_sanitizer/cid_to_src.rb`.
- **Evidence:** test. `![y](cid:abc@x)` renders `<img alt="y"/>`.
- **Fix:** give `MessageBody` an image component that maps `cid:` to an attachment URL.

#### F22. A Google Group or list address in front of the inbox loses the customer
- **Category:** classification. **Type:** Incorrect. **Severity:** high.
- **Input:** `support@acme` is a Google Group forwarding to dimmi: `From: 'Jane' via Support <support@acme>`, `X-Google-Group-Id`, `X-Original-Sender: jane@customer`.
- **Now:** classified `list`, conversation is closed on arrival (default), contact is `support@acme`, name `'Jane Customer' via Support`.
- **Should:** the original sender becomes the contact and the mail stays open.
- **Ours:** `app/integrations/email/classifier.go:96`, `:141`; `app/actions/email/ingest_email.go:153`, `:307`; `app/models/email_address_settings.go:32`.
- **Theirs:** Chatwoot `mail_presenter.rb` `original_sender`, fixture `group_sender_support.eml`.
- **Evidence:** test. `[ggroup] class=list conv status=closed contact=support@acme.test`.
- **Fix:** read `X-Original-Sender` and `X-Original-From`, trusted only when the group is the workspace's own forwarding source.

#### F23. Mail from our own address becomes a customer
- **Category:** classification. **Type:** Missed. **Severity:** high.
- **Input:** a teammate sends from `support@` in Gmail to a customer and copies the inbox. Or mail from the forwarding address to itself without a stamp.
- **Now:** a conversation opens with the workspace's own address as the contact. Replies then fail, since own addresses are removed from recipients. `insp.own` is used only for participants.
- **Should:** treat it as outgoing, with the first To as the customer.
- **Ours:** `app/actions/email/ingest_email.go:290-307`, `app/actions/email/inspect.go:83`.
- **Theirs:** Zammad `app/models/channel/filter/sender_is_system_address.rb`, with a spec.
- **Evidence:** test. `[own-from] contact=acme-ada-lovelace-1@inbound.test`.
- **Fix:** add an own-sender rule to the classifier; file it under the first non-own recipient.

#### F24. A teammate replying by email is stored as a customer message
- **Category:** classification. **Type:** Incorrect. **Severity:** high.
- **Input:** a teammate answers the thread from their mailbox.
- **Now:** `author_type=contact type=message`, a contact is made for the teammate, the conversation wakes, and it counts as customer activity. Nothing reaches the customer. `Member` only affects unthreaded mail, and only once authenticated.
- **Should:** store it as the member's, and either send it on or file it as a note.
- **Ours:** `app/integrations/email/classifier.go:95`, `app/actions/email/ingest_email.go:164`.
- **Theirs:** Zammad `sender_is_system_address.rb`, `detect_article_sender`.
- **Evidence:** test.
- **Fix:** when the sender is an authenticated member on a thread, append as that member, as a note at minimum.

#### F25. Read receipts reopen closed conversations
- **Category:** classification. **Type:** Missed. **Severity:** high.
- **Input:** Outlook sends `multipart/report; report-type=disposition-notification`, `Subject: Read: …`, `In-Reply-To` our reply, with no auto headers.
- **Now:** class `normal`, threaded, closed conversation reopens.
- **Should:** drop it, or at most note the read.
- **Ours:** `app/integrations/email/classifier.go:83`, `framework/mailbox/internal/report/report.go:56`.
- **Theirs:** neither has explicit handling (no `disposition-notification` in either tree); surfaced while testing.
- **Evidence:** test. `[mdn] class=normal conv status=open`, from closed.
- **Fix:** treat that report type as automatic and non-waking.

#### F26. Message text with angle brackets disappears in the thread
- **Category:** Markdown/rendering. **Type:** Incorrect. **Severity:** high.
- **Input and result:** plain text mail is stored raw and rendered as Markdown with `skipHtml`.

| Text | Renders as |
|---|---|
| `The <div> tag breaks and <NullPointerException> appears` | `The  tag breaks and  appears` |
| `Regards <John Smith>` | `Regards ` |
| `List<String> returns null and x<y>z` | `List returns null and xz` |
| `<div>whole line in div</div>` then `next line` | nothing at all, including the next line |
| `<script>alert(1)</script>` | nothing at all |

- **Now:** safe, but the words vanish, and the list excerpt drops them too (`The  breaks`). A developer customer's message can render blank.
- **Should:** show literal text.
- **Ours:** `resources/js/features/inbox/message-body.tsx:55`, `app/integrations/email/htmlbody.go:123`, `framework/markdown/text.go`.
- **Theirs:** Zammad `text2html` escapes plain text (`lib/core_ext/string.rb`); Chatwoot keeps plain text apart in `text_content`.
- **Evidence:** test (`out/md.txt`).
- **Fix:** store plain text mail escaped, as the HTML path already is, or mark its format and render it as text. Applies to F27 and F49 too.

#### F27. Numbered lists renumber from 1, and years become list items
- **Category:** Markdown/rendering. **Type:** Incorrect. **Severity:** high.
- **Input:** `3. third step` / `4. fourth step`, or `2024. was a good year`.
- **Now:** our `ol` component drops `start`, so they show `1.` and `2.`, and `2024.` shows `1.`. It also affects teammate notes and HTML mail with `<ol start=5>`, which converts to `1.`.
- **Should:** keep the writer's number.
- **Ours:** `resources/js/features/inbox/message-body.tsx:35`.
- **Theirs:** Zammad does not parse plain text as a list.
- **Evidence:** test.
- **Fix:** pass `start` through, plus the F26 escape.

#### F28. Single line breaks render in our thread but not in the email
- **Category:** Markdown/rendering. **Type:** Incorrect. **Severity:** high.
- **Input:** teammate writes `Hi,` newline `Thanks,` newline `Ana`.
- **Now:** the thread uses `remark-breaks`, so three lines. The email HTML has `<p>Hi,\r\nThanks,…</p>` and the text twin has `Hi, Thanks, Ana`. The customer gets one line.
- **Should:** the same breaks everywhere.
- **Ours:** `app/jobs/channel_outbound.go:77`, `:85`; `framework/markdown/text.go`; `message-body.tsx:16`.
- **Theirs:** Chatwoot `app/helpers/email_helper.rb:10` renders with hard breaks.
- **Evidence:** test. `HTML="…<p>Hi,\r\nline two\r\n<a…"`, `TEXT="Hi, line two link…"`.
- **Fix:** add a hard break option to `markdown.Options` for this renderer, and keep line breaks in `PlainText`.

#### F29. Searching for an email, order id, URL, filename or phone in messages finds nothing
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** high.
- **Input and result** against the `english` tsvector:

| Message contains | Query compiled | Match |
|---|---|---|
| `foo@bar.com` | `'foo' <-> 'bar' <-> 'com'` | false |
| `ORD-1234` | `'ord' <-> '1234'` | false (lexeme is `-1234`) |
| `1.2.3` | `'1' <-> '2' <-> '3'` | false |
| `my_report.pdf` | `'my' <-> 'report' <-> 'pdf'` | false |
| `https://x.com/a?b=1`, `x.com/pricing` | split on punctuation | false |
| `+1-555-123-4567` | split | false |

- **Should:** find what the teammate pasted.
- **Ours:** `app/support/searchquery/searchquery.go:242`, `database/queries/conversation_lists.sql:52`.
- **Theirs:** Chatwoot `app/finders/conversation_finder.rb:139` uses ILIKE; Zammad searches through its index.
- **Evidence:** test (`scratch/ts.sql`).
- **Fix:** build the query with `plainto_tsquery` per field (it tokenizes as the vector does), bound, plus a trigram or ILIKE fallback.

#### F30. Searching a bare number never looks in messages
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** high.
- **Input:** `48213`, an order number in a message.
- **Now:** read as a jump to conversation #48213 only. Not found, then no results, since `TSQuery` is empty. The list view applies only the number filter.
- **Should:** jump when it exists, and also search.
- **Ours:** `app/support/searchquery/searchquery.go:94`, `app/actions/conversations/search.go` (`conversations`), `filters.go` (`listParams`).
- **Theirs:** Chatwoot `conversation_finder.rb:139` searches content.
- **Evidence:** read only, plus parser output `"1042" ts="" num=1042`.
- **Fix:** keep the words in `TSQuery` beside `Number`.

#### F31. Snoozed conversations never wake
- **Category:** inbox logic. **Type:** Missed. **Severity:** high.
- **Input:** snooze until tomorrow, advance 3 days, drain jobs.
- **Now:** stays `snoozed`. No query filters on `snoozed_until` and no schedule exists. Only a customer reply wakes it, and there is no snoozed view, so it can be found only by searching.
- **Should:** a job opens due conversations.
- **Ours:** `routes/console.go:29-35`, `app/actions/conversations/status.go`.
- **Theirs:** Chatwoot `app/jobs/conversations/reopen_snoozed_conversations_job.rb`; Zammad `app/models/ticket.rb:147`, `db/seeds/schedulers.rb:5`.
- **Evidence:** test, plus grep.
- **Fix:** per-minute job with `FOR UPDATE SKIP LOCKED`, through `Writer.ChangeStatus`; add the view.

#### F32. `reassign_to_member_id` is ignored and a deactivated member keeps conversations
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** high.
- **Input:** `PATCH /members/A {status: deactivated, reassign_to_member_id: B}`.
- **Now:** assignee stays A, only the audit log sees B. `unassigned.open=0`, so it reaches no queue. Setting a team returns 422 `member_inactive`. A reply does not take it. A customer reopen stays with A.
- **Should:** move to B, or unassign.
- **Ours:** `app/actions/members/member_changes.go:86-108`, `app/listeners/write_audit_log.go:162`, `app/actions/conversations/assignment.go` (`people`), `app/actions/messages/reply.go:187`.
- **Theirs:** Chatwoot `app/jobs/agents/destroy_job.rb:33`; Zammad `app/models/ticket.rb:769`, `spec/models/ticket_spec.rb:750`.
- **Evidence:** test.
- **Fix:** reassign open and snoozed conversations in `deactivate`, and allow an unchanged inactive holder in `people`.

#### F33. The Bridge drops the second copy of every event
- **Category:** realtime. **Type:** Incorrect. **Severity:** high.
- **Input:** one message event, same `evt_` id, on `conversation:` then `workspace:`.
- **Now:** conversation first: list row not patched, counts not refetched. Workspace first: message dropped, timeline stale, `recovered=true` so no resync. Workspace first is the likely order on reconnect recovery.
- **Should:** dedupe per channel.
- **Ours:** `resources/js/lib/realtime/bridge.ts:81`, `:91`; `app/broadcasting/broadcaster.go:200`.
- **Theirs:** Chatwoot `app/javascript/dashboard/helper/actionCable.js:43`, `:62`.
- **Evidence:** test against the real `bridge.ts`. Browser order not observed.
- **Fix:** key `seen` on channel and id.

#### F34. A teammate's list goes wrong on every row patch
- **Category:** realtime. **Type:** Incorrect. **Severity:** high.
- **Input:** customer message, close or spam broadcast for a cached row.
- **Now:** unread stays false (broadcasts omit it), the row does not move up, a closed or spam row stays in Open while counts disagree. `refetchOnWindowFocus: false` keeps this until remount.
- **Should:** reflect membership, order and per-reader unread.
- **Ours:** `resources/js/lib/realtime/bridge.ts:141`, `cache.ts` (`patchArray`), `app/http/resources/broadcast_payloads.go:40`, `resources/js/app.tsx:17`.
- **Theirs:** Chatwoot `actionCable.js:199-217`.
- **Evidence:** test.
- **Fix:** invalidate lists on row events; send per-member unread on the member channel.

#### F35. Failed delivery is invisible and cannot be retried
- **Category:** inbox logic. **Type:** Missed. **Severity:** high.
- **Input:** suppressed address or provider refusal.
- **Now:** the API has status and error, `toEntry` drops them, no retry route exists, and `record` dispatches no event. The teammate believes the customer got the reply.
- **Should:** a failed badge with the reason and a retry.
- **Ours:** `resources/js/features/inbox/conversations.ts:93`, `app/jobs/send_channel_message.go` (`record`).
- **Theirs:** Chatwoot `app/controllers/api/v1/accounts/conversations/messages_controller.rb:30`; Zammad `app/jobs/ticket_article_communicate_email_job.rb:148`.
- **Evidence:** read only.
- **Fix:** dispatch from `record`, show status, add a retry action.

#### F36. Contact identities split from `contacts.email`
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** high.
- **Input and result:** PATCH email a to new: mail from `new@` makes a separate contact, mail from `a@` lands on the old one, replies go to `new@`. Lead from inbound, then `POST /contacts {email, external_id}`: two contacts, mail keeps going to the lead. PATCH c's email to b's address: 200, two contacts. Clearing the email: mail still resolves, replies refused.
- **Should:** one person per address, or an explicit conflict.
- **Ours:** `app/actions/contacts/update_contact.go:137`, `create_contact.go:186`, `:246`; `database/queries/contact_directory.sql:37`.
- **Theirs:** Chatwoot `app/models/contact.rb:38`, `:51`; Zammad `app/models/user.rb:908`, `:986`.
- **Evidence:** test.
- **Fix:** sync identity rows in UpdateContact; return `ErrContactExists` on conflict.

#### F37. A blocked or spam-verdict sender still reopens existing threads
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** high.
- **Input:** block a contact, then mail on their closed thread.
- **Now:** `reopened=true status=open spam=false`, unread. A new thread is correctly spam. A provider spam verdict with a reply token behaves the same.
- **Should:** store quietly with no wake.
- **Ours:** `app/actions/conversations/receive_inbound_message.go:193` (`wake`).
- **Theirs:** Chatwoot `app/models/message.rb:415`, `app/models/conversation.rb:315`.
- **Evidence:** test.
- **Fix:** include `SpamVerdict` and `contact.BlockedAt` in `wake`.

#### F38. The composer defeats both reply guards
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** high.
- **Input:** a re-send after a lost response; two teammates answering at once.
- **Now:** `client_message_id` is new on each call, so a retry is a second email. `expected_last_message_id` is never sent, so the 409 path is unreachable from the UI. The echo of the first commit is dropped as own, so the sender does not see it.
- **Should:** a stable id per draft, send the newest non-event id, show the 409.
- **Ours:** `resources/js/lib/api/inbox.ts:178-194`, `conversation-view.tsx` (`useTimeline`), `composer.tsx:177`.
- **Theirs:** Chatwoot `echo_id`, `messages_controller.rb:30`; Zammad `app/models/taskbar.rb`.
- **Evidence:** read only.
- **Fix:** hold the id in draft state, refresh on failure. See F73 before wiring expected ids.

#### F39. The list stops at 50 rows and the thread at the newest 100
- **Category:** inbox logic. **Type:** Missed. **Severity:** high.
- **Input:** 300 open conversations, or a long thread.
- **Now:** the header says 300, 50 rows show, no paging. Older entries are unreachable. The API cursors are unused.
- **Should:** load more.
- **Ours:** `resources/js/lib/api/inbox.ts:120`, `:147`; `inbox-page.tsx`.
- **Theirs:** Chatwoot paginates `conversation_finder` and fetches earlier messages.
- **Evidence:** read only.
- **Fix:** `useInfiniteQuery`; `appendRecord` then targets the newest page.

#### F70. Clicking the verification mail hands a pre-registered account to the stranger
- **Category:** accounts. **Type:** Incorrect. **Severity:** high.
- **Input:** attacker registers `grace@x`, the real Grace clicks the verification mail.
- **Now:** verified, attacker's session stays valid, attacker's password works after her later magic link. It needs the victim's click, so it is high rather than critical.
- **Ours:** `app/actions/auth/verify_email.go:159`, `consume_magic_link.go:258`.
- **Theirs:** Chatwoot `app/models/user.rb:65`.
- **Evidence:** test.
- **Fix:** run the claim on the verify path.


### Medium

#### F40. Bytes that contradict the declared charset become replacement characters or mojibake
- **Category:** email parsing. **Type:** Incorrect. **Severity:** medium.
- **Input:** Latin-1 bytes with `charset=utf-8`, or with no charset: `caf� au lait`. UTF-8 bytes with `charset=iso-8859-1`: `cafÃ© au lait`. The parser warns of a mismatch and carries on.
- **Should:** if bytes are not valid in the declared charset, try UTF-8, then Windows-1252 or detection.
- **Ours:** `framework/mailbox/parse/text.go` (`clean`), `parse.go:141`.
- **Theirs:** Zammad `mail016` (declared GB2312, detected GB18030), `mail060`.
- **Evidence:** test (`out/b1.txt`).
- **Fix:** use the part's raw bytes and the parser's detection on invalid input.

#### F41. ISO-2022-JP without a declared charset shows escape codes
- **Category:** email parsing. **Type:** Missed. **Severity:** medium.
- **Input:** Outlook `Content-Type: text/plain;` with no charset, `\x1b$B…` body.
- **Now:** Markdown holds `\x1b$B$3$l$O%F…`, unreadable.
- **Should:** recognise the escape sequence.
- **Ours:** `framework/mailbox/parse/parse.go:141`.
- **Theirs:** Zammad `test/data/mail/mail116.box`.
- **Evidence:** test.
- **Fix:** detect `ESC $ B` when no charset is declared.

#### F42. Charset labels seen in real mail are unknown
- **Category:** email parsing. **Type:** Missed. **Severity:** medium.
- **Input:** `iso-gb2312`, `x-big5`, `cp950`, `ms932`, `windows-932`, `iso-2022-jp-2`, `x-windows-874`, `win-1252`, `unicode-1-1-utf-7`, `unknown-8bit`, `binary`, `iso-2022-kr`.
- **Now:** body bytes turn into `�`, unrecoverable in Markdown. A `utf-16` BOM stays as `\ufeff`.
- **Should:** alias them.
- **Ours:** `framework/mailbox/parse/parse.go:133`.
- **Theirs:** Zammad `mail060`, `mail018`.
- **Evidence:** test.
- **Fix:** register aliases with the parser, or decode parts through a small alias table.

#### F43. Plain-text Outlook quotes in many languages stay in the body
- **Category:** email parsing. **Type:** Missed. **Severity:** medium.
- **Input:** `Enviado el:`, Danish, Polish, Czech, Russian, Turkish, Finnish, Hungarian, Chinese, Japanese, Korean or Arabic From and Sent lines. Also `**Von:**&nbsp;Bob`, because `\s` misses NBSP.
- **Now:** the whole old thread stays visible, and feeds the agent.
- **Ours:** `app/integrations/email/quotes.go:51-52`.
- **Theirs:** Chatwoot `EmailReplyTrimmer` in `mail_presenter.rb`.
- **Evidence:** test.
- **Fix:** extend both lists, or key on the structure: a colon-labelled block of From, Date, To, Subject in order.

#### F44. Attribution lines in other languages are left behind
- **Category:** email parsing. **Type:** Missed. **Severity:** medium.
- **Input:** French with NBSP (`a écrit\u00a0:`, Gmail's real output), Swedish, Danish, Norwegian, Finnish, Polish, Czech, Hungarian, Turkish, Russian, Ukrainian, Arabic, Hebrew, Japanese, Korean, Indonesian, Romanian, Greek, or a header wrapped over three lines.
- **Now:** quoted lines are cut but the line `… skrev Bob <b@x.com>:` stays at the end of the message.
- **Ours:** `app/integrations/email/quotes.go:44`, `:131`.
- **Theirs:** Chatwoot `EmailReplyTrimmer`.
- **Evidence:** test.
- **Fix:** when a quote run is cut, also drop the line above it if it ends in `:` and names an address.

#### F45. Footers after a quote keep the whole thread visible
- **Category:** email parsing. **Type:** Incorrect. **Severity:** medium.
- **Input:** a reply, its quoted thread, then a gateway disclaimer or a signature below the quote.
- **Now:** any text after a quoted line counts as an inline answer, so nothing is cut.
- **Ours:** `app/integrations/email/quotes.go:160`.
- **Theirs:** Chatwoot's fixture `mail_with_quote.eml` has this shape; its expected result was not traced.
- **Evidence:** test. Fixture Markdown keeps `On Thu… wrote:` and `> Any update on this?`.
- **Fix:** ignore a trailing signature or disclaimer when deciding it is interleaved.

#### F46. Text hidden from people still reaches the agent
- **Category:** email parsing. **Type:** Incorrect. **Severity:** medium.
- **Input:** `color:transparent`, `font-size:1px;color:#fff`, `height:0;overflow:hidden`.
- **Now:** all survive into Markdown. The file's stated purpose is to drop instructions aimed at the agent.
- **Ours:** `app/integrations/email/htmlbody.go:54`.
- **Theirs:** no equivalent aim in either; Zammad `lib/html_sanitizer/scrubber` is the closest area, not traced. Surfaced while testing.
- **Evidence:** test. `md="z\n\nw\n\nv"`.
- **Fix:** add the zero height, tiny font and transparent colour patterns.

#### F47. Executable blocking misses trailing dots and spaces
- **Category:** email parsing. **Type:** Incorrect. **Severity:** medium.
- **Input:** `INVOICE.EXE ` and `invoice.exe.`, which Windows treats as `.exe`. Names like `../../etc/passwd` and `C:\temp\evil.pdf` are also stored as they arrive.
- **Now:** stored as allowed. The disk path is generated, so no traversal on disk, but the name reaches any later download header.
- **Ours:** `app/actions/email/inbound_message.go:265`, `:217`.
- **Theirs:** Chatwoot `mail_presenter.rb` `attachments` and Zammad's parser name files, but the exact trimming was not traced.
- **Evidence:** test.
- **Fix:** trim dots and spaces before the check, and store the base name only.

#### F48. A customer can suppress any address with a forged bounce
- **Category:** classification. **Type:** Incorrect. **Severity:** medium.
- **Input:** a customer has our Message-ID, and sends a `multipart/report` naming `Final-Recipient: ceo@victim.test`.
- **Now:** the address is suppressed for the workspace with no check that it was a recipient of that mail or that the report came from a mail system.
- **Should:** only recipients of the sent email; prefer a null envelope sender.
- **Ours:** `app/actions/email/delivery_reports.go:133`, `:168`; `app/actions/email/inspect.go:210`.
- **Theirs:** Zammad `app/models/channel/filter/bounce_delivery_permanent_failed.rb` reads recipients from the article.
- **Evidence:** test. `suppressed ceo@victim.test rows=1`.
- **Fix:** require the address to be in `ToAddresses` or `CcAddresses`.

#### F49. Plain text turns into emphasis, code, headings or nothing
- **Category:** Markdown/rendering. **Type:** Incorrect. **Severity:** medium.
- **Input and result:**

| Text | Renders as |
|---|---|
| `2*3*4 equals 24` | `2<em>3</em>4 equals 24` |
| `# 123 is my order number` | no hash |
| indented `Name: John` | code block |
| `Order summary` / `---` | line lost |
| `[1]: http://x.com` | nothing at all |
| ` ``` ` unclosed | everything after becomes code |
| `_foo_`, `__bar__` | emphasis |

- **Should:** literal text, as F26.
- **Ours:** `resources/js/features/inbox/message-body.tsx:55`, `app/integrations/email/htmlbody.go:123`.
- **Theirs:** Zammad text2html.
- **Evidence:** test.
- **Fix:** as F26.

#### F50. Non-standard bounces become conversations
- **Category:** classification. **Type:** Missed. **Severity:** medium.
- **Input:** from `mailer-daemon` or `postmaster`: `Message not delivered`, `Delivery has failed…`, `Nicht zustellbar`, `Non recapitabile`, `No se puede entregar`, `Não entregue`, `Zustellung fehlgeschlagen`, `Mail could not be delivered`. Or a report nested in `multipart/mixed`, or `X-Failed-Recipients`.
- **Now:** classified `noreply` or `normal`, opening a conversation. Non-standard ones that do match are ignored with nothing recorded on the reply, so it still reads as sent.
- **Ours:** `app/integrations/email/classifier.go:40`, `:187`; `framework/mailbox/internal/report/report.go:56`; `app/actions/email/delivery_reports.go:95`.
- **Theirs:** Chatwoot `mail_presenter.rb` `bounced?`; Zammad `bounce_follow_up_check.rb`, `mail033*`, `mail055`.
- **Evidence:** test.
- **Fix:** a system sender alone is enough; search nested parts; match the embedded original Message-ID.

#### F51. Policy bounces suppress a customer forever
- **Category:** classification. **Type:** Incorrect. **Severity:** medium.
- **Input:** `5.7.1` (our IP listed) or `5.2.2` (mailbox full).
- **Now:** permanent, scope all, no expiry, and no query or action lifts it. Every later reply fails `recipient_suppressed`.
- **Should:** suppress only for bad-address codes, with expiry and a way to lift.
- **Ours:** `framework/mailbox/dsn/dsn.go:52`, `app/actions/email/delivery_reports.go:145`, `database/queries/email_suppressions.sql:1`.
- **Theirs:** Zammad `bounce_delivery_permanent_failed.rb` filters by error status.
- **Evidence:** test for the parse, read for the rest.
- **Fix:** an allowlist of 5.1.x codes; give others an expiry.

#### F52. One complaint suppresses every recipient of that mail
- **Category:** classification. **Type:** Incorrect. **Severity:** medium.
- **Input:** a copied person marks a reply as spam.
- **Now:** the customer is suppressed too; `OriginalRcptTo` is parsed and unused.
- **Ours:** `app/actions/email/delivery_reports.go:70`.
- **Theirs:** no complaint handling found in the paths read; surfaced while testing.
- **Evidence:** read only.
- **Fix:** use `OriginalRcptTo` when present.

#### F53. Auto-replies without the headers we know count as a person
- **Category:** classification. **Type:** Missed. **Severity:** medium.
- **Input:** `X-Loop: yes`, `X-Autoresponse`, `X-Autoresponder`, `X-Mail-Autoreply`, `X-Vacation`, `X-Auto-Response-Suppress` with Exchange headers such as `Mailbox Rules Agent`. Subjects like `Autosvar:`, `Automatisk svar:`, `Автоответ:`, `AUTO:`, `Abwesend:`, `Vacation reply`, `AW: Abwesenheitsnotiz`, `Re: Out of office`.
- **Now:** `normal`, which wakes a closed or snoozed conversation and can be answered automatically. Sender rate limits are the only stop.
- **Ours:** `app/integrations/email/classifier.go:35`, `:126`.
- **Theirs:** Zammad `auto_response_check.rb`, `out_of_office_check.rb`.
- **Evidence:** test.
- **Fix:** widen the header list; let subjects mark a threaded mail as quiet rather than discard it.

#### F54. Reused Message-IDs lose the second sender's mail
- **Category:** threading. **Type:** Incorrect. **Severity:** medium.
- **Input:** two different senders, same `Message-ID` (buggy scripts reuse ids).
- **Now:** the second is ignored `duplicate`, workspace-wide.
- **Should:** dedupe on id plus sender.
- **Ours:** `app/integrations/email/threading.go:56`, `app/actions/conversations/receive_inbound_message.go:147`.
- **Theirs:** Chatwoot scopes to the conversation, `app/mailboxes/mailbox_helper.rb`.
- **Evidence:** test. `[same-id] ignore="duplicate" conversations=1`.
- **Fix:** include the sender in the key.

#### F55. Forwards from Apple Mail, Outlook and other languages are not recognised
- **Category:** classification. **Type:** Missed. **Severity:** medium.
- **Input:** a teammate forwards: Apple text twin (`> From:`), Outlook with no marker, Swedish or Portuguese markers.
- **Now:** `FindForward` returns false, so the mail is marked as a teammate's test, not the customer's conversation. Our header claims every common client.
- **Ours:** `app/integrations/email/forwarded.go:29`.
- **Theirs:** no direct equivalent; Chatwoot `forwarder_email.eml` covers forwarding by `X-Forwarded-For` only. Surfaced while testing.
- **Evidence:** test.
- **Fix:** strip `>` before reading, accept a block with no marker, add languages.

#### F56. Reply text twin drops link targets and list numbers
- **Category:** outbound. **Type:** Incorrect. **Severity:** medium.
- **Input:** `[click](https://x.com/a)`, `1. one`, `Use the <div> tag`.
- **Now:** text twin `click`, `one`, `Use the  tag`. HTML escapes `<div>`, but our thread hides it, so teammate and customer read different text.
- **Ours:** `framework/markdown/text.go`, `app/jobs/channel_outbound.go:85`, `message-body.tsx:55`.
- **Theirs:** Zammad `html2text` `link_style: :numbered`, `lib/core_ext/string.rb:102`.
- **Evidence:** test.
- **Fix:** a separate plain renderer for mail: `text (url)`, markers kept.

#### F57. Search breaks on Hindi, Thai, CJK substrings, accents and stop words
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** medium.
- **Input and result:**
  - `हिन्दी` compiles to `'ह' <-> 'न' <-> 'द'` and `สวัสดี` to `'สว' <-> 'สด'`: combining marks count as separators.
  - `日本語` misses `私は日本語を話します`, one long lexeme.
  - `muller` misses `Müller`; `rechnung` misses `Rechnungen` under the `english` config.
  - `the` or `can't` has zero nodes, and the query's `numnode = 0` branch then matches every conversation.
- **Ours:** `app/support/searchquery/searchquery.go:242`, `database/queries/conversation_lists.sql:45`.
- **Theirs:** Chatwoot `conversation_finder.rb:139` ILIKE; Zammad's search backend.
- **Evidence:** test.
- **Fix:** keep `unicode.Mn` and `Mc`, add `unaccent` and a substring fallback, and return nothing for stop-word-only searches.

#### F58. Inbound mail ignores the conversation lock, so a close in flight loses the message
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** medium.
- **Input:** close in an open transaction while mail arrives.
- **Now:** `status=closed message_count=2 reopened=false`, an unanswered customer message on a closed conversation.
- **Ours:** `app/actions/conversations/receive_inbound_message.go:315`.
- **Theirs:** none explicit.
- **Evidence:** test.
- **Fix:** `LockConversation` in `existing()`.

#### F59. An out-of-office on a long-closed thread opens a new conversation
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** medium.
- **Input:** auto-reply 40 days after close.
- **Now:** a new, open, unassigned, unread conversation. Bulk and noreply mail take the same path, by reading.
- **Ours:** `app/actions/email/inbound_message.go:117`, `ingest_email.go:156`.
- **Theirs:** Zammad `email_parser.rb:257`; Chatwoot `message.rb:226`.
- **Evidence:** test.
- **Fix:** when automatic and past the window, append quietly or ignore.

#### F60. A teammate's reply on an old closed conversation does not extend the window
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** medium.
- **Input:** closed 40 days ago, teammate replies, customer answers an hour later.
- **Now:** a new conversation with no assignee.
- **Ours:** `app/actions/conversations/receive_inbound_message.go:306`, `app/actions/messages/reply.go` (`then`).
- **Theirs:** Chatwoot `message.rb:435` reopens with no window; Zammad shares this behaviour.
- **Evidence:** test.
- **Fix:** `ClosedBefore` takes the later of close and last reply.

#### F61. Spam does not stick to the sender
- **Category:** inbox logic. **Type:** Missed. **Severity:** medium.
- **Input:** a spam thread past the window, then more mail; or a teammate marks spam and the sender starts anew.
- **Now:** `spam=false status=open`.
- **Ours:** `receive_inbound_message.go:265`, `:289`; `app/actions/conversations/attributes.go` (`setSpam`).
- **Theirs:** Chatwoot `mute!`, `spec/models/conversation_spec.rb:518-528`.
- **Evidence:** test.
- **Fix:** inherit from the found thread; decide whether marking spam blocks.

#### F62. A reused relay key silently loses a message
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** medium.
- **Input:** key `k1` on conversation A, then on B.
- **Now:** 200 with A's message and `meta.conversation=B`. B's body is dropped.
- **Ours:** `app/actions/messages/notes.go:159`, `session.go` (`replay`).
- **Theirs:** no equivalent confirmed.
- **Evidence:** test.
- **Fix:** include the conversation id in the key, as replies do.

#### F63. Out-of-order broadcasts overwrite newer state
- **Category:** realtime. **Type:** Missed. **Severity:** medium.
- **Input:** cached closed at 10:00:05, then a broadcast open at 10:00:01.
- **Now:** ends open. A patch during a fetch can also be overwritten.
- **Ours:** `resources/js/lib/realtime/cache.ts` (`patchArray`), `bridge.ts`.
- **Theirs:** Chatwoot `app/javascript/dashboard/store/modules/conversations/index.js:296`.
- **Evidence:** test.
- **Fix:** compare `updated_at` in `upsertRecord`.

#### F64. Phone normalisation corrupts numbers
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** medium.
- **Input and result:** `+1 (555) 123-4567 ext. 89` to `+1555123456789`; `1-800-FLOWERS` to `1800`; `call me 5` to `5`; `+44 (0)20 7946 0958` to `+4402079460958`; `00 44…` keeps `00`.
- **Ours:** `app/actions/contacts/fields.go:60`.
- **Theirs:** Chatwoot `app/models/contact.rb:54-56`.
- **Evidence:** test.
- **Fix:** validate E.164 and refuse otherwise.

#### F65. Magic link mails the typed spelling but creates the folded address
- **Category:** accounts. **Type:** Incorrect. **Severity:** medium.
- **Input:** `POST /magic-links` with `Kate@example.com` using the Kelvin sign.
- **Now:** mail goes to the raw look-alike, consuming it creates `kate@example.com`, verified.
- **Ours:** `app/actions/auth/send_magic_link.go:97`, `:146`; `app/actions/users/create_user.go:80`.
- **Theirs:** Zammad `app/models/user.rb:908`, Chatwoot `app/models/user.rb:126`.
- **Evidence:** test.
- **Fix:** one canonical form for mailing and for the account.

#### F66. Email has no Unicode or IDN normalisation
- **Category:** accounts. **Type:** Missed. **Severity:** medium.
- **Input:** each is a separate account: `josé@` NFC and NFD, Cyrillic `аdmin@`, zero-width `x​@`, fullwidth, `ſam@`, `dım@`, `a@bücher.de` and `a@xn--bcher-kva.de`.
- **Ours:** `app/actions/users/create_user.go:80`, `app/http/requests/store_user_request.go:30`.
- **Theirs:** Zammad `user.rb:908`, `lib/email_address_validation.rb` for IDN. Neither handles NFC or homoglyphs.
- **Evidence:** test.
- **Fix:** one `NormalizeEmail` helper everywhere; migrate existing rows.

#### F67. Invitation address check uses wider folding than the database
- **Category:** accounts. **Type:** Incorrect. **Severity:** medium.
- **Input:** invite `sam@example.com`, sign up with the token as `ſam@example.com`.
- **Now:** 201, `ſam@…` stored verified.
- **Ours:** `app/actions/auth/register_user.go:184`, `app/actions/invitations/accept_invitation.go:146`, `handle_oauth_callback.go:386`.
- **Theirs:** no equivalent.
- **Evidence:** test.
- **Fix:** exact compare after normalising.

#### F68. A correct password clears the 2FA streak, and wrong codes alert nobody
- **Category:** accounts. **Type:** Incorrect. **Severity:** medium.
- **Input:** attacker with the password: 5 wrong TOTP codes, wait 31s, sign in, 5 more.
- **Now:** lock stays 30s each round; wrong passwords grow to 300 and 1800. No event or mail.
- **Ours:** `app/actions/auth/attempt_login.go:325`, `complete_two_factor_challenge.go:143`, `:311`.
- **Theirs:** Chatwoot `config/initializers/rack_attack.rb:177`.
- **Evidence:** test.
- **Fix:** clear only after the second factor; log and warn.

#### F69. A reset does not lift the lock
- **Category:** accounts. **Type:** Missed. **Severity:** medium.
- **Input:** 5 failed passwords, reset by mail, correct code.
- **Now:** 429 `Retry-After: 30`.
- **Ours:** `app/actions/auth/reset_password.go:97-126`.
- **Theirs:** Zammad `app/models/user.rb:1217`.
- **Evidence:** test.
- **Fix:** clear streak and email locks after reset commits.

#### F71. Audit gaps around invitations and bulk token revokes
- **Category:** accounts. **Type:** Missed. **Severity:** medium.
- **Input:** invite as member, re-invite as admin.
- **Now:** one `invitation.created` from the first. Resends dispatch nothing. Deactivation and leaving revoke tokens with no `token.revoked`.
- **Ours:** `app/actions/invitations/create_invitations.go:240`, `change_invitation.go:98`, `app/actions/members/member_changes.go:100`, `app/actions/auth/claim_account.go:80`.
- **Theirs:** no equivalent.
- **Evidence:** test for the re-invite, read only for the rest.
- **Fix:** `invitation.resent` with role changes, and token counts on deactivation.

#### F91. A deactivated admin's pending invitations stay valid
- **Category:** accounts. **Type:** Missed. **Severity:** medium.
- **Input:** an admin invites `alt@` as admin, owner deactivates that admin, `alt@` accepts: joins as admin.
- **Ours:** `app/actions/members/member_changes.go:86`.
- **Theirs:** neither has invitations of this shape.
- **Evidence:** test.
- **Fix:** revoke them on deactivation, or flag them.


#### F89. Sign-up reveals taken addresses
- **Category:** accounts. **Type:** Missed. **Severity:** medium.
- **Input:** `POST /users` with an existing email: 422 `email_taken`, capped per IP only. Deliberate per the file header, so flagged as a choice; reset, magic link and login do not leak.
- **Ours:** `app/actions/users/create_user.go:90`, `register_user.go:157`.
- **Theirs:** Zammad `app/services/service/user/signup.rb:31`, `:62`.
- **Evidence:** test.
- **Fix:** needs a product decision, as it conflicts with instant sign-in.


### Low

#### F72. Encoded words that real clients get wrong stay raw
- **Category:** email parsing. **Type:** Missed. **Severity:** low.
- **Input:** `=?ISO-2022-JP?B?…==?=` with excess padding shows raw as a name. `=?utf-8?B?5L?= =?utf-8?B?2g5aW9?=`, split mid-base64, shows raw. Raw Shift_JIS in a name becomes `�c�[��`. `=?unicode-1-1-utf-7?…` stays raw.
- **Ours:** `framework/mailbox/parse/parse.go:183`.
- **Theirs:** Zammad `mail098`, `mail093`, `mail018`.
- **Evidence:** test.
- **Fix:** a tolerant retry on padding; join adjacent base64 words.

#### F73. The expected-id guard refuses with nothing to show
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** low.
- **Input:** expected id from another conversation, garbage, or an event row.
- **Now:** 409 `new_messages_since`, possibly empty. A composer sending the newest timeline entry fails whenever an event is newest.
- **Ours:** `app/actions/messages/reply.go` (`guard`).
- **Theirs:** none confirmed.
- **Evidence:** test.
- **Fix:** 422 for unknown ids; compare with the newest non-event entry.

#### F74. Unnamed parts are stored without a useful name, and body parts become attachments
- **Category:** email parsing. **Type:** Missed. **Severity:** low.
- **Input:** `message/rfc822`, `text/calendar`, image parts with no filename. MIME::Lite alternatives with `Content-Disposition: inline`.
- **Now:** `attachment-2` with no extension. A body's own text and HTML parts also appear as inline attachments (`mail013`, `mail026`, `mail093`).
- **Ours:** `app/actions/email/inbound_message.go:217`, `framework/mailbox/parse/parse.go:143`.
- **Theirs:** Zammad names `.eml`, `.ics` and `delivery-status.txt`; Chatwoot `attachments_without_filename.eml`.
- **Evidence:** test.
- **Fix:** derive extension from type; use the inner subject for `.eml`; skip parts already used as body.

#### F75. `x-uuencode` attachments are stored encoded
- **Category:** email parsing. **Type:** Missed. **Severity:** low.
- **Input:** `Content-Transfer-Encoding: x-uuencode`. The parser warns and keeps wrong bytes.
- **Ours:** `framework/mailbox/parse/parse.go:143`.
- **Theirs:** Zammad `mail078-content_transfer_encoding_x_uuencode.box`.
- **Evidence:** test.
- **Fix:** decode, or list as unreadable.

#### F76. `CET` and other zone names parse as UTC
- **Category:** email parsing. **Type:** Incorrect. **Severity:** low.
- **Input:** `Wed, 5 Sep 2018 10:00:00 CET` becomes `10:00 +0000`, an hour off.
- **Ours:** `framework/mailbox/parse/parse.go:196`, `app/actions/email/inbound_message.go:349`.
- **Theirs:** Chatwoot `mail_with_no_date.eml` covers a missing date; zone names not confirmed in either.
- **Evidence:** test.
- **Fix:** treat an unknown-offset abbreviation as unreadable, use arrival time.

#### F77. A list's `<!--THE END-->` marker reaches the customer in quotes
- **Category:** outbound. **Type:** Incorrect. **Severity:** low.
- **Input:** HTML with `<ul>` then `<ol>`.
- **Now:** the converter adds `<!--THE END-->`; the thread hides it, but the quoted text in our reply shows `> <!--THE END-->`. Quotes also carry raw Markdown syntax.
- **Ours:** `app/actions/email/compose_reply.go:224`, `app/integrations/email/composer.go:189`.
- **Theirs:** not confirmed in either; surfaced while testing.
- **Evidence:** test.
- **Fix:** quote `PlainText` of the body with comments removed.

#### F78. Small HTML leftovers
- **Category:** email parsing. **Type:** Incorrect. **Severity:** low.
- **Input:** `<img style="width:1px;height:1px">` and `width="1px"` survive as images. `<hr>` before `divRplyFwdMsg` leaves `* * *` at the end. `tel:` links lose their target in both the sanitizer and the renderer, leaving `href=""`.
- **Ours:** `app/integrations/email/htmlbody.go:185`, `:259`, `:276`; `message-body.tsx:18`.
- **Theirs:** Zammad `lib/html_sanitizer/scrubber` is the closest area, not traced.
- **Evidence:** test.
- **Fix:** read pixel size from style, drop a trailing rule, allow `tel:`.

#### F79. Subject prefixes in several languages are treated as the title
- **Category:** threading. **Type:** Missed. **Severity:** low.
- **Input:** `回覆：`, `轉寄:`, `ответ:`, `رد:`, `תגובה:`, `Απ:`, `Σχετ:`, `답장:`, `RV:` (Outlook in Spanish), `Antwort:`, `Vá:`, `Ref:`, `I:`, `[Ticket#123] Re:`, `*** SPAM *** Re:`.
- **Now:** the subject heuristic misses, and outbound reads `Re: RV: Help`.
- **Ours:** `app/integrations/email/subject.go:37`.
- **Theirs:** Zammad `follow_up_check.rb` strips any short prefix; Chatwoot threads on references.
- **Evidence:** test.
- **Fix:** add these; consider a generic short-word-and-colon rule for the normalized compare.

#### F80. Noreply variants and `X-Autoreply: no`
- **Category:** classification. **Type:** Incorrect. **Severity:** low.
- **Input:** `no_reply@`, `no.reply@`, `do_not_reply@`, `x-noreply@`, `info-noreply@`, `postmaster@` read as a person. `X-Autoreply: no` and `X-Autorespond: 0` read as automatic.
- **Ours:** `app/integrations/email/classifier.go:38`, `:131`.
- **Theirs:** Chatwoot `mail_presenter.rb` `x_auto_reply?` checks the value.
- **Evidence:** test.
- **Fix:** allow `_` and `.` separators, and check the value.

#### F81. RTL messages have no text direction
- **Category:** Markdown/rendering. **Type:** Missed. **Severity:** low.
- **Input:** an Arabic message. The body has no `dir`, so punctuation lands on the wrong side. Project rules ask for `dir="auto"`.
- **Ours:** `resources/js/features/inbox/message-body.tsx:54`.
- **Theirs:** not confirmed in either; project rule rather than a peer behaviour.
- **Evidence:** test for missing attribute; browser display not viewed.
- **Fix:** `dir="auto"` on each block.

#### F82. Rendered tables have no style or overflow container
- **Category:** Markdown/rendering. **Type:** Missed. **Severity:** low.
- **Input:** `a | b` / `--|--` / `1 | 2`, which also turns customer text into a table.
- **Now:** a bare `<table>` with no component, so a wide one can overflow the thread.
- **Ours:** `resources/js/features/inbox/message-body.tsx:31`.
- **Theirs:** not confirmed in either.
- **Evidence:** test for the output; overflow not viewed in a browser.
- **Fix:** add a table component in a scrolling wrapper.

#### F83. Automatic messages mark conversations unread
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** low.
- **Input:** an out-of-office on an open thread. `unread=true`, `last_message_at` moves.
- **Ours:** `database/queries/conversation_lists.sql` (unread EXISTS), `app/actions/conversations/append.go`.
- **Theirs:** Chatwoot `automation_rule_listener.rb:107`.
- **Evidence:** test.
- **Fix:** `AND m.substantive` for non-note messages.

#### F84. Relay reopens a spam conversation
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** low.
- **Input:** relay on spam gives `status=open spam=true`, unlike inbound.
- **Ours:** `app/actions/messages/notes.go` (`relay`).
- **Theirs:** Chatwoot `message.rb:415`.
- **Evidence:** test.
- **Fix:** same rule as inbound.

#### F85. `#number` search hides spam rows
- **Category:** inbox logic. **Type:** Incorrect. **Severity:** low.
- **Input:** `#1042` when it is spam. The filter comment says "whatever its status", yet `c.spam=false` applies.
- **Ours:** `app/actions/conversations/filters.go` (`listParams`).
- **Theirs:** none confirmed.
- **Evidence:** test.
- **Fix:** skip the spam filter for a number jump.

#### F86. A new member sees every open conversation as unread
- **Category:** inbox logic. **Type:** Missed. **Severity:** low.
- **Input:** new hire: `unassigned.unread=3/3`.
- **Ours:** `database/queries/conversation_lists.sql`.
- **Theirs:** Chatwoot has the same null-seen behaviour per conversation, `conversation.rb:199`.
- **Evidence:** test.
- **Fix:** default the marker to the join time.

#### F87. Timeline event pills are not live
- **Category:** realtime. **Type:** Missed. **Severity:** low.
- **Input:** a teammate closes a conversation another is viewing. The status changes, the "X closed" pill appears only after a refetch.
- **Ours:** `app/actions/conversations/writer.go` (`Record`).
- **Theirs:** not traced; Chatwoot `app/listeners/action_cable_listener.rb` is the broadcast path to compare.
- **Evidence:** read only.
- **Fix:** broadcast event rows.

#### F88. Teammates are never told about each other's typing
- **Category:** realtime. **Type:** Missed. **Severity:** low.
- **Input:** two teammates in a conversation. The code comment says teammates are told, but `record_typing.go:120` relays only to widget channels, so email conversations POST every 4 seconds for nothing.
- **Ours:** `resources/js/features/inbox/use-typing-report.ts:1-8`, `app/actions/conversations/record_typing.go:120`.
- **Theirs:** Chatwoot `app/listeners/action_cable_listener.rb:103`.
- **Evidence:** read only.
- **Fix:** broadcast on the conversation channel, or remove the report.

#### F90. A revoked invitation still creates an account through a live magic link
- **Category:** accounts. **Type:** Incorrect. **Severity:** low.
- **Input:** invite-only install, invitee requests a link, invitation revoked, link consumed: 201.
- **Ours:** `app/actions/auth/consume_magic_link.go:286`.
- **Theirs:** Chatwoot's signup check in `omniauth_callbacks_controller.rb`.
- **Evidence:** test.
- **Fix:** re-check admission in `signUp`.

#### F92. A `*` token gains power on promotion
- **Category:** accounts. **Type:** Missed. **Severity:** low.
- **Input:** member mints `*`, is promoted: the same token passes `PATCH /workspace`.
- **Ours:** `app/actions/tokens/create_personal_access_token.go:139`, `app/auth/authz/can.go:45`.
- **Theirs:** Zammad `spec/models/token_spec.rb:110`.
- **Evidence:** test.
- **Fix:** expand `*` at mint.

#### F93. Later second-factor changes are silent, and passkey clones are not reported
- **Category:** accounts. **Type:** Missed. **Severity:** low.
- **Input:** add a passkey, remove a non-last factor, regenerate codes: security log only. A non-advancing passkey counter reads as a generic wrong answer.
- **Ours:** `app/actions/factors/activate_auth_factor.go:217`, `remove_auth_factor.go:102`, `regenerate_recovery_codes.go:79`; `framework/auth/webauthn/ceremony.go:146`.
- **Theirs:** neither mails; Zammad stores the count, `lib/auth/two_factor/authentication_method/security_keys.rb:63`.
- **Evidence:** read only.
- **Fix:** one `FactorsChangedMail`; a distinct clone event.

#### F94. Pasted codes with stray characters count as wrong guesses
- **Category:** accounts. **Type:** Missed. **Severity:** low.
- **Input:** recovery code plus newline or tab, en dash, TOTP `123-456`: 422 and counted.
- **Ours:** `framework/auth/twofactor/recovery.go:186`, `totp.go:139`.
- **Theirs:** both are stricter still, `chatwoot/app/services/mfa/management_service.rb:58`.
- **Evidence:** test.
- **Fix:** strip whitespace and dash variants.

#### F95. Re-invite skips the per-invitation send cap
- **Category:** accounts. **Type:** Incorrect. **Severity:** low.
- **Input:** `POST /invitations` for one address 6 times: each mails. `ResendInvitation` caps at 3 a day.
- **Ours:** `app/actions/invitations/create_invitations.go:240`, `change_invitation.go:92`.
- **Theirs:** none.
- **Evidence:** test.
- **Fix:** share the cap.

#### F96. A reset can set the password to the person's own email
- **Category:** accounts. **Type:** Missed. **Severity:** low.
- **Input:** password equal to the email at reset. Sign-up refuses it.
- **Ours:** `app/http/requests/mailed_secret_requests.go:39`, `store_user_request.go:47`.
- **Theirs:** Zammad `signup.rb:29` at sign-up; reset path not traced.
- **Evidence:** read only.
- **Fix:** check in `ResetPassword.Handle`.

## 4. Checked and fine

**Email**

- Attachment filenames: RFC 2231, continuations and encoded words decode.
- Base64 with bad padding and quoted-printable soft breaks.
- Most charsets: GBK, GB18030, Big5, Shift_JIS, ISO-2022-JP declared, EUC-JP, EUC-KR, `ks_c_5601-1987`, KOI8, Windows and ISO-8859 sets, `us-ascii` with 8-bit.
- RFC 2047 split mid-character across UTF-8 Q words.
- `<>`, `John (Sales) Doe`, `hans@x (Hans)`, IDN and non-ASCII addresses.
- Multipart: empty text with HTML, signed mail, winmail.dat kept, calendars kept, `message/rfc822` kept, zero byte files.
- Gmail, Apple, Thunderbird, Yahoo, Outlook web and desktop (including the older blue rule) quotes in English.
- HTML escapes user Markdown correctly.
- A mail wholly in a quote falls back to everything.
- Threading: In-Reply-To, References, reverse references, token, tenant isolation.
- Loops: own stamp and outbound ids, sender throttling and quarantine stop two helpdesks answering each other. `Auto-Submitted` and `X-Auto-Response-Suppress` on automated mail.
- DSN `Permanent` for `4.x.x` and delays.
- Outbound: header injection refused (`invalid header`, line break in subject), `DisplayName` strips CRLF and quotes, long and non-ASCII subjects encode and fold, non-ASCII names encode, comma names encode, Reply-To carries the token.
- Own address never copied. Suppressed customer fails, suppressed copy is dropped.

**Markdown**

- `javascript:`, `data:`, `vbscript:` links stripped in thread and outbound.
- Autolinks with underscores.
- Emails and URLs with `_`, intraword asterisks in URLs.
- Long unbroken text wraps. Code fences scroll.
- Goldmark escapes raw HTML outbound.

**Inbox**

- tsquery injection impossible: only quoted letters and digits, bound parameter. `&|!():*`, `'`, `\`, empty and over-long queries are safe.
- Reopen within the window, wake on customer reply, auto-reply does not reopen.
- Snooze limits and preset timezones across DST. Assign to inactive member or deleted team refused.
- Team deletion unassigns. Own messages and events do not count unread. Closed and spam excluded from counts.
- Cursors stable with null `last_message_at`, ties and sort change.
- Mixed-case addresses on inbound. Reply and note retries with the same id replay.

**Realtime**

- Private notes stay on member-guarded channels. Tenant isolation by public id and membership.
- Channel auth re-runs on reconnect. Deactivation disconnects across nodes. Recovery failure resyncs.

**Accounts**

- Reset, magic link and login do not enumerate.
- Expired, revoked and reused invitation tokens refused. Re-inviting an active member is reported.
- Demoted tokens shrink at once. Tokens bound to their workspace.
- Last owner protected with locks. Recovery codes single use, sealed. TOTP replay blocked, Valkey failure closed.
- Unverified OAuth email refused. Trusted proxies only for client IP.

## 5. Not checked

- **Real browser rendering** of the thread: findings come from `renderToStaticMarkup` with our plugin set and our `ol` and heading components. Reconnect event order was proved at Bridge level only.
- **Real SMTP with Unicode addresses**, real passkey ceremonies, real provider webhooks (SES was out of scope; the test inbound source was used).
- **Huge HTML** performance and memory beyond the size caps as read; the oversized `walk` path was read, not run.
- **Attachment scanning**: rows start `pending`, and nothing reads them because no download exists.
- **`-tags cloud`** for account findings: read only. Verification is required by default there, which blocks tokens and invites for unverified accounts, but not F09 or F10 (linking and enrolment have no `verified` middleware).
- **Password change, email change, account deletion, message deletion, contact merge**: no action code found, so skipped.
- **Non-default database collations** for citext folding (F65 to F67 depend on it).
- **Their git history**: clones are shallow, so bug-fix commits were not mined. Tests, fixtures and code were.
- **Chatwoot and Zammad line numbers** are for the commits above.
