# 07 Dimmi Agent (BYOK AI support agent)

Style: caveman. Scope trim 2026-11-02: 14 per-post AI summary (later: `feedback-ai-summary`) and 15 survey AI summary spend (later: `survey-ai-summary`) removed; agent actions now launch item; 08 docs sync (`docs_sync` task runs) and 16 imports (never trigger agent) integrated. Date 2026-11-01. Backend only. Read with `specs/00-conventions/README.md`, `specs/00-foundation/*`. Detail files: [pipeline.md](pipeline.md) (answer pipeline, prompts, safety, streaming, handoff, spend cap), [llm-contract.md](llm-contract.md) (LLM contract, drivers, provider facts), [actions.md](actions.md) (agent actions: connections, HTTP/MCP tools, identity binding, confirmation, approval, test runner, templates, OpenAPI import; launch item 18), [edge-cases.md](edge-cases.md), [tasks.md](tasks.md).

## Goal

AI agent answers customers on every channel from workspace knowledge, with citations, using workspace's own LLM key. Says "I don't know" when not grounded, stays silent on "thanks", hands off to human with summary any time customer asks.

Why: roadmap launch item 7 (`research/roadmap.md` "7. Dimmi Agent (BYOK)"). AI first answer is table stakes 2026; every competitor bills per resolution (Fin $0.99, Productlane $0.79, Featurebase $0.49). Dimmi charges zero AI markup and same price human or bot (`research/pricing.md` D6, section 5).

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Key pasted to agent answering in playground | under 2 minutes (one form: provider, key; models auto-suggested) |
| Key pasted to agent live on widget | under 10 minutes (with 08 knowledge indexed) |
| Validation on save | under 3 s p95, zero tokens spent (list endpoint) for 5 of 6 providers |
| First visible token in widget after debounce | p50 1.5 s, p95 4 s (Haiku-class classifier, Sonnet-class answer) |
| Pipeline overhead excl. provider | p95 under 120 ms (fake LLM benchmark in CI) |
| Answers with zero valid citations sent | 0 (enforced in Go) |
| "Talk to a human" | visible on every AI reply on every channel; honored in under 1 s (no LLM on path) |
| Handoff notification with summary to teammate | under 10 s |
| Silence on "thanks"/"ok"/emoji | 100% of acknowledgement-classified messages get no reply |
| Cost per conversation visible | every run: tokens, cost (micro-USD), model; conversation total; month total |
| Spend cap overshoot | bounded and documented (max `ai.workspace_concurrency` (3) concurrent runs x per-run ceiling) |
| Provider coverage day one | OpenAI, Anthropic, Gemini, OpenRouter, Bedrock, any OpenAI-compatible (Ollama, LM Studio, vLLM) |
| Tests | `go test ./...` with zero network, zero real LLM calls; every driver passes contract suite on recorded fixtures |
| Prompts | open, versioned files in repo; every run records prompt version + hashes |

## Scope

In (MVP):
- Provider keys CRUD, envelope-encrypted, validated on save, rotation in place, model lists, test probe.
- Model per role: answer model, classifier model (cheap), optional fallback model. Embedding model chosen in 08, using key from here.
- LLM contract + 6 drivers with streaming, tool calling, usage incl. cache tokens, typed errors, retries, timeouts, circuit breaker.
- Agent config: name, avatar, tone preset, guidance, enabled channels, schedule vs office hours, handoff rules, languages, max AI replies, grounding mode, privacy, link allowlist, spend cap.
- Answer pipeline: trigger, skip rules, classify (intent, language, query rewrite), retrieval via 08, answerability gate, grounded answer with citations, unknown path, silence path, refusals, handoff with summary + routing, commit with supersede checks.
- Streaming to widget via realtime deltas; whole message to email and chat apps.
- `ai_runs` / `ai_run_steps` traces with tokens and cost; per-conversation and monthly spend; hard monthly cap with alerts.
- Playground API (no customer message), runs API, usage API, price table API.
- Prompt injection defenses, PII redaction, outbound link allowlist, content safety refusals.
- Price table as config file, hot reloaded, `dimmi ai:prices:sync`.
- MCP tools for playground, runs, agent config, conversation AI pause/resume.
- Agent actions (launch item 18, [actions.md](actions.md)): connections (HTTP, remote MCP), actions with input schema + identity binding from verified login, read inline, write via customer confirmation card + optional teammate approval, test runner, playground fake/live, templates (Stripe, Supabase, Clerk, Resend), OpenAPI import, timeline + audit trail, MCP toolset `ai_actions`.

Out (later):
- Agent action extensions (actions.md "Out"): `action-oauth-client-credentials`, `action-mcp-oauth`, `email-action-confirmations`, `action-procedures`, `action-conditional-approval`, `action-sql-connector`.
- Evals CLI `dimmi test`, replays (post-MVP: agent evals). `ai_runs` stores snapshots so replay possible later.
- Knowledge-gap report (post-MVP: knowledge-gap report). Runs with outcome `unknown` + queries already stored for it.
- Agent assist for humans: drafts, summarize on demand, translate (post-MVP: agent assist for humans).
- Auto-translation of help center (post-MVP: auto-translation).
- Image/attachment understanding, voice.
- Multiple agents per workspace, audiences/brands (table supports, API singleton).
- Azure OpenAI native driver (workaround via openai_compatible).
- Dimmi-sold AI credits (pricing.md "later").
- Edit-streaming on Slack/Discord.
- CSAT on AI answers: covered by 15-announcements-surveys CSAT (`ai_involved` split). Thumbs feedback on single answers and AI reports: post-MVP: reports.

## User stories

Workspace member (owner/admin):
- Paste OpenAI/Anthropic/... key, see "Valid, 42 models", pick suggested answer + classifier models.
- Point at local Ollama on self-host with one URL, get exact error telling which config allows localhost.
- Write guidance ("Never promise refunds; for refunds hand off"), choose tone, enable widget + email.
- Ask playground "how do I cancel?", see retrieved chunks with scores, answer, citations, tokens, cost, latency.
- Set $25 monthly AI cap, get email at 80%, AI stops at 100% and humans take over.
- Rotate leaked key without downtime.
- Pick Stripe template, paste restricted key, test against own customer, publish: agent answers "what's my plan?" for signed-in customers.
- Add `refund_order` write action with teammate approval; approve from notification.

Workspace member (any):
- See in conversation: AI answered with which sources, cost $0.0031, open trace when answer wrong.
- Pause AI on one conversation; hand back to AI.
- Get notified on handoff with 3-bullet summary.

End customer:
- Ask question in widget, see answer stream in under 2 s with source links.
- Type "human" or press button: connected immediately, told when team is back if offline.
- Write in German, get German answer.
- Say "thanks": no extra bot message.
- Signed in: ask "where is my order 1042?", get status from shop API in seconds; ask refund, see "Refund $20.00 for order #1042?" card, confirm with one click.

Developer:
- Use REST/MCP to test agent from CI script (playground endpoint), fetch run traces, read cost per conversation via API and `ai.run_completed` webhook.
- Self-host with `DIMMI_AI__BOOTSTRAP__OPENAI__API_KEY` env, agent works after first boot.
- Read and fork prompts in repo.

AI agent (external, via MCP):
- `dimmi_ask_agent` to check what Dimmi Agent would answer; `dimmi_get_ai_run` to debug; `dimmi_pause_conversation_ai` before human-like intervention.

## Data model

New tables (all tenant tables: `workspace_id uuid not null references workspaces(id) on delete cascade`, indexes lead with `workspace_id`, RLS in cloud, `created_at`, `updated_at`). Public ID prefixes (new, add to `app/models/public_ids.go`, `framework/ids` registry): `aia_` agent, `aik_` provider key, `air_` run, `ais_` run step; agent actions (actions.md "Data model"): `acn_` connection, `aac_` action, `aacv_` action version, `aax_` action call.

### ai_provider_keys

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | |
| workspace_id | uuid | |
| name | text not null | 1 to 60 chars, unique per workspace among non-deleted (`lower(name)`) |
| provider | text not null CHECK (provider ~ '^[a-z][a-z0-9_]{1,39}$') | registry enum (extensibility.md P1, 00-conventions registry list): Go registry `enums.AiProvider` (`app/enums/ai_provider.go`), validated in request rules; MVP values `openai`, `anthropic`, `gemini`, `openrouter`, `bedrock`, `openai_compatible`, `voyage`; new driver registers at boot with its `LLMManager` factory, no migration.  `voyage` embeddings-only (requested by 08 so Anthropic-only users get vectors): no LLM driver, never selectable as answer/classifier/fallback key (422 `provider_key_embeddings_only`); usable only as 08 `embedding_key_id` |
| base_url | text | required for `openai_compatible`; optional override for openai/anthropic/openrouter (proxies, gateways) and voyage (legacy Voyage base vs default MongoDB Atlas endpoint); null otherwise. Normalized (no trailing slash), http allowed only when host private and allowed (self-host) |
| region | text | bedrock only, `^[a-z]{2}(-[a-z]+)+-\d$` |
| auth_type | text not null default 'api_key' CHECK in ('api_key','aws_iam','bedrock_api_key','none') | `none` for local servers without auth |
| credentials_encrypted | bytea | `encryption.Sealed` (`Seal/Open[ProviderCredentials]`): `{api_key}` or `{access_key_id, secret_access_key, session_token}`; null for `none` |
| key_hint | text | last 4 chars of secret (`…a1B9`); bedrock IAM: last 4 of access key id |
| status | text not null default 'unverified' CHECK in ('unverified','valid','invalid','error') | `invalid` = auth failed; `error` = quota/permission/network at last check |
| status_reason | text | stable code: `auth`, `quota`, `permission`, `network`, `blocked_host`, `model_not_found` |
| last_error | text | human message, redacted, max 500 chars |
| last_validated_at, last_error_at, last_used_at | timestamptz | `last_used_at` updated at most once per minute (cache gate) |
| models | jsonb not null default '[]' | cached `[{id, display_name, context_tokens}]`, max 2,000 entries, refreshed on validate |
| provider_info | jsonb not null default '{}' | non-secret provider facts (OpenRouter `limit_remaining`, `is_free_tier`) |
| config | jsonb not null default '{}' | non-secret: `organization`, `project` (openai), `anthropic_version`, `extra_header_names` (values treated secret, stored inside `credentials_encrypted.extra_headers`), `max_tokens_field`, `supports_tools`, `supports_stream_usage`, `json_mode`, `price_overrides` `[{model, input_per_mtok, cached_input_per_mtok, output_per_mtok}]` |
| source | text not null default 'dashboard' CHECK in ('dashboard','api','env') | `env` = self-host bootstrap |
| created_by_member_id | uuid null references workspace_members on delete set null | |
| deleted_at | timestamptz | soft delete; credentials_encrypted nulled at delete (crypto-shred) |

