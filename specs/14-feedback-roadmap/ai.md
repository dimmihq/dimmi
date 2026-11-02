# 14 AI: duplicates, request capture (BYOK)

Style: caveman. Owner: 14-feedback-roadmap. Read with README.md and `07-dimmi-agent/README.md`.

Rules (all features):
- Optional. Without valid 07 provider key or with 07 spend cap reached: features silently degrade (duplicates fall back to FTS + trigram candidates without judge; capture unavailable: 409 `ai_unavailable` on manual call, skipped job otherwise). Human flows never blocked.
- AI never writes customer-visible or vote data directly. Output = `feedback_suggestions` rows. Member accepts. Per-post AI summary: later: feedback-ai-summary.
- All calls through 07 `ai.RunTask` (README required edit 36): workspace key, classifier model by default (cheap), retries, timeouts, redaction, `ai_runs` row `kind = feedback` with purpose in `config_snapshot`, spend recorded and capped. Tests use 07 fake LLM; never real calls.
- Customer text (conversation messages, post bodies, quotes) wrapped with 07 `ai.WrapUntrusted` (untrusted data blocks). Model output parsed against JSON schema; invalid JSON: one retry with repair hint, then run `failed`, no suggestion.
- Prompts versioned in `resources/prompts/feedback/<date>/`; run records version + hashes.

## 1. Candidate retrieval (no AI, always on)

`SearchSimilarPosts(workspace, text, opts)` used by similar-as-you-type, duplicate detection, capture matching:
1. FTS: `search @@ websearch_to_tsquery('dimmi_search', $text)` ranked `ts_rank_cd`, top 20.
2. Trigram: `title % $title` with `similarity(title, $title)`, top 20.
3. Vector (only when `feedback_post_embeddings` populated for workspace): cosine top 20 against query embedding from 08 `knowledge.EmbedTexts` (cached per normalized text 10 min, workspace-scoped, Valkey `cache.Store` per 08 query embedding cache).
4. Fuse with RRF (k = 60), drop merged/trashed/rejected/spam, scope filter (public only for portal/widget; members see all), limit.
Similar-as-you-type uses steps 1 and 2 only (no provider call on keystrokes).

## 2. Duplicate detection (new posts)

Trigger: `feedback_post.created` (any source except import) when `ai.duplicate_detection` true and AI ready. Job `detect_feedback_duplicates` (queue `ai`).
1. Candidates: `SearchSimilarPosts` top `feedback.ai.duplicate_candidates` (10) excluding self, same workspace, any board.
2. No candidates: done (no LLM call).
3. `RunTask{Purpose: duplicate}` with prompt `judge_duplicates.md`: input new post (title, body 2,000 chars), candidates (number, title, body 500 chars, status, votes). Output schema: `{"duplicates": [{"number": int, "confidence": 0..1, "reason": string<=200}]}`.
4. Each with confidence >= `feedback.ai.duplicate_min_confidence` (0.75): `feedback_suggestions` kind `merge_posts`, `post_id` = older or higher-voted post (target), `source_post_id` = other. Max 3 suggestions per new post. Pending duplicate pair exists (either direction): skip.
5. Event `feedback_suggestion.created`; dashboard shows "Possible duplicate of #12" on post and in triage queue.
6. Accept = `MergePosts(target, [source])` with `notify` default true; reviewer may swap direction (`overrides.direction: "reverse"`).

Portal/widget authors also see similar posts before submit (step 1 retrieval), so most duplicates never get created.

## 3. Request capture from conversations

Enable: `feedback_settings.ai.capture = suggest`.

Triggers:
- `conversation.closed` (03): job delayed `capture_delay` (10 min; reopen within delay cancels by job check `status still closed`). Skip when: spam, `is_test`, no customer `type = message` rows, contact blocked, already captured for same `last_message_id`, workspace daily cap `capture_daily_max` (200) reached (skip with metric; dashboard shows "capture paused for today").
- 07 classifier intent `feedback` on customer message: 07 calls `feedback.QueueCapture(conversation_id, message_id)` after commit; job runs after 2 min debounce (unique by conversation, coalesces with close trigger via 1 h period; manual run bypasses).
- Manual: `POST /feedback-captures {conversation_id}` (202), same job, bypasses period and daily cap (counts toward it).

