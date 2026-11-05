# Search

Style: caveman. Owned by 03. Postgres FTS + pg_trgm only (`research/tech-stack.md` row Search). Vector search over conversations stays out; FTS only. Builds on F29 (emails, ids, URLs in message text), F30 (bare number also searches text), F57 (non-Latin, accents, stop words): assume those parser and index fixes landed.

## Surfaces

| Endpoint | Use |
|---|---|
| `GET /api/v1/search?q=` | Cmd+K: top 5 conversations, 5 contacts, 3 companies in one call |
| `GET /api/v1/conversations?q=` | full conversation search with all list filters, cursor pagination |
| `GET /api/v1/contacts?q=` | contact search |
| `GET /api/v1/companies?q=` | company search |
| `POST /api/v1/conversation-searches` | structured search (P-04) |

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
| `is:open` `is:closed` `is:snoozed` `is:spam` `is:unread` `is:priority` (any level above none) `is:unassigned` `is:mine` `is:waiting` | filters |
| `priority:high` `priority:>=medium` | level |
| `contact:bob` `company:acme` | contact or company name trigram, domain, email |
| `title:invoice` | subject only |
| `number:>1000` `number:<2000` | conversation number, numeric ops |
| `started:>2025-01-01` `started:<2025-06-01` | alias of `after:`, `before:` |
| `attr.plan:pro` | defined conversation attribute (track 12) |
| `any:` prefix (`any: tag:vip priority:urgent`) | operators ORed instead of ANDed (match any) |
| `assignee:me` `assignee:ada@team.dev` `team:billing` | filters |
| `tag:billing` | filter (tag name, case-insensitive) |
| `inbox:support` `channel:email` | filters |
| `before:2025-01-01` `after:2024-06-01` | conversation `created_at` range, workspace timezone |
| `in:notes` `in:replies` `in:customer` | restrict message type/author |
| `imported:4512` `imported:https://app.intercom.com/...` | imported conversation by source external id, source ticket number or source URL (16; `source.imported.external_id` / `external_number` / `external_url` exact). `is:imported` filter |

Rules:
- I-100: parser `searchquery.Query` gains `From`, `Assignee`, `Team`, `Tag`, `Inbox`, `Channel`, `Before`, `After`, `In`, `Priority`, `Contact`, `Company`, `Title`, `NumberOp`, `Any`. Today only words, phrases, `-word`, `OR`, `is:` exist; rest build here. Resolved names to ids in one lookup per operator, unknown name = no results for that clause, never 400.
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
Phase 2: join candidate conversation ids with conversations + `conversations.VisibleScope(membership)` + list filters (status, assignee, tag, ...) built by `querybuilder`, order by sort, keyset paginate. Visibility always applied in phase 2, never trusted from phase 1 (restricted members' invisible candidates drop here). Number/id jump and typo fallback go through same scope (outside scope = no result, never 403).

Phase 3: highlight only page rows (max 25): `ts_headline('dimmi_search', body, tsquery, 'MaxFragments=2, MaxWords=18, MinWords=6, StartSel=<mark>, StopSel=</mark>')` on `best_message_id`. Output HTML-escaped first, then marks inserted (headline run on escaped text), so only `<mark>` tags survive.

## Sorts

I-102. Today code sorts newest only; relevance below is built. Test: rare phrase in old conversation beats common word in new one on first page.

- `sort=relevance` (default when `q` present): score = `ts_rank_cd(search, tsquery)` of best message × recency decay `1 / (1 + age_days/90)` + 0.5 if subject hit. Computed over candidate set only. Cursor encodes score + id.
- `sort=-last_message_at` (newest), `sort=last_message_at` (oldest) and every list sort allowed. UI control: Best match, Newest, Oldest.

Bounded cost: candidate cap 2000 messages means very common term ("payment" in 50k messages) only searches newest 2000 matching messages. Response `meta.search.capped = true` and UI shows "Showing newest matches; add words to narrow". Rare terms (typical for finding old tickets) never hit cap, so 3-year-old conversation found.

Typo fallback: when phase 1 returns 0 rows and query has no operators-only, run trigram on subject and on contact name/email of conversations (`word_similarity(q, dimmi_unaccent(lower(subject))) > 0.4` using `%>` operator, index-backed), limit 25, `meta.search.fuzzy = true`.

`statement_timeout` 2 s via `SET LOCAL`; timeout returns 503 `search_timeout` with `Retry-After: 1` (never 500).

## Contact search
- `q` with `@`: exact email first (`(workspace_id, email)` index), then trigram on email.
- Else: FTS on `search` with prefix last term UNION trigram on name (`%>`), rank: exact external_id/email = 3, FTS rank, trigram similarity; limit 25; excludes `merged_into_id IS NOT NULL` and deleted.
- Phone-like query (digits, `+`): normalized to E.164 digits, prefix match on `phone`.

## Cmd+K `/search`
Runs conversation (limit 5), contact (5), company (3) searches concurrently (3 pooled connections, each own `statement_timeout` 1 s). One failing section returns `[]` plus `meta.errors.<section> = "timeout"`, never whole failure. Number/id jump returns single exact result in `data.jump`.

## Search everywhere (I-101)

`/search` already built; nothing on screen calls it. Command menu Results section ([workbench.md](workbench.md)) and `/` calls it. Email and id at top: `q` containing `@` runs exact contact email first; `conv_`, `ctc_`, `msg_`, `#123` return `data.jump`. List search box stays current-view. Enter from menu opens conversation at `message_id` (`around`).

## Filter screen (I-100)

`resources/js/features/inbox/search/search-filters.tsx`: `Popover` beside box with Assignee, Team, Tag, Inbox, Channel, Status, Priority, Started range (`date-picker`), Contact, Company. Selecting writes operator text into box (one source of truth, teachable). Match all or any toggle adds `any:`. Save as view: converts to definition. e2e `tests/e2e/inbox-search.spec.ts`.

## Structured search (P-04)

`POST /conversation-searches`, `conversations:read`, read-only (rest rule 11; computed, not stored; POST since body nests; not idempotency-keyed).

```json
{ "query": { "op": "and", "groups": [ { "op": "and", "conditions": [
    { "field": "priority", "op": "gte", "value": "high" },
    { "field": "waiting_since", "op": "older_than", "value": "PT4H" } ] } ] },
  "sort": "waiting_since", "limit": 50, "cursor": null, "include": ["contact"] }
```

Same grammar and compiler as custom views ([views.md](views.md)). Response same as `GET /conversations` plus `meta.total_capped`. Same scope, cursor hash, 400/422 rules. `statement_timeout` 2 s, 503 `search_timeout`. `waiting_since` sort also on `GET /conversations`. MCP `dimmi_structured_search`.

## Budgets
- Selective query (under 2,000 matches): p95 under 50 ms at 1M messages.
- Any query: p95 under 150 ms (cap guarantees bounded heap fetch).
- Cmd+K: p95 under 80 ms.
- Structured, indexed fields: p95 under 100 ms; with `text` condition: as search.
Bench queries in M11: rare word (1 hit), medium (500 hits), common (50k hits), phrase, prefix, fuzzy fallback, with and without filters.