Indexes: unique `(workspace_id, lower(name)) where deleted_at is null`; `(workspace_id, provider) where deleted_at is null`. Limit 20 non-deleted keys per workspace.

### ai_agents

Singleton per workspace at MVP (unique `(workspace_id) where deleted_at is null`). Created lazily on first GET/PATCH with defaults, `enabled=false`. Also holds workspace AI settings (spend cap) while singleton.

| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| workspace_id | uuid | |
| name | text not null default 'Dimmi' | 1 to 40 chars, shown as author name |
| avatar_attachment_id | uuid null references attachments on delete set null | image, max 1 MB, square crop by client |
| enabled | boolean not null default false | |
| product_name | text | used in prompts ("questions about {product}"); default workspace name |
| tone | text not null default 'friendly' CHECK in ('friendly','professional','concise') | maps to `resources/prompts/agent/<v>/tones/*.md` |
| guidance | text not null default '' | max 4,000 chars |
| enabled_channels | text[] not null default '{widget}' | subset of `channel_connections.type` registry values (widget, email, slack, discord, telegram, whatsapp, api; validated against `enums.ChannelType` registry in request rule, no CHECK: new channel driver needs no 07 migration) |
| excluded_channel_connection_ids | uuid[] not null default '{}' | per-connection opt-out (e.g. one Slack workspace) |
| schedule | text not null default 'always' CHECK in ('always','outside_office_hours','inside_office_hours') | |
| office_hours | jsonb | `{"timezone":"Europe/Berlin","weekly":{"mon":[["09:00","17:30"]],...},"holidays":["2026-12-25"]}`; null = always open; timezone default workspace timezone. Owned by 07 at MVP: 03 defines no office hours (03 README: office hours post-MVP 11). Moves to 03 workspace-level setting post-MVP; 07 then reads it, column dropped via expand/contract |
| assigned_behavior | text not null default 'skip_when_human_active' CHECK in ('skip_when_human_active','never_when_assigned') | |
| answer_key_id | uuid null references ai_provider_keys | |
| answer_model | text | max 200 chars |
| classifier_key_id | uuid null references ai_provider_keys | null = same as answer key |
| classifier_model | text | null = answer model |
| fallback_key_id | uuid null references ai_provider_keys | optional |
| fallback_model | text | |
| temperature | numeric(3,2) | null = provider default; 0 to 1 |
| max_output_tokens | int not null default 600 | 200 to 2,000 (email uses max(this, 1000) capped 2,000) |
| max_ai_replies | int not null default 5 | 1 to 50 AI answers per conversation before forced handoff |
| unknown_behavior | text not null default 'offer_handoff' CHECK in ('offer_handoff','handoff') | |
| grounding_mode | text not null default 'citations' CHECK in ('citations','verify') | |
| require_citations | boolean not null default true | |
| min_similarity | numeric(3,2) not null default 0.35 | 0 to 0.9 |
| languages | text[] not null default '{}' | empty = any; BCP 47 primary subtags |
| default_language | text not null default 'en' | templates fallback |
| streaming | boolean not null default true | widget only |
| include_contact_context | text not null default 'basic' CHECK in ('none','basic','allowlist') | |
| contact_attribute_allowlist | text[] not null default '{}' | max 20 keys |
| pii_redaction | boolean not null default true | |
| link_allowlist | text[] not null default '{}' | domains, `*.example.com` allowed, max 50 |
| handoff | jsonb not null default '{}' | `{"team_id": "<uuid>" or null, "set_priority": false}` |
| messages | jsonb not null default '{}' | overrides per template key: `unknown`, `handoff_open`, `handoff_closed`, `greeting`, `refusal`; per language map `{"en": "...", "de": "..."}`; max 500 chars each |
| prompt_version | text | null = latest embedded |
| monthly_spend_cap_micros | bigint default 25000000 | USD micro-dollars ($25); null = no cap (UI warns) |
| actions_enabled | boolean not null default true | workspace switch for all agent actions (actions.md); false: no action offered, pending cards canceled |
| created_at, updated_at, deleted_at | | |

Readiness (computed, not stored): enabled requires answer key `valid`, answer model set and present in key `models` (or probe passed), 08 `Knowledge.Status` reports `HasContent` (active chunk visible to agent), at least 1 enabled channel connected.

### ai_runs

One per pipeline execution, playground question, help-center Ask AI (08), or handoff-only path. Append-mostly.

| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| workspace_id | uuid | |
| agent_id | uuid not null references ai_agents | |
| kind | text not null CHECK in ('conversation','playground','help_center','changelog','feedback','docs_sync','inbox_translation') | registry `enums.AiRunKind`. `inbox_translation` (03 collaboration.md translation; 07 spend cap applies), `changelog` (13), `feedback` (14) and `docs_sync` (08 docs sync; purposes `screen_change`, `draft_edit`, `draft_article`, `verify_article` in `config_snapshot.purpose`) = `ai.RunTask` runs (no conversation, no agent enable needed); runs list filter `kind` shows them. Action follow-up runs are `conversation` with `trigger_action_call_id` |
| conversation_id | uuid null references conversations on delete cascade | null for playground/help_center/changelog/feedback/docs_sync |
| trigger_message_id | uuid null references messages on delete set null | last customer message included |
| trigger_action_call_id | uuid null | follow-up run reporting write action result (FK to `ai_action_calls` added by `000710`) |
| action_calls_count | int not null default 0 | agent action calls requested in run |
| included_message_ids | uuid[] not null default '{}' | customer messages batched in this run (debounce); replay input |
| actor_type, actor_id | text, uuid | playground: member/token; conversation: contact |
| channel_type | text | widget, email, slack, ... |
| status | text not null CHECK in ('running','completed','failed','skipped') | |
| outcome | text CHECK in ('answered','unknown','handoff','silent','greeted','refused','skipped','superseded','error','awaiting_confirmation','awaiting_approval') | null while running; `awaiting_*` = action card committed (actions.md) |
| skip_reason | text | codes from pipeline.md gate table |
| handoff_reason | text | `customer_requested`, `unknown`, `unknown_streak`, `max_replies`, `frustration`, `feedback`, `abusive`, `safety`, `unsupported_language`, `provider_error`, `spend_cap`, `billing_paused`, `schedule`, `human_replied`, `member_paused`, `agent_disabled`, `action_failed`, `action_declined`, `action_expired` |
| intent | text | classifier intent |
| language | text | |
| query_original | text | redacted, max 2,000 chars |
| query_rewritten | text | redacted |
| retrieval | jsonb not null default '[]' | `[{n, chunk_id, document_id, article_id, source_id, title, url, similarity, score, content_hash, used}]` (text copies live in step request for replay) |
| answer_message_id | uuid null references messages on delete set null | |
| prompt_version | text not null | |
| prompt_hashes | jsonb not null default '{}' | sha256 per rendered system block |
| config_snapshot | jsonb not null default '{}' | agent settings relevant to answer (models, tone, grounding, thresholds, guidance hash), never secrets |
| flags | text[] not null default '{}' | `injection_suspected`, `truncated_input`, `usage_estimated`, `price_unknown`, `fallback_used`, `degraded_retrieval`, `blocklist_hit`, `action_called`, `action_blocked`, `action_failed` |
| input_tokens, cached_input_tokens, cache_write_tokens, output_tokens | bigint not null default 0 | sums over steps |
| cost_micros | bigint not null default 0 | USD micro-dollars, sum over steps |
| price_table_version | text | |
| first_token_ms, latency_ms, queue_wait_ms | int | |
| error_code, error_message | text | redacted |
| started_at, completed_at | timestamptz | |

Indexes: `(workspace_id, conversation_id, created_at)`, `(workspace_id, created_at desc)`, `(workspace_id, kind, outcome, created_at desc)`, `(workspace_id, trigger_message_id)`.

### ai_run_steps

| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| workspace_id | uuid | |
| run_id | uuid not null references ai_runs on delete cascade | |
| seq | int not null | unique `(run_id, seq)` |
| kind | text not null CHECK in ('classify','retrieve','generate','tool_call','verify','summarize','translate','probe') | `tool_call`: one per `search_knowledge` or agent action call (`request` carries `{name, action_call_id}`) |
| provider | text | |
| provider_key_id | uuid null references ai_provider_keys on delete set null | |
| model | text | requested model |
| response_model | text | model reported by provider (OpenRouter routing) |
| attempt | int not null default 1 | each attempt own row, `seq` increments, `retry_of_step_id` points to failed attempt |
| retry_of_step_id | uuid null | |
| status | text not null CHECK in ('ok','error','canceled') | |
| error_kind, provider_error_code | text | `LLMError.Kind`, raw provider code |
| http_status | int | |
| provider_request_id | text | |
| request | jsonb | redacted rendered messages, tool specs, params; max 256 KB (truncated with marker); nulled after retention. `RunTaskInput.PersistInput = false` (08 docs sync): params, prompt hashes, message sizes only, never message text |
| response | jsonb | text, tool calls, stop reason; max 256 KB; nulled after retention |
| input_tokens, cached_input_tokens, cache_write_tokens, output_tokens, reasoning_tokens | bigint not null default 0 | |
| usage_estimated | boolean not null default false | |
| cost_micros | bigint not null default 0 | |
| latency_ms, first_token_ms | int | |
| started_at, completed_at | timestamptz | |

Index `(workspace_id, run_id, seq)`. Retention: `request`/`response` nulled after `ai.traces.retention_days` (default 30); rows (numbers) kept with run.

### ai_spend_periods

Fast cap check and monthly totals. One row per workspace per UTC month.

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid | pk part |
| period_start | date | pk part, first day of UTC month |
| cost_micros | bigint not null default 0 | |
| input_tokens, cached_input_tokens, output_tokens | bigint not null default 0 | |
| runs_count, answered_count, unknown_count, handoff_count, silent_count | int not null default 0 | |
| thresholds_notified | int[] not null default '{}' | subset of {50, 80, 100} |
| capped_at | timestamptz | |
| updated_at | timestamptz | |

