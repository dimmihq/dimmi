# Search

Style: caveman. Owned by 03. Postgres FTS + pg_trgm only (`research/tech-stack.md` row Search). Vector search over conversations post-MVP.

## Surfaces

| Endpoint | Use |
|---|---|
| `GET /api/v1/search?q=` | Cmd+K: top 5 conversations, 5 contacts, 3 companies in one call |
| `GET /api/v1/conversations?q=` | full conversation search with all list filters, cursor pagination |
| `GET /api/v1/contacts?q=` | contact search |
| `GET /api/v1/companies?q=` | company search |

## Query language (`app/support/searchquery`)

| Syntax | Meaning |
|---|---|
| `refund stripe` | all words (AND), stemmed, accent-insensitive |
| `"card declined"` | phrase (`<->` chain) |
| `refund OR chargeback` | OR (uppercase only) |
| `-invoice` | exclude |
| last bare word | prefix match (`:*`) for as-you-type, only when 2+ chars |
| `#1042` or bare `1042` alone | conversation number jump (exact) |
| `conv_…`, `ctc_…`, `msg_…` | id jump |
| `from:bob@x.com` / `from:bob` | message author contact email exact / contact name trigram |
| `is:open` `is:closed` `is:snoozed` `is:spam` `is:unread` `is:priority` `is:unassigned` `is:mine` | filters |
| `assignee:me` `assignee:ada@team.dev` `team:billing` | filters |
| `tag:billing` | filter (tag name, case-insensitive) |
| `inbox:support` `channel:email` | filters |
| `before:2025-01-01` `after:2024-06-01` | conversation `created_at` range, workspace timezone |
| `in:notes` `in:replies` `in:customer` | restrict message type/author |
| `imported:4512` `imported:https://app.intercom.com/...` | imported conversation by source external id, source ticket number or source URL (16; `source.imported.external_id` / `external_number` / `external_url` exact). `is:imported` filter |

Rules:
- Unknown operator `foo:bar` treated as plain text (no 400).
- Operators combine with `filter[...]` params (AND).
- Query length max 256 chars (422 `search_query_too_long`). Max 16 terms; extra ignored.
- Stop-word-only query (`the`): no text predicate, returns filtered list sorted by recency (never error).
- Lexemes passed as bind parameters into `to_tsquery('dimmi_search', $n)` built from quoted lexemes; no string-built SQL with user text.
- Unicode NFC normalize, `unaccent` via text search config.

## Index and text config

- `dimmi_search` text search configuration: copy of `english`, mapping `word, hword, hword_part` with `unaccent, english_stem`.
- `messages.search = to_tsvector('dimmi_search', coalesce(body,''))` generated stored. GIN `(workspace_id, search)` via `btree_gin`.
- `conversations.search` on subject; GIN `(workspace_id, search)`. Trigram GIN on `dimmi_unaccent(lower(subject))`.
- `contacts.search` (foundation: name, email, phone, external_id) GIN `(workspace_id, search)`; trigram on name and email.
- Event messages (`type = event`) have null body, not indexed in practice. Deleted messages have cleared body, drop out automatically.

## Conversation search SQL shape

Phase 1, candidates (newest N matching messages + subject hits):
```sql
-- name: SearchConversationCandidates :many
WITH msg_hits AS (
  SELECT m.conversation_id, m.id AS message_id, m.created_at
  FROM messages m
  WHERE m.workspace_id = @workspace_id
    AND m.search @@ @tsquery::tsquery
    AND m.deleted_at IS NULL
    AND (@types::text[] IS NULL OR m.type = ANY(@types))
    AND (@author_types::text[] IS NULL OR m.author_type = ANY(@author_types))
  ORDER BY m.created_at DESC
  LIMIT @rank_candidates            -- 2000
), subj_hits AS (
  SELECT c.id AS conversation_id, NULL::uuid AS message_id, c.last_message_at AS created_at
  FROM conversations c
  WHERE c.workspace_id = @workspace_id AND c.search @@ @tsquery::tsquery
  ORDER BY c.last_message_at DESC
  LIMIT 500
)
SELECT conversation_id, max(created_at) AS hit_at, (array_agg(message_id ORDER BY created_at DESC))[1] AS best_message_id, count(*) AS hits
FROM (SELECT * FROM msg_hits UNION ALL SELECT * FROM subj_hits) h
GROUP BY conversation_id;
```
Phase 2: join candidate conversation ids with conversations + `conversations.VisibleScope(principal)` + list filters (status, assignee, tag, ...) built by `querybuilder`, order by sort, keyset paginate. Visibility always applied in phase 2, never trusted from phase 1 (MVP scope = workspace; later inbox/team restriction drops invisible candidates here). Number/id jump and typo fallback go through same scope (outside scope = no result, never 403).

Phase 3: highlight only page rows (max 25): `ts_headline('dimmi_search', body, tsquery, 'MaxFragments=2, MaxWords=18, MinWords=6, StartSel=<mark>, StopSel=</mark>')` on `best_message_id`. Output HTML-escaped first, then marks inserted (headline run on escaped text), so only `<mark>` tags survive.

Sorts:
- `sort=relevance` (default when `q` present): score = `ts_rank_cd(search, tsquery)` of best message × recency decay `1 / (1 + age_days/90)` + 0.5 if subject hit. Computed over candidate set only. Cursor encodes score + id.
- `sort=-last_message_at` and every list sort allowed.

Bounded cost: candidate cap 2000 messages means very common term ("payment" in 50k messages) only searches newest 2000 matching messages. Response `meta.search.capped = true` and UI shows "Showing newest matches; add words to narrow". Rare terms (typical for finding old tickets) never hit cap, so 3-year-old conversation found.

Typo fallback: when phase 1 returns 0 rows and query has no operators-only, run trigram on subject and on contact name/email of conversations (`word_similarity(q, dimmi_unaccent(lower(subject))) > 0.4` using `%>` operator, index-backed), limit 25, `meta.search.fuzzy = true`.

`statement_timeout` 2 s via `SET LOCAL`; timeout returns 503 `search_timeout` with `Retry-After: 1` (never 500).

## Contact search
- `q` with `@`: exact email first (`(workspace_id, email)` index), then trigram on email.
- Else: FTS on `search` with prefix last term UNION trigram on name (`%>`), rank: exact external_id/email = 3, FTS rank, trigram similarity; limit 25; excludes `merged_into_id IS NOT NULL` and deleted.
- Phone-like query (digits, `+`): normalized to E.164 digits, prefix match on `phone`.

## Cmd+K `/search`
Runs conversation (limit 5), contact (5), company (3) searches concurrently (3 pooled connections, each own `statement_timeout` 1 s). One failing section returns `[]` plus `meta.errors.<section> = "timeout"`, never whole failure. Number/id jump returns single exact result in `data.jump`.

## Budgets
- Selective query (under 2,000 matches): p95 under 50 ms at 1M messages.
- Any query: p95 under 150 ms (cap guarantees bounded heap fetch).
- Cmd+K: p95 under 80 ms.
Bench queries in M11: rare word (1 hit), medium (500 hits), common (50k hits), phrase, prefix, fuzzy fallback, with and without filters.