Steps:
1. Load last `capture_max_messages` (40) messages type `message` (customer + team; no notes, no events), each truncated 1,500 chars, oldest first, with author role. Attachments ignored.
2. Pass A `RunTask{Purpose: capture}` prompt `extract_requests.md`: find product feedback (feature requests, missing capability, improvement asks; bug reports only when phrased as product gap) said by customer. Output schema `{"requests": [{"summary": string<=150, "quote": string<=500 (verbatim customer words), "message_index": int, "kind": "feature"|"improvement"|"bug", "priority_hint": "low"|"medium"|"high"|null}]}`, max 5. Quote must be verbatim substring of referenced customer message (verified in Go, normalized whitespace; else request dropped, flag `quote_not_verbatim`).
3. Each request: `SearchSimilarPosts(summary + quote)` top 8 (members scope: includes private boards).
4. Pass B `RunTask{Purpose: duplicate}` with `judge_duplicates.md` variant: does request match any candidate? Output `{"match": number|null, "confidence": 0..1, "proposed_title": string<=150, "proposed_body": string<=1000}`.
5. Match with confidence >= `capture_min_confidence` (0.7): suggestion `link_request` (post, conversation, message id from index, contact, quote, confidence). Else: suggestion `new_post` (proposed title/body, `proposed_board_id` = default board). Conversation already linked to that post (request exists): skip. Pending suggestion same conversation + post: skip.
6. Events `feedback_suggestion.created`; 06 not notified per suggestion (triage queue count badge via `GET /feedback-suggestions?filter[status]=pending` meta count).

Cost guard: two cheap classifier calls per conversation with requests (pass B only when pass A finds something; usually zero). Typical closed conversation: 1 call, about 3k input tokens.

## 4. Accepting suggestions

`PATCH /feedback-suggestions/{fbsg} {status: "accepted", overrides?}` (member, `feedback:write`). One tx:

| Kind | Effect | Overrides |
|---|---|---|
| link_request | `LinkRequest` (post, conversation, message, quote, contact, `add_vote` true, `subscribe` true, source `ai_capture`, `suggestion_id`) | `post_id` (link to other post), `quote`, `priority`, `add_vote`, `subscribe` |
| new_post | `CreatePost` (board, title, body, author = contact on behalf, `created_by_member_id` reviewer, source `ai_capture`, moderation approved) + request + vote | `title`, `body`, `board_id`, `category_id`, `status_id`, `post_id` (instead link to existing: becomes link_request) |
| merge_posts | `MergePosts` | `direction: "reverse"`, `notify` |

- Target post trashed/merged since suggestion: merged follows `merged_into_id`; trashed 409 `post_trashed` (reviewer picks other via override).
- Conversation erased or deleted: suggestion `expired` by eraser; accept 409 `suggestion_not_pending`.
- Dismiss: `status dismissed`; same (conversation, post) never re-suggested (unique check includes dismissed rows for 90 days).
- Expire after 30 days pending (job).
- MCP: `dimmi_list_feedback_suggestions` read only; accepting `link_request` possible via `dimmi_link_feedback_request` with same data (agent decision, no gate, internal).

## 5. Embeddings (optional)

- When 08 knowledge embedding configured (`knowledge.EmbedTexts` available): `embed_feedback_post` job on create and on title/body change (debounced 60 s). Text = title + first 2,000 chars body. `model_key` change (08 re-embed): `dimmi feedback:reembed` or automatic on `knowledge.embeddings_switched` event (job per workspace, chunked 100).
- Not configured: vector arm skipped; FTS + trigram still good for near-duplicates.

## 6. Prompts (files)

| File | Purpose | Notes |
|---|---|---|
| `extract_requests.md` | pass A | lists what counts as request, never invent, quote verbatim, ignore team suggestions, ignore support how-to questions |
| `judge_duplicates.md` | duplicates + pass B | same underlying user need = duplicate; different scope = not; output JSON only |

Open source in repo like 07 prompts. Prompt tests: golden fixtures with fake LLM verifying schema handling, verbatim check, thresholds.