Updated by `UPDATE ... SET cost_micros = cost_micros + $1 ... RETURNING` (row lock only, no table scan). Daily breakdown for usage API computed from `ai_runs` (index `(workspace_id, created_at)`) and cached 60 s.

### Core table changes

Migration `20261101000701_add_ai_columns_to_conversations_table.sql`:
| Column | Type | Notes |
|---|---|---|
| ai_reply_count | int not null default 0 | answered AI replies; drives `max_ai_replies` |
| ai_cost_micros | bigint not null default 0 | inbox list/sidebar cost without aggregate |
| ai_language | text | sticky detected customer language |
| ai_handed_off_at | timestamptz | |
| ai_handoff_reason | text | |
`ai_state` already exists in core (07 owns transitions). No new index (all read with conversation row).

`messages`: no schema change. Uses `author_type='agent'`, `author_id=ai_agents.id`, `metadata.ai` = `{run_id, outcome, citations[], model, language, actions[]}` (`actions: ["handoff"]` renders button in widget; `request_email: true` asks 01 to show email capture); agent actions add `confirmation {action_call_id, title, text, status, expires_at}` (widget card) and `action_calls [{id, name, title, status}]` (member resources only). Reads inbound `substantive = false` (any channel `AutoGenerated`), 04 `metadata.auto_reply = true` and 04 `metadata.ai_skip` (gate rules 6, 6b in pipeline.md).

## Public actions for other specs

One generic task action (`ai.RunTask`: 08 docs sync, 13 changelog, 14 feedback) plus spend helpers. All in `app/actions/ai/`.

```go
// run_task.go
type RunTaskInput struct {
    WorkspaceID     ids.UUID
    Kind            enums.AiRunKind      // changelog (13) | feedback (14) | docs_sync (08); registry of task kinds, new specs add values
    Purpose         string               // caller label stored in ai_runs.config_snapshot (13 "generate_entry", 14 "capture" | "duplicate", 08 docs sync "screen_change" | "draft_edit" | "draft_article" | "verify_article")
    ModelRole       enums.AiModelRole    // answer | classifier; answer missing -> classifier key (13 rule)
    StepKind        enums.AiRunStepKind  // generate (default) | classify | summarize
    System          []contracts.SystemBlock // caller prompts, owned by caller under resources/prompts/<kind>/
    Messages        []contracts.ChatMessage // untrusted text already wrapped by caller with 07 envelope helper (`ai.WrapUntrusted`)
    ResponseFormat  contracts.ResponseFormat // text | json
    JSONSchema      json.RawMessage      // optional; json output validated, retried once with "Return valid JSON only"
    MaxOutputTokens int
    Redact          bool                 // run PII redaction (pipeline.md Privacy) on Messages
    PersistInput    *bool                // nil = true (Go zero value safe); false -> ai_run_steps.request keeps params, prompt hashes, message sizes only, never message text (08 docs sync passes false: customer code never stored). Response stored as usual
    Actor           contracts.Actor
}
type RunTaskResult struct {
    Text       string
    JSON       json.RawMessage           // set when ResponseFormat json
    Usage      contracts.LLMUsage
    CostMicros int64
    RunID      ids.UUID                  // ai_runs row (kind = Kind)
}
func RunTask(ctx context.Context, a *app.App, in RunTaskInput) (RunTaskResult, error)
```

- Steps: `ResolveProviderKey` (role key + model; key readiness only: agent `enabled` not required), `PreflightSpend` (cap reached: `spend_cap_reached`), billing gate 9b (cloud), `llmretry` + fallback, one `ai_run_steps` row (`StepKind`), `RecordAiSpend` (kind `task`), `ai_runs` row (kind = `Kind`, `conversation_id` null, outcome `answered` or `error`). Anonymous budget n/a.
- Errors (`fault` codes): `ai_unavailable` (no usable key), `spend_cap_reached`, `provider_error` (`LLMError` kind attached), `invalid_output` (JSON invalid after retry).
- 08 docs sync, 13 and 14 all call `RunTask`. Never streams; max 1 LLM call (+1 JSON retry).

Helpers (also used by 08, 13):
- `ResolveProviderKey(ctx, ws, role) (models.AiProviderKey, model string, err)`: agent role key/model; `ai_unavailable` when none valid.
- `PreflightSpend(ctx, ws) error`: usable outside runs (13 preflight before generation); returns `spend_cap_reached` when `capped_at` set or spend >= cap. Read-only, no reservation.
- `RecordAiSpend(ctx, tx, RecordAiSpendInput{WorkspaceID, Kind, Provider, Model, Usage, CostMicros, RunID *ids.UUID})`: kinds `agent_run`, `playground`, `help_center`, `task`, `embedding` (08); upserts `ai_spend_periods`, thresholds, cap. Kind recorded on step/metric only (period row sums all).
- `WrapUntrusted(label, text) string`: untrusted-data envelope (pipeline.md injection defenses) for callers' own prompts.
- `linkguard.Filter(markdown, allow linkguard.Allowlist) (string, []Removed)`: exported from `app/support/linkguard` for 13 generation output (same rules as agent replies).

## Code map

```
app/contracts/llm.go                          LLM interface, request/response/stream types
app/contracts/llm_errors.go                   LLMError, kinds
app/contracts/knowledge.go                    OWNED BY 08; 07 consumes (see Dependencies)
app/managers/llm_manager.go                   ForKey(ctx, key) -> contracts.LLM, driver cache
app/drivers/llm/openai/{driver.go,stream.go,errors.go,capabilities.go}       openai-go/v3 (ChatCompletionAccumulator)
app/drivers/llm/anthropic/{driver.go,stream.go,errors.go,capabilities.go}    anthropic-sdk-go (Message.Accumulate)
app/drivers/llm/gemini/{driver.go,stream.go,errors.go,capabilities.go}       google.golang.org/genai
app/drivers/llm/openrouter/driver.go          openai-go/v3 with OpenRouter base URL + key info + attribution headers
app/drivers/llm/bedrock/{driver.go,stream.go,errors.go,capabilities.go}      aws-sdk-go-v2 bedrockruntime (Converse/ConverseStream)
app/drivers/llm/openaicompat/{driver.go,stream.go,errors.go,capabilities.go} openai-go/v3 with user base URL (Ollama, LM Studio, vLLM)
app/drivers/llm/fake/fake.go                  llm.Fake() scripted driver for tests
                                              (no internal/sse: SDKs own SSE + tool-delta assembly, llm-contract.md)
app/support/llmretry/{retry.go,breaker.go}    retries (cenkalti/backoff/v5), Retry-After, fallback, circuit breaker (sony/gobreaker/v2)
app/support/aiprices/{table.go,loader.go,cost.go}   price table, hot reload, cost calc (micros)
app/support/pii/{redact.go,restore.go}        PII placeholders
app/support/linkguard/linkguard.go            outbound link allowlist on Markdown (walks `markdown.Parse` AST); exported `Filter` used by 13
app/support/officehours/officehours.go        schedule eval, next open time (DST safe)
app/support/prompts/{loader.go,render.go}     embedded + override prompts, hashes
app/enums/{ai_provider.go (registry),ai_run_kind.go,ai_run_step_kind.go,ai_model_role.go,ai_spend_kind.go,ai_run_outcome.go,ai_state.go,ai_handoff_reason.go,ai_tone.go,ai_schedule.go}
app/actions/ai/
  create_provider_key.go                      validate (sync, 10 s) then store
  update_provider_key.go                      rename, rotate secret (validate before swap), config
  delete_provider_key.go                      in-use check, crypto-shred
  validate_provider_key.go                    ListModels or Probe, status update, model cache
  probe_provider_model.go                     "test model" tiny request
  show_agent.go                               lazy create defaults
  update_agent.go                             validation, readiness when enabling
  check_agent_readiness.go
  run_agent.go                                pipeline orchestrator (steps in app/ai/pipeline)
  answer_question.go                          stateless pipeline for playground + 08 Ask AI (kind help_center forces public-only retrieval, see Retrieval contract)
  reply_as_agent.go                           wraps 03 ReplyToConversation (actor agent, Substantive, dedupe key) inside commit tx
  would_attempt.go                            WouldAttempt(ctx, conversationID) bool: trigger pre-check without LLM (06 notification suppression)
  hand_off_conversation.go
  request_human_handoff.go                    RequestHumanHandoff: called by 07 widget handoff controller (POST /widget/v1/conversations/{conversation}/handoffs); HandOffConversation reason customer_requested, idempotent
  resume_conversation_ai.go                   ResumeConversationAi: called by 03 ApplyConversationChanges for PATCH /conversations/{conversation} `ai_state: active`
  erase_ai_contact_data.go                    EraseAiContactData: GDPR contact eraser (00-foundation/contracts.md "Contact eraser hooks"), registered on 03 eraser registry; nulls ai_run_steps.request/response and ai_runs.query_* for contact's conversations; keeps token, cost, model, timing numbers; chunked, idempotent
  pause_conversation_ai.go                    PauseConversationAi: called by 03 ApplyConversationChanges for PATCH `ai_state: handed_off` (HandOffConversation reason member_paused)
  record_ai_spend.go                          spend upsert + thresholds + cap (kinds incl. task, embedding)
  run_task.go                                 RunTask: generic one-shot task for 13 (changelog) and 14 (feedback)
  resolve_provider_key.go                     ResolveProviderKey(ws, role)
  preflight_spend.go                          PreflightSpend: cap check usable outside runs
  wrap_untrusted.go                           WrapUntrusted envelope helper for callers' prompts
  acquire_ai_slot.go                          workspace concurrency slot lease (Valkey)
  summarize_handoff.go
  bootstrap_env_provider_keys.go              self-host env keys into first workspace
app/ai/pipeline/                              pipeline stages (tested through RunDimmiAgent integration tests, testing.md)
  gate.go fastpath.go classify.go retrieve.go answerability.go generate.go
  stream_buffer.go postprocess.go verify.go commit.go templates.go
app/ai/tools/search_knowledge.go              built-in tool; agent actions in app/ai/actions/ (actions.md "Code map": connections, actions, versions, calls, executor, MCP client, templates, OpenAPI import)
app/http/controllers/api/v1/
  ai_provider_key_controller.go               Index, Store, Show, Update, Destroy
  ai_provider_key_validation_controller.go    Store (POST .../validations)
  ai_provider_key_model_controller.go         Index (refresh param)
  ai_provider_key_probe_controller.go         Store (POST .../probes)
  ai_provider_catalog_controller.go           Index (static providers + suggested models)
  ai_agent_controller.go                      Show, Update
  ai_agent_readiness_controller.go            Show
  ai_playground_run_controller.go             Store (POST /ai/playground-runs; JSON or SSE)
  ai_run_controller.go                        Index, Show
  ai_usage_controller.go                      Show
  ai_price_controller.go                      Index
  conversation_ai_controller.go               Show
  (no pause/resume controllers: `ai_state` is a field on 03 `PATCH /conversations/{conversation}`, 03 controller hands it to 07 actions; rest.md rule 3)
app/http/controllers/widget/v1/
  conversation_handoff_controller.go          Store: POST /widget/v1/conversations/{conversation}/handoffs (07 owns controller + action; 01 owns widget route group, registers `widget.conversations.handoffs.store` in routes/widget.go)
app/http/requests/{store_ai_provider_key_request.go,update_ai_provider_key_request.go,probe_ai_model_request.go,update_ai_agent_request.go,store_ai_playground_run_request.go,list_ai_runs_request.go,show_ai_usage_request.go}
app/http/resources/{ai_provider_key_resource.go,ai_agent_resource.go,ai_run_resource.go,ai_run_step_resource.go,ai_usage_resource.go,ai_price_resource.go,conversation_ai_resource.go,ai_readiness_resource.go}
app/policies/{ai_provider_key_policy.go,ai_agent_policy.go,ai_run_policy.go}
app/events/{ai_reply_started.go,ai_reply_delta.go,ai_reply_discarded.go,ai_state_changed.go,ai_handoff_requested.go,ai_run_completed.go,ai_spend_threshold_reached.go,ai_spend_cap_reached.go,ai_spend_cap_cleared.go,ai_provider_key_failed.go,ai_provider_key_changed.go,ai_agent_updated.go}
app/listeners/
  trigger_dimmi_agent.go                      queued ai: message.created (contact)
  pause_agent_on_member_reply.go              queued default: message.created (member, public)
  reset_agent_state_on_close.go               queued default: conversation.closed
  cancel_agent_run.go                         sync after commit: conversation.assigned/marked_spam/deleted, message.created (member) -> PubSub cancel signal
  retrigger_billing_paused_runs.go            queued ai: billing.activations_resumed (cloud)
  notify_ai_spend.go                          queued default: ai.spend_threshold_reached, ai.spend_cap_reached
  notify_ai_provider_key_failed.go            queued default: ai.provider_key_failed
app/jobs/{run_dimmi_agent.go,summarize_handoff.go,revalidate_provider_keys.go,prune_ai_traces.go,sync_ai_prices.go}
app/notifications/{ai_spend_threshold_notification.go,ai_spend_cap_reached_notification.go,ai_provider_key_failed_notification.go}
app/mcp/tools/{ask_agent.go,list_ai_runs.go,get_ai_run.go,get_ai_agent.go,update_ai_agent.go,pause_conversation_ai.go,resume_conversation_ai.go,get_ai_usage.go}
app/console/commands/{ai_prices_sync_command.go,ai_prices_show_command.go,ai_keys_validate_command.go,ai_run_show_command.go}
app/broadcasting/ai_channels.go               delta payload builders (widget-safe)
config/ai.go
resources/prompts/agent/2026-11-01/{system.md,classify.md,answer_user.md,verify.md,summarize_handoff.md,translate.md,tones/*.md,CHANGELOG.md}
resources/ai/prices.yaml                      embedded default price table
resources/ai/human_phrases.yaml
resources/ai/blocklist/{en,es,de,fr,pt}.txt
lang/{en,es,de,fr,pt,it,nl,ja,zh,ko}/ai.json  templates: unknown, handoff_open, handoff_closed, greeting, refusal_off_topic, refusal_abusive, safety, human_footer, cap_notice
database/migrations/
  20261101000701_add_ai_columns_to_conversations_table.sql
  20261101000702_create_ai_provider_keys_table.sql
  20261101000703_create_ai_agents_table.sql
  20261101000704_create_ai_runs_table.sql
  20261101000705_create_ai_run_steps_table.sql
  20261101000706_create_ai_spend_periods_table.sql
  20261101000707..000710                       agent actions tables (actions.md)
database/queries/{ai_provider_keys.sql,ai_agents.sql,ai_runs.sql,ai_run_steps.sql,ai_spend_periods.sql}
database/factories/{ai_provider_key_factory.go,ai_agent_factory.go,ai_run_factory.go}
database/seeders/ai_seeder.go                 demo agent + fake-provider key (dev only, provider openai_compatible to local fake server)
tests/contracts/llm_test.go                   every driver (SDK wrappers) against fixtures via httpclienttest.New(a.HTTP)
tests/fixtures/llm/<driver>/*.sse|*.json
tests/unit/llm/{openai_map_test.go,anthropic_map_test.go,gemini_map_test.go,bedrock_map_test.go,errors_map_test.go}   driver zone: pure request/usage/error mapping only
tests/feature/ai/{provider_keys_test.go,agent_test.go,playground_test.go,runs_test.go,usage_test.go,conversation_ai_test.go,widget_handoff_test.go,pipeline_test.go,spend_cap_test.go,metering_test.go,streaming_test.go}
tests/feature/ai/{gate_test.go,classify_test.go,answerability_test.go,stream_buffer_test.go,postprocess_test.go,linkguard_test.go,pii_test.go,officehours_test.go,prices_test.go,retry_test.go,breaker_test.go,prompts_golden_test.go,run_task_test.go}   integration (real App, Postgres, Valkey, llm.Fake); no app unit tests
```

## HTTP API

All under `/api/v1`, `auth` (guard `A`: browser session, first-party app, token; abilities apply to tokens only), `workspace`, `throttle:api`. REST rules: `00-foundation/rest.md`. Policies: see Permissions.

| Method | Path | Route name | Permission (Permissions matrix) | Purpose |
|---|---|---|---|---|
| GET | /ai/providers | ai.providers.index | `ai:read` | static catalog: providers, fields, default base URLs, suggested models per role |
| GET | /ai/provider-keys | ai.provider-keys.index | `ai:read` | list keys (no secrets) |
| POST | /ai/provider-keys | ai.provider-keys.store | `ai:keys` | create, validate synchronously, `idempotent` |
| GET | /ai/provider-keys/{key} | ai.provider-keys.show | `ai:read` | |
| PATCH | /ai/provider-keys/{key} | ai.provider-keys.update | `ai:keys` | rename, rotate secret, config |
| DELETE | /ai/provider-keys/{key} | ai.provider-keys.destroy | `ai:keys` | soft delete, 409 when in use unless `?force=true` |
| POST | /ai/provider-keys/{key}/validations | ai.provider-keys.validations.store | `ai:keys` | re-validate now; 201 outcome `ai_provider_key_validation` (`status`, `status_reason`, `models_count`, `key` resource); result lives on key status columns, no own row |
| GET | /ai/provider-keys/{key}/models | ai.provider-keys.models.index | `ai:read` | cached list; `?refresh=true` (`ai:keys`) refetches |
| POST | /ai/provider-keys/{key}/probes | ai.provider-keys.probes.store | `ai:keys` | tiny real request to model (costs tokens); body `{model}`; 201 `ai_provider_key_probe` (`status`, `latency_ms`, `usage`, `run` compact: probe recorded as `ai_runs` kind `playground`) |
| GET | /ai/agent | ai.agent.show | `ai:read` | agent config + readiness summary |
| PATCH | /ai/agent | ai.agent.update | `ai_agent:manage` (ai:write) | update config, enable/disable |
| GET | /ai/agent/readiness | ai.agent.readiness.show | `ai:read` | checklist |
| POST | /ai/playground-runs | ai.playground-runs.store | `ai:write` | ask question, no customer message; creates `ai_runs` row kind `playground`; 201 JSON or SSE |
| GET | /ai/runs | ai.runs.index | `ai:read` | filter `conversation`, `kind`, `outcome`, `created_after`, `created_before`; sort `-created_at` |
| GET | /ai/runs/{run} | ai.runs.show | `ai:read` | `include=steps,steps.payload` (payload needs `ai_runs:read_payloads`) |
| GET | /ai/usage | ai.usage.show | `ai:read` | `period=2026-11`; totals, cap status, daily series, by model, by outcome |
| GET | /ai/prices | ai.prices.index | `ai:read` | active price table + version + source |
| GET | /conversations/{conversation}/ai | conversations.ai.show | `conversations:read` + 03 `VisibleScope` | state, reply count, cost, last run, handoff reason |
| PATCH | /conversations/{conversation} `{ai_state}` | conversations.update (owned by 03) | `conversations:write` | `ai_state: "handed_off"` stops AI (07 `PauseConversationAi`, reason `member_paused`, no customer notice); `ai_state: "active"` hands back (07 `ResumeConversationAi`). 03 `ApplyConversationChanges` delegates field, same tx, after `status` step (03 api.md "Order of application") |
| POST | /widget/v1/conversations/{conversation}/handoffs | widget.conversations.handoffs.store | `auth:widget` (conversation of this contact) | "Talk to a human" button. 07 owns controller + action (`RequestHumanHandoff`); 01 owns widget route group. 202 `{object: "handoff", status: "requested"}`, idempotent |
| * | /ai/action-templates, /ai/action-connections[/...], /ai/actions[/...], /ai/action-imports, /ai/action-calls[/...], /widget/v1/conversations/{conversation}/action-calls/{call} | ai.action-*.* | `ai_actions:read`, `ai_actions:manage`, `ai_action_calls:approve` | agent actions: full table in actions.md "HTTP API" |

Token abilities (listed in foundation `api.md`): `ai:read`, `ai:write`, `ai:keys`, `ai:actions` (agent actions management). `*` includes them. `settings:write` does NOT imply `ai:keys` (keys cost money; explicit) nor `ai:actions` (customer-API secrets, acts on customers).

### Key examples

`POST /api/v1/ai/provider-keys`
```json
{
  "name": "Anthropic prod",
  "provider": "anthropic",
  "api_key": "sk-ant-api03-...",
  "config": { }
}
```
201:
```json
{
  "data": {
    "id": "aik_01jbf3k2w8x9...",
    "object": "ai_provider_key",
    "name": "Anthropic prod",
    "provider": "anthropic",
    "base_url": null,
    "region": null,
    "auth_type": "api_key",
    "key_hint": "…a1B9",
    "status": "valid",
    "status_reason": null,
    "last_error": null,
    "last_validated_at": "2026-11-01T10:00:02Z",
    "last_used_at": null,
    "models_count": 14,
    "suggested": { "answer_model": "claude-sonnet-5-5", "classifier_model": "claude-haiku-5-5" },
    "provider_info": {},
    "config": {},
    "source": "dashboard",
    "in_use_by": [],
    "created_at": "2026-11-01T10:00:00Z",
    "updated_at": "2026-11-01T10:00:02Z",
    "links": { "self": "https://app.trydimmi.com/api/v1/ai/provider-keys/aik_01jbf3k2w8x9..." }
  }
}
```
Validation failure does NOT store key: 422
```json
{
  "message": "The API key was rejected by Anthropic (401 authentication_error).",
  "errors": { "api_key": ["The API key was rejected by Anthropic (401 authentication_error)."] },
  "code": "provider_key_invalid"
}
```
Ollama on self-host without allow entry: 422 `code: provider_host_blocked`, message "http://localhost:11434 is a private address. Set `a.HTTP`.allow_hosts: [\"localhost:11434\"] in dimmi.yaml (or DIMMI_SAFEHTTP_ALLOW_HOSTS) and restart." Cloud: "Private network addresses are not reachable from Dimmi Cloud. Use a public HTTPS endpoint."
Provider unreachable / 5xx during save: key stored with `status=error`, 201 + `status_reason: network` (user may be offline-configuring local model), `?strict=true` turns into 422.

Bedrock body:
```json
{ "name": "Bedrock us-east-1", "provider": "bedrock", "region": "us-east-1",
  "auth_type": "aws_iam", "aws_access_key_id": "AKIA...", "aws_secret_access_key": "...", "aws_session_token": null }
```
OpenAI-compatible body:
```json
{ "name": "Local Ollama", "provider": "openai_compatible", "base_url": "http://localhost:11434/v1", "auth_type": "none",
  "config": { "max_tokens_field": "max_tokens", "supports_tools": false } }
```

`PATCH /api/v1/ai/agent`
```json
{
  "enabled": true,
  "name": "Ada",
  "tone": "friendly",
  "guidance": "Never promise refunds. For refunds or account deletion, hand off.",
  "enabled_channels": ["widget", "email", "discord"],
  "answer_key_id": "aik_01jbf3k2w8x9...",
  "answer_model": "claude-sonnet-5-5",
  "classifier_model": "claude-haiku-5-5",
  "schedule": "always",
  "office_hours": { "timezone": "Europe/Berlin", "weekly": { "mon": [["09:00","17:30"]], "tue": [["09:00","17:30"]] }, "holidays": [] },
  "handoff": { "team_id": "team_01jb...", "set_priority": false },
  "max_ai_replies": 5,
  "monthly_spend_cap_usd": "25.00"
}
```
API accepts `monthly_spend_cap_usd` decimal string (max 2 decimals, 0.01 to 100000.00, or null); stored as micros. Enabling with failing readiness: 422 `code: agent_not_ready`, `errors.enabled: ["Answer model is not set.", "No knowledge indexed yet."]`. Knowledge empty is a hard block (agent useless without docs; matches "answers only from sources").

`POST /api/v1/ai/playground-runs`
```json
{
  "question": "How do I cancel my plan?",
  "history": [ { "role": "customer", "text": "Hi" }, { "role": "agent", "text": "Hi! How can I help?" } ],
  "channel": "widget",
  "language": null,
  "contact": { "name": "Sam", "custom_attributes": { "plan": "pro" } },
  "overrides": { "answer_model": "gpt-5-mini", "answer_key_id": "aik_...", "guidance": "Be brief.", "grounding_mode": "verify" }
}
```
201 (non-stream):
```json
{
  "data": {
    "object": "ai_playground_run",
    "run": { "id": "air_01jbf...", "object": "ai_run", "kind": "playground", "outcome": "answered" },
    "classification": { "intent": "question", "language": "en", "standalone_query": "How do I cancel my plan?", "search_queries": ["cancel plan"] },
    "retrieved": [
      { "n": 1, "chunk_id": "kc_01jb...", "article_id": "art_01jb...", "title": "Cancel your subscription", "url": "https://help.acme.com/cancel", "similarity": 0.82, "used": true, "text": "Go to Settings > Billing ..." }
    ],
    "answer": { "status": "answered", "body": "You can cancel any time from **Settings > Billing** [1].", "citations": [ { "n": 1, "title": "Cancel your subscription", "url": "https://help.acme.com/cancel" } ] },
    "would_send": true,
    "usage": { "input_tokens": 2310, "cached_input_tokens": 1800, "output_tokens": 64, "cost_usd": "0.000912", "cost_estimated": false },
    "timing": { "first_token_ms": 820, "total_ms": 1460 }
  }
}
```
`Accept: text/event-stream`: SSE events `classification`, `retrieval`, `action_call`, `delta`, `answer`, `usage`, `done`, `error` (same JSON pieces). Agent actions: body fields `actions {mode: off|fake|live, version, only, fakes}` and `contact.verified` / `contact.external_id` / `contact.id`; response `action_calls`, `would_request_confirmation` (actions.md "Playground"). Playground never creates messages, never changes conversation state, never meters (D10), counts toward AI spend cap. Throttle 30/min per member. Playground works with `enabled=false` (test before going live) but requires key + model.

`GET /api/v1/ai/runs/{run}?include=steps`
```json
{
  "data": {
    "id": "air_01jbf...", "object": "ai_run", "kind": "conversation",
    "conversation": { "id": "conv_01jb...", "object": "conversation", "number": 1042 },
    "status": "completed", "outcome": "answered", "skip_reason": null, "handoff_reason": null,
    "intent": "question", "language": "de", "channel_type": "widget",
    "query_original": "wie kündige ich?", "query_rewritten": "Wie kündige ich mein Abo?",
    "retrieval": [ { "n": 1, "title": "Abo kündigen", "url": "https://help.acme.com/de/cancel", "similarity": 0.79, "used": true } ],
    "answer_message": { "id": "msg_01jb...", "object": "message" },
    "prompt_version": "2026-11-01",
    "flags": [],
    "usage": { "input_tokens": 3120, "cached_input_tokens": 2400, "cache_write_tokens": 0, "output_tokens": 181, "cost_usd": "0.003104", "cost_estimated": false, "price_table_version": "2026-11-01" },
    "timing": { "queue_wait_ms": 1520, "first_token_ms": 1310, "latency_ms": 3900 },
    "error": null,
    "steps": [
      { "id": "ais_...", "object": "ai_run_step", "seq": 1, "kind": "classify", "provider": "anthropic", "model": "claude-haiku-5-5", "status": "ok", "input_tokens": 610, "output_tokens": 58, "cost_usd": "0.000090", "latency_ms": 430 },
      { "id": "ais_...", "object": "ai_run_step", "seq": 2, "kind": "retrieve", "status": "ok", "input_tokens": 12, "cost_usd": "0.000001", "latency_ms": 48 },
      { "id": "ais_...", "object": "ai_run_step", "seq": 3, "kind": "generate", "provider": "anthropic", "model": "claude-sonnet-5-5", "status": "ok", "input_tokens": 2498, "cached_input_tokens": 2400, "output_tokens": 123, "cost_usd": "0.003013", "first_token_ms": 610, "latency_ms": 2900 }
    ],
    "created_at": "2026-11-01T10:05:00Z", "completed_at": "2026-11-01T10:05:03Z"
  }
}
```
`include=steps.payload` adds `request`/`response` (redacted prompts; `ai_runs:read_payloads`; null after retention with `payload_expired: true`).

Money in API: decimal strings in USD with 6 decimals (`cost_usd`). Never floats.

### Error codes

| Status | code | When |
|---|---|---|
| 422 | `provider_key_invalid` | provider 401 on save/rotate |
| 422 | `provider_key_forbidden` | permission error on save (model access, region) |
| 422 | `provider_quota_exceeded` | quota/billing error on save |
| 422 | `provider_host_blocked` | `a.HTTP` blocked base URL |
| 422 | `provider_unreachable` | network/5xx with `?strict=true` |
| 422 | `agent_not_ready` | enable with failing readiness |
| 422 | `model_not_available` | model not in key list and probe failed |
| 422 | `provider_key_embeddings_only` | `voyage` key set as answer, classifier or fallback key (agent PATCH, playground override) |
| 409 | `provider_key_in_use` | delete key referenced by agent roles or 08 embeddings |
| 409 | `ai_not_active` | PATCH `ai_state: active` when conversation closed or spam |
| 422 | `invalid_transition` | PATCH `ai_state: active` from `off` (AI activates only via gate); body `from`, `to` (rest.md rule 3) |
| 422 | `provider_keys_limit` | more than 20 keys |
| 423 | `ai_spend_cap_reached` | playground when cap reached |
| 424 | `provider_error` | playground provider failure; body includes `provider_error_kind`, `provider_request_id` |
| 429 | `too_many_requests` | playground throttle |

Agent actions error codes: actions.md "Error codes".

## MCP tools

| Tool | Ability | Purpose |
|---|---|---|
| `dimmi_ask_agent` | `ai:write` | playground question (`POST /ai/playground-runs`); returns answer, citations, retrieved titles, cost |
| `dimmi_list_ai_runs` | `ai:read` | filter conversation/outcome |
| `dimmi_get_ai_run` | `ai:read` | trace without payload |
| `dimmi_get_ai_agent` | `ai:read` | config + readiness |
| `dimmi_update_ai_agent` | `ai:write` | guidance, tone, channels, enabled (no key fields) |
| `dimmi_pause_conversation_ai` | `conversations:write` | REST `PATCH /conversations/{id}` `{ai_state: "handed_off"}` (tool keeps task name, rest.md rule 15) |
| `dimmi_resume_conversation_ai` | `conversations:write` | REST `PATCH /conversations/{id}` `{ai_state: "active"}` |
| `dimmi_get_ai_usage` | `ai:read` | month spend, cap |

No MCP tool creates or reads provider keys (secrets must not pass through agent transcripts). Key setup via dashboard, REST, or env bootstrap.

Agent actions tools live in separate opt-in toolset `ai_actions` (8 tools: list/get/create/update/test/publish actions, list calls, list connections; no secrets, no approvals, no live writes): actions.md "MCP tools".

## CLI commands (server-side, artisan style)

| Command | Purpose |
|---|---|
| `dimmi ai:prices:sync [--url] [--dry-run]` | fetch price table YAML (default `ai.prices.sync_url`), validate schema, write `storage/app/ai/prices.yaml`, hot reload picks up |
| `dimmi ai:prices:show [--provider] [--model]` | print active table + source + version |
| `dimmi ai:keys:validate [--workspace ws_...]` | re-validate keys now |
| `dimmi ai:run:show air_...` | print trace (ops debugging) |

Client CLI (`npx dimmi`) belongs to 09; it wraps REST endpoints above.

## Events

Emitted (all existing catalog names reused where present):
| Event | Realtime | Webhook | Payload |
|---|---|---|---|
| message.created (author agent) | yes | yes | standard message resource (metadata.ai included; widget resource omits run internals except citations + actions) |
| ai.handoff_requested (existing) | yes | yes | conversation, reason, summary (bullets), run id |
| ai.run_completed (existing) | no | yes | run resource without steps: outcome, tokens, cost_usd, model, latency |

New events (add to catalog):
| Event | Realtime | Webhook | Channels | Notes |
|---|---|---|---|---|
| ai.reply_started | yes | no | conversation:{conv}, widget_contact:{ctc} | run id, agent name/avatar; widget shows streaming bubble |
| ai.reply_delta | yes (ephemeral, no resume replay) | no | same | seq + text segment |
| ai.reply_discarded | yes | no | same | run id, reason `superseded`, `canceled`, `error`; widget removes bubble |
| ai.state_changed | yes | yes | workspace:{ws}, conversation:{conv}, widget_contact:{ctc} (state only) | from, to, reason; widget toggles human button |
| ai.spend_threshold_reached | yes | yes | workspace:{ws} | percent 50 or 80, period, cost_usd, cap_usd |
| ai.spend_cap_reached | yes | yes | workspace:{ws} | period, cost_usd, cap_usd |
| ai.spend_cap_cleared | yes | yes | workspace:{ws} | cap raised/removed |
| ai.provider_key_failed | yes | yes | workspace:{ws} | key id, name, provider, kind (`auth`, `quota`, `permission`), never secret |
| ai.provider_key_changed | no | no | WriteAuditLog + 08 `SuggestEmbedderOnKeyChange` (queued) | created, rotated, deleted, config changed; key id + provider only |
| ai.agent_updated | yes | no | workspace:{ws} | changed field names (not values of guidance) + audit |
| ai.action_call_updated, ai.action_call_completed, ai.action_call_approval_requested, ai.action_changed, ai.action_connection_failed | see actions.md "Events" | | | agent actions |

Consumed:
| Event | Listener | Action |
|---|---|---|
| message.created (contact, type message) | TriggerDimmiAgent | enqueue RunDimmiAgent |
| message.created (member, type message) | PauseAgentOnMemberReply, CancelAgentRun | ai_state handed_off reason human_replied; cancel in-flight run |
| conversation.created | none | no-op: TriggerDimmiAgent subscribes only to `message.created` (first message always dispatches it) |
| conversation.assigned | CancelAgentRun | cancel in-flight if gate would now skip |
| conversation.closed | ResetAgentStateOnClose | ai_state off |
| conversation.marked_spam, conversation.deleted | CancelAgentRun | cancel |
| billing.activations_resumed (cloud) | RetriggerBillingPausedRuns | re-enqueue RunDimmiAgent for conversations with `billing_paused` skip in last 24 h whose last message is from contact |
| ai.spend_*, ai.provider_key_failed | NotifyAiSpend, NotifyAiProviderKeyFailed | notifications to owners/admins via 06 |

Catalog change for 00-foundation: `SendMemberNotifications` already consumes `ai.handoff_requested`; add `ai.spend_cap_reached`, `ai.spend_threshold_reached`, `ai.provider_key_failed` to its list or keep 07's own notify listeners (07 chooses own listeners, notification classes routed through 06 channels).

## Jobs

| Job (kind) | Queue | Trigger | Retries | Uniqueness | Timeout |
|---|---|---|---|---|---|
| RunDimmiAgent (`run_dimmi_agent`) | ai | TriggerDimmiAgent, scheduled +debounce | max 3 attempts; attempt 2-3 only if no message committed; final failure: handoff `provider_error` when ai_state active | by `conversation_id`, states scheduled/available/retryable | 120 s |
| SummarizeHandoff (`summarize_handoff`) | ai | HandOffConversation | 1 retry; deterministic fallback | by `conversation_id` + `handoff_at` | 15 s |
| RevalidateProviderKeys (`revalidate_provider_keys`) | ai | periodic daily 04:00 UTC + `ai:keys:validate` | 3 | periodic unique | 10 min (batches of 50 keys, 1 call each, jittered) |
| PruneAiTraces (`prune_ai_traces`) | maintenance | periodic daily | 3 | periodic unique | 10 min, chunked 5,000 rows; `RetentionGuard` per chunk (below) |
| SyncAiPrices (`sync_ai_prices`) | maintenance | periodic weekly when `ai.prices.auto_sync=true` | 3 | periodic unique | 1 min |
| ExecuteAiActionCall, ExpireAiActionCall, SyncAiActionConnectionTools | ai / default | agent actions | | | actions.md "Jobs" |

Queue `ai` concurrency default 10 per process (`queue.ai.concurrency`). Per-workspace fairness: max `ai.workspace_concurrency` (entitlement, default 3) concurrent RunDimmiAgent per workspace (Valkey slot leases: `SET NX PX 150000` on `ai:slot:{ws}:{0..n-1}`, value = run id, renewed every 30 s, deleted on finish; crash frees slot within 150 s; no DB transaction held during LLM calls); no slot: job snoozes 500 ms (River `JobSnooze`), not counted as attempt. Limit lowered while runs in flight: running jobs finish; new jobs see fewer slots.

Retention rule (`00-foundation/contracts.md` `RetentionGuard`, extensibility.md section 8): `PruneAiTraces` (payload nulling after `ai.traces.retention_days`, run row deletion after `ai.runs.retention_days`, playground run deletion after `ai.playground.retention_days`) calls `RetentionGuard.MayDestroy` per chunk: `{workspace, ws}` once per workspace batch, then `{conversation, conversation_id}` per distinct conversation of conversation runs (playground and probe runs: workspace check only). Held: rows untouched, `retention_held` logged, `dimmi_retention_held_total{kind}`. `EraseAiContactData` runs inside 03 `EraseContact` (guarded there). Feature test with fake guard false per class.

Cancel: in-flight run subscribes internal pubsub (`pubsub.PubSub`, Valkey driver) topic `ai.cancel.{conversation_id}`; CancelAgentRun publishes; worker cancels ctx within 200 ms (stops token spend). Missed message (Valkey blip): commit step re-checks superseded state in Postgres (pipeline.md 10), so no wrong reply, only wasted tokens.

Valkey state (PO rule: rebuildable only): concurrency slot leases, cancel pubsub, abuse counters `ai.contact_daily_runs` / `ai.conversation_hourly_runs` (`a.Limiter`), playground throttle, template translation cache (`cache.Store`, 30 days), `last_used_at` write gate, usage API 60 s cache. Durable in Postgres: keys, agents, runs, steps, `ai_spend_periods` (cap is money: never in Valkey). Valkey unreachable (foundation data-model.md Valkey-down policy): slot leases, abuse counters and playground throttle fail open with `log.Warn` + `dimmi_valkey_fallback_total{subsystem="ai"}` (spend cap in Postgres stays hard guard; per-run cost ceiling bounds overshoot); agent action limiters fail closed (actions.md: actions not offered, no egress). `/ready` 503 meanwhile.

## Config keys (`config/ai.go`, env `DIMMI_AI_*`)

| Key | Default | Notes |
|---|---|---|
| ai.enabled | true | kill switch for whole instance (`false`: agent never runs, API still manages config) |
| ai.prices.path | "" | external YAML; overrides embedded `resources/ai/prices.yaml`; reloaded on mtime change (checked every 60 s) and SIGHUP |
| ai.prices.sync_url | `https://raw.githubusercontent.com/<org>/dimmi/main/resources/ai/prices.yaml` | for `ai:prices:sync`; fetched via `a.HTTP` (framework httpclient), max 1 MB, schema validated |
| ai.prices.auto_sync | false | weekly sync job |
| ai.prompts.version | latest embedded | |
| ai.prompts.path | "" | self-host override dir (ignored when `app.edition=cloud`) |
| ai.traces.retention_days | 30 | payload retention; 1 to 365 |
| ai.traces.store_payloads | true | false: never store request/response (privacy-max mode) |
| ai.runs.retention_days | 400 | run rows (numbers) |
| ai.playground.retention_days | 30 | playground runs deleted |
| ai.debounce.widget_ms / chat_ms / email_ms | 1500 / 2500 / 0 | |
| ai.workspace_concurrency | 3 | self-host value of entitlement `ai.workspace_concurrency`; cloud default in catalog (below) |
| ai.contact_daily_runs | 100 | |
| ai.conversation_hourly_runs | 30 | |
| ai.timeouts.* | see llm-contract.md | |
| ai.retry.max_attempts | 3 | |
| ai.breaker.failures / window / open | 5 / 60s / 60s | |
| ai.anthropic.version | `2023-06-01` | header value; bump via config, no deploy |
| ai.default_monthly_cap_usd | 25 | new agents |
| ai.bootstrap.<provider>.api_key | "" | self-host only: `DIMMI_AI__BOOTSTRAP__OPENAI__API_KEY`, `DIMMI_AI__BOOTSTRAP__ANTHROPIC__API_KEY`, `DIMMI_AI__BOOTSTRAP__GEMINI__API_KEY`, `DIMMI_AI__BOOTSTRAP__OPENROUTER__API_KEY`, `DIMMI_AI__BOOTSTRAP__OPENAI_COMPATIBLE__BASE_URL` (+ `_API_KEY`), `DIMMI_AI__BOOTSTRAP__BEDROCK__REGION` + AWS keys; supports `_FILE` |
| ai.bootstrap.answer_model / classifier_model | "" | |
| ai.actions.* | see actions.md "Config keys" | agent actions limits, TTLs, kill switch |
| `a.HTTP`.allow_hosts | [] (11-self-host system setting; foundation security.md) | `host:port` entries; needed for Ollama/LM Studio/vLLM on private network (narrower than `http_client.allow_private_networks`, preferred) |

### Entitlement keys (catalog `app/support/entitlements/catalog.go`, contracts.md `Entitlements`)

| Key | Kind | Self-host | Cloud default | Use |
|---|---|---|---|---|
| `ai.workspace_concurrency` | limit | config `ai.workspace_concurrency` (3) | 3 | concurrent `RunDimmiAgent` slots per workspace (Valkey slot leases `0..n-1`, read at job start via `Entitlements.Limit`, cached 60 s); contract customers raised by override (12). Bounds spend cap overshoot (pipeline.md spend cap) |
| `ai.actions.max_connections` | limit | config (unlimited) | 20 | action connections (actions.md) |
| `ai.actions.max_published` | limit | config (unlimited) | 50 | published actions |
| `ai.actions.calls_hourly` | limit (hourly) | config (unlimited) | 2000 | live + test action calls per workspace (egress abuse guard) |

Not entitlements (stay config, abuse protection not contract): `ai.contact_daily_runs`, `ai.conversation_hourly_runs`. AI spend cap is workspace setting on BYOK keys (`ai_agents.monthly_spend_cap_micros`), never a Dimmi cloud cap. No anonymous-visitor AI budget exists in 07 (extensibility.md change list item "07 anonymous AI budget percent if per-contract": n/a).

### Price table format (`resources/ai/prices.yaml`)

```yaml
version: "2026-11-01"
currency: USD
# price per 1M tokens in USD == micro-USD per token
models:
  - provider: anthropic
    match: exact            # exact | prefix
    model: claude-haiku-5-5
    input_per_mtok: "1.00"           # example values only; real numbers maintained by PR
    cached_input_per_mtok: "0.10"
    cache_write_per_mtok: "1.25"
    output_per_mtok: "5.00"
    roles: [classifier]              # suggestions in catalog
  - provider: openrouter
    match: prefix
    model: "anthropic/"
    inherit: { provider: anthropic, strip_prefix: "anthropic/" }
embeddings:
  - provider: openai
    model: text-embedding-3-small
    input_per_mtok: "0.02"
```
Resolution order: key `config.price_overrides` > external file > embedded. Unknown model: cost 0, flag `price_unknown`. Cost math in `big.Rat`, rounded half-up to micros per step. Table version recorded per run. Invalid file at reload: keep previous table, log error, metric `dimmi_ai_price_table_errors_total`, `dimmi doctor` reports.

## Permissions matrix

Implemented as permissions (`00-foundation/extensibility.md` section 4, P2). Policies (`ai_provider_key_policy.go`, `ai_agent_policy.go`, `ai_run_policy.go`) call `authz.Can(principal, permission)`; role columns derived from 00-accounts `roles.go`, never checked in 07 code. Token effective permissions = role permissions intersect expanded abilities (replaces "token owner must be admin"). Agent actor (internal, `Actor{Type: agent}`) bypasses member permissions; scoped by 07 gate rules.

07 permissions (registered in 00-accounts catalog `app/auth/permissions.go`; each in exactly one ability group; 8 incl. agent actions):

| Permission | Ability group | Roles (roles.go) | Gates |
|---|---|---|---|
| `ai:read` | ai:read | owner, admin, member | catalog, keys list (no secrets), agent config, readiness, prices, usage, runs + steps (numbers) |
| `ai_runs:read_payloads` | ai:read | owner, admin | step payloads (redacted prompts) |
| `ai:write` | ai:write | owner, admin, member | playground |
| `ai_agent:manage` | ai:write | owner, admin | agent config, enable/disable, spend cap |
| `ai:keys` | ai:keys | owner, admin | keys create/rotate/delete, validate, probe, refresh models |
| `ai_actions:read` | ai:read | owner, admin, member | action templates, connections (no secrets), actions, versions, calls (no payloads) |
| `ai_actions:manage` | ai:actions (new group) | owner, admin | connections + secrets, actions CRUD, publish, tests, imports, playground live actions, approve `approval: admin` calls |
| `ai_action_calls:approve` | ai:write | owner, admin, member | approve/decline `approval: teammate` calls (`U` guard: session or first-party app only) |

| Action | owner | admin | member | Permission (token ability group) |
|---|---|---|---|---|
| view providers catalog, keys list (no secrets), agent config, readiness, prices, usage | yes | yes | yes | `ai:read` |
| create / rotate / delete keys, re-validate, probe, refresh models | yes | yes | no | `ai:keys` |
| update agent config, enable/disable, set spend cap | yes | yes | no | `ai_agent:manage` (ai:write) |
| playground | yes | yes | yes | `ai:write` |
| view runs + steps (numbers) | yes | yes | yes | `ai:read` |
| view step payloads (redacted prompts) | yes | yes | no | `ai_runs:read_payloads` (ai:read) |
| pause / resume AI on conversation (PATCH `ai_state`) | yes | yes | yes | `conversations:write` (03) |
| widget handoff | contact of conversation | | | widget session |
| view agent actions, connections (no secrets), calls | yes | yes | yes | `ai_actions:read` (ai:read) |
| manage connections + secrets, actions, publish, tests, imports | yes | yes | no | `ai_actions:manage` (ai:actions) |
| approve / decline action call (`approval: teammate`) | yes | yes | yes | `ai_action_calls:approve` (ai:write), `U` guard |
| confirm / cancel action card | contact of conversation, verified session | | | widget session |
Secrets never readable by anyone after save (write-only). Audit log: key create/rotate/delete/config change, agent enable/disable, guidance change (hash), cap change, prompt version change, agent actions entries (actions.md "Permissions").

## Performance budgets

| Metric | Budget |
|---|---|
| pipeline overhead excluding provider + 08 retrieval | p95 120 ms |
| first visible token (widget) | p50 1.5 s, p95 4 s after debounce |
| handoff (button) to `ai.state_changed` broadcast | p95 300 ms |
| key validation | p95 3 s |
| runs list API | p95 50 ms (index `(workspace_id, created_at desc)`) |
| run show with steps | p95 50 ms |
| memory per in-flight run | under 512 KB (stream buffers bounded) |
| delta events | max 10/s per run, each under 1 KB |

## Observability

- Prometheus: `dimmi_ai_runs_total{kind,outcome,channel}`, `dimmi_ai_run_duration_seconds{channel}`, `dimmi_ai_first_token_seconds{channel}`, `dimmi_ai_queue_wait_seconds`, `dimmi_ai_llm_requests_total{provider,step,status}`, `dimmi_ai_llm_errors_total{provider,kind}`, `dimmi_ai_tokens_total{provider,direction}` (direction: input, cached_input, output), `dimmi_ai_cost_micros_total{provider}`, `dimmi_ai_breaker_open{provider}`, `dimmi_ai_spend_cap_reached_total`, `dimmi_ai_price_table_errors_total`, `dimmi_ai_key_decrypts_total`. No `model` or workspace label (cardinality); model breakdown via usage API.
- Logs: one structured line per run end: run id, workspace, conversation, outcome, skip/handoff reason, provider, model, tokens, cost, latency, error kind, provider request id. Never prompt text, never customer text, never key (redaction middleware + typed credential `LogValue`).
- OTel spans: `ai.run` with child spans per step (`ai.classify`, `knowledge.search`, `ai.generate`), attributes = numbers + model only.
- Traces API (above) is debugging UI source. `dimmi doctor`: price table valid, prompts valid, keys status summary, `a.HTTP` allow entries for configured private base URLs.

## Security and privacy notes

- Keys: `encryption.Sealed` column (`encryption.Seal/Open[ProviderCredentials]`) via `encryption.KeyManager` envelope (per-workspace data key). Write-only API. `key_hint` only. Decrypted only inside worker/request goroutine memory. Crypto-shred on delete (column nulled). Extra header values for compat endpoints stored inside encrypted blob.
- Outbound: all LLM calls via `a.HTTP` (framework httpclient) (SSRF guard). Base URL validated on save and at every connect (DNS rebinding). Cloud: private networks never allowed. Self-host: explicit `http_client.allow_hosts`.
- BYOK boundary: data only to keys in workspace. No Dimmi-owned fallback key. Fallback model must be workspace key. Self-host env bootstrap creates visible row.
- Prompt injection, PII, links, content safety: [pipeline.md](pipeline.md).
- AI output treated untrusted: Markdown safe render in widget/dashboard/email (foundation rule).
- Notes never sent to provider. Contact attributes excluded unless allowlisted; allowlisted values and contact name go in user-slot `<contact_context>` data block, never system blocks (unverified leads set them).
- Spoofed or unverified email sender (04 `metadata.ai_skip`): AI never answers (would leak verified contact context and history to attacker via Reply-To).
- Cross-tenant tests: other workspace's key id, run id, conversation id return 404; retrieval cannot cross workspace.
- Abuse (cloud): per-contact and per-conversation AI rate limits stop token-drain attacks via widget; widget bot checks (01) run before trigger; spend cap default on.
- Data deletion: workspace delete cascades; contact erase purges payloads.
- Agent actions (actions.md "Injection defenses and safety"): identity variables only from verified context, writes only after customer click on Dimmi-rendered card (+ optional teammate approval), action results untrusted data, connection secrets Sealed + write-only, SSRF guard + no redirects + own host blocked, MCP never executes or publishes write actions.

## Metering impact (pricing.md section 2)

- D6: AI reply with `substantive=true` (outcome `answered`) activates billable conversation exactly like human reply. `RecordBillableActivity` (12) already listens to `message.created` substantive replies; agent messages have `author_type=agent`. No AI surcharge.
- D2: greeting, unknown notice, handoff notice, refusals, safety message, cap notice: `substantive=false`. Bot failures never bill. Silent outcome: no message, no bill.
- D10: playground runs create no messages, never bill. Widget test-mode conversations follow 12's rule (free).
- D9: AI skips spam/blocked contacts before reply, so no activation from bot on spam.
- D13: failed delivery of AI email reply credited by 12 like human replies.
- Agent actions: calls never metered (no procedure-handoff fee); confirmation, approval, decline, expiry notices non-substantive; follow-up answer after write follows D6.
- AI spend (provider tokens) is separate from Dimmi bill; shown side by side in usage API (`ai.cost_usd` vs Dimmi usage from 12). Comparison line "Fin would charge" computed client-side from `answered_count` (not stored).

## Dependencies on other specs

| Spec | What 07 needs | Contract |
|---|---|---|
| 18-audiences | surfaces `dimmi.knowledge`, `dimmi.action`, evaluated every run in `access` mode (verified values only) | `app/support/audience` |
| 08-help-center | retrieval API, knowledge readiness, embedding key references, help center hosts for link allowlist, Ask AI reuse of `AnswerQuestion` | see "Retrieval contract" below |
| 01-messenger-widget | render `ai.reply_*` deltas, citations chips, persistent "Talk to a human" button while `ai_state=active`, `metadata.ai.actions`, email capture on `request_email`; registers `POST /widget/v1/conversations/{conversation}/handoffs` (`widget.conversations.handoffs.store`) in widget route group; controller + action (`RequestHumanHandoff`) owned by 07; widget channel must carry `ai.reply_started`, `ai.reply_delta`, `ai.reply_discarded`, `ai.state_changed` (01 list lacks them today) | widget-safe message resource includes `metadata.ai.citations`, `actions` |
| 03-shared-inbox | `ReplyToConversation` (actor agent, `Substantive`), `AssignConversation`, `UpdateConversation` (priority), `ReopenConversation` (reason `manual`, actor agent), `UnsnoozeConversation` callable with actor `agent`; message resource shows agent author name/avatar; `PATCH /conversations/{conversation}` accepts `ai_state` and hands it to 07 `PauseConversationAi`/`ResumeConversationAi` in same tx (03 never writes column) | actor type `agent` |
| 04-email-channel | sets `AutoGenerated` (03 stores `substantive=false`) and `messages.metadata.auto_reply=true` on inbound auto-replies; sets `metadata.ai_skip` (`unverified_sender`, `non_participant`, `throttled`, `recipient_rate`, `no_reply_address`); render citations footer from `metadata.ai.citations`; From name "{agent name} at {workspace}" for agent messages | gate rules 6, 6b |
| 05-chat-app-channels | outbound for agent messages, citations footer per platform format, typing indicator capability while generating | |
| 06-notifications | deliver handoff (with summary), spend threshold/cap, key failed notifications; may call `ai.WouldAttempt` to suppress `new_conversation` during debounce (06 asks for `ai_state=active` in creation tx; 07 sets it only at gate after debounce, so 06 re-checks state at fan-out) | notification classes in 07 |
| 10-webhooks | catalog entries for new webhook events | |
| 12-cloud-billing | `RecordBillableActivity` treats `author_type=agent` substantive messages as replies; `PreflightActivation` (cloud) called in gate before any LLM call; `billing.activations_resumed` event | skip reason `billing_paused` |
| 13-changelog | calls `ai.RunTask` (kind `changelog`), `linkguard.Filter`; changelog chunks via 08 knowledge (published only, `published_at`) | `RunTask` |
| 14-feedback-roadmap | calls `ai.RunTask` (kind `feedback`, purposes capture/duplicate; per-post AI summary later: `feedback-ai-summary`); provides `feedback.QueueCapture(conversationID, messageID)` (dedupe by message) and workspace setting `ai.capture` | `RunTask`, intent `feedback` hook |
| 08-help-center docs sync | calls `ai.RunTask` (kind `docs_sync`, purposes `screen_change`, `draft_edit`, `draft_article`, `verify_article`; `PersistInput` false: customer code never in `ai_run_steps.request`); prompts owned by 08 under `resources/prompts/docs_sync/` | `RunTask` |
| 16-importers | imported messages (`metadata.imported`) never trigger agent (gate 6d); imported conversations start `ai_state = 'off'`; imported messages included as prompt history (author labels `former teammate`, `bot`); imported bot messages never counted in `ai_reply_count` or spend; imported custom attributes untrusted unless admin marks mapping `trusted` (actions.md) | gate |
| 15-announcements-surveys | agent never triggered by system messages (`author_type system`, gate); no AI helper use at MVP (later: `survey-ai-summary`) | gate |
| agent actions (01, 02, 03, 06, 09, 10, 11, 12, 16, foundation) | confirmation card + identity stamp (01), trusted attribute keys (02, 03, 16, foundation), timeline helpers (03), approval notifications (06), MCP toolset + CLI (09), webhook catalog (10), no meter (12) | actions.md "Required from other specs" |
| 00-accounts | new abilities `ai:read`, `ai:write`, `ai:keys`; `authz.Can` + permission catalog (07 registers 5 permissions), roles.go; audit log action names `ai.*` | |
| 00-foundation + framework | ID prefixes `aia_`, `aik_`, `air_`, `ais_`; events catalog rows; broadcasting publish without history for `ai.reply_delta` (no recovery replay); Valkey (`pubsub`, `cache`, `ratelimit`, slot leases); `httpclient` `StdClient()` for SDKs; registry enum `ai_provider_keys.provider` (00-conventions registry list); `http_client.allow_hosts` key (11 setting); `Entitlements` contract + catalog (07 registers `ai.workspace_concurrency`); `RetentionGuard`; registry enum `channel_connections.type` (`ai_agents.enabled_channels` values validated against it, not hardcoded) | |

### Retrieval contract (08 implements, 07 consumes)

Canonical: `contracts.Knowledge` in `app/contracts/knowledge.go`, defined in `specs/00-foundation/contracts.md` (wins over this file). Types `KnowledgeSearchOptions`, `KnowledgeSearchResult`, `KnowledgeChunk`, `KnowledgeDocument`, `KnowledgeStatus`, `KnowledgeVisibility`. 07 uses `Search`, `Document` (citation expansion), `Status` (readiness gate on enable), `EmbeddingKeyInUse` (key delete), `PublicHosts` (link allowlist). Tests: `knowledge.Fake()`.

07-specific requirements on 08:
- Agent calls pass `Visibility: [public, agent_only]`, `Surface: agent`, `LogSearch: false` (07 logs in `ai_run_steps`).
- `AnswerQuestion` kind `help_center` (08 Ask AI, anonymous public visitors) hard-codes `Visibility: [public]`, `Surface: ask_ai`, `HelpCenterIDs: [that help center]`, `ExcludeSources: true` inside 07, ignoring caller input. `search_knowledge` tool in that run inherits same options. agent_only text never reaches public answer (leak test in tasks).
- Workspace id for every `Search` call comes from run row, never from model tool arguments or request body.
- Threshold on `KnowledgeChunk.Similarity` (cosine 0..1, nil when FTS-only hit) so 07 threshold query-independent. `Score` used only for FTS-only ranking.
- Cost attribution: `KnowledgeSearchResult.EmbeddingUsage` + `EmbeddingKeyID` recorded on `retrieve` step and added to run cost (counts toward spend cap).
- Query scoped by `workspace_id` (and RLS); p95 under 150 ms end to end for workspaces up to 50,000 chunks (contract: 100 ms excluding query embedding).
- Embedding uses workspace key from `ai_provider_keys` resolved through 07 resolver `ai.ResolveProviderKey(ctx, workspaceID, keyID) (ProviderEndpoint, error)` (decrypted credentials + base URL + region + `a.HTTP` client; never persisted by 08). Key storage single-owned; 08 stores only `embedding_key_id` + `embedding_model` in own settings.
- 08 records own ingestion embedding spend into `ai_spend_periods` via 07 action `RecordAiSpend` (kind `embedding`).

## Open questions (recommended default in bold)

1. Office hours ownership: decided. 03 defines no office hours at MVP (rules + office hours: post-MVP: rules lite), so 07 keeps `ai_agents.office_hours` for MVP. Moves to 03 workspace-level setting post-MVP; 07 reads it then. 01 widget uses free-text `office_hours_text` at MVP (no schedule computation), so no sharing needed yet.
2. Default monthly AI cap value: **$25** (cheap enough to never shock, high enough for ~5,000 Sonnet-class answers).
3. Spend period in UTC month vs workspace timezone month: **UTC** (matches provider invoices).
4. Greeting reply on first "hi" in widget: **reply, non-substantive**. Productlane stays silent; we judge empty-feeling widget worse.
5. Should "knowledge empty" hard-block enabling: **yes**.
6. `RecordAiSpend` for 08 ingestion embeddings counts toward AI cap: **yes, same money**. Cap reached blocks ingestion embeddings too (08 falls back to FTS-only indexing and retries next period).

## Provider sources delta (2026-11-06, decided)

BYOK is no longer the only way in. A connection (`ai_provider_keys`) now covers three sources:

| Source | provider | auth_type | Notes |
|---|---|---|---|
| Managed dimmi | `dimmi` | `managed` | Default for every workspace. No secret on the row; upstream creds live in instance config `ai.managed.{anthropic,openai,openweight}`. Driver multiplexes model -> family -> upstream driver. Cloud: Dimmi's keys (billed at cost, invoicing rides 12). Self-host: operator's keys, else provider hidden. |
| ChatGPT subscription | `chatgpt` | `oauth` | Sealed OAuth token set; refreshes + re-seals on rotation. Official Sign-in-with-ChatGPT Responses API at `api.openai.com/v1/responses` only (never `backend-api/codex`). Self-host: dynamic client registration; cloud: interest-form client, flag `ai.chatgpt.enabled`. |
| BYOK | `anthropic`, `openai`, `engy`, `openai_compatible` | `api_key`/`none` | Launch set. engy = OpenAI-compatible at api.engy.ai; reports real cost via `X-Engy-Charged-Micro`. gemini/bedrock/openrouter/voyage deferred; driver registry stays `Extend`-able. |

Prompt files gain a family layer: `resources/prompts/agent/<v>/{common,anthropic,openai,openweight}/`, per-file override falling back to `common`. `ai_runs.config_snapshot` records `model_family`. Usage API reports `by_source`.
