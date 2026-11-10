# 07 Dimmi Agent: LLM contract and drivers

Style: caveman. Owner: 07-dimmi-agent. Read with `README.md`. Provider facts verified 2026-11-01 (sources at bottom).

## Goals

- One small Go interface every provider driver implements. Agent pipeline never imports provider SDK.
- Streaming, tool calling, token usage (incl. cache tokens), typed errors, retries, timeouts, cancellation.
- Every driver passes same contract test suite against recorded fixtures. `go test ./...` never touches network, never calls real LLM.
- Drivers wrap official SDKs (openai-go/v3 for OpenAI, OpenRouter, OpenAI-compatible incl. Ollama; anthropic-sdk-go; google genai; aws-sdk-go-v2 bedrockruntime). No hand-written wire protocol, SSE reader or tool-delta assembly.

## Contract: `app/contracts/llm.go`

```go
package contracts

// LLM is one configured provider key bound to one base URL / region.
// Built per run by managers.LLMManager from ai_provider_keys row. Safe for concurrent use.
type LLM interface {
    Provider() LLMProvider // "openai", "anthropic", "gemini", "openrouter", "bedrock", "openai_compatible"

    // Chat runs one non-streaming completion.
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

    // Stream runs one streaming completion. Caller must Close.
    // Errors before first event come back from Stream; mid-stream errors from Next.
    Stream(ctx context.Context, req ChatRequest) (ChatStream, error)

    // ListModels returns models key can call. Used for validation and model pickers.
    // Returns ErrLLMNotSupported when provider has no list endpoint (caller falls back to Probe).
    ListModels(ctx context.Context) ([]LLMModel, error)

    // Probe sends smallest possible request (max 8 output tokens) to model.
    // Used by "Test model" button and validation fallback. Real cost, tiny.
    Probe(ctx context.Context, model string) (*ProbeResult, error)

    // Capabilities for model id: tools, streaming, system prompt, json schema, prompt cache, max context.
    // Static table per driver + overrides from key config. Never network.
    Capabilities(model string) LLMCapabilities
}

type ChatRequest struct {
    Model           string
    System          []SystemBlock   // static first (cacheable), dynamic last
    Messages        []ChatMessage   // role user | assistant | tool
    Tools           []ToolSpec      // empty when model lacks tool support
    ToolChoice      ToolChoice      // auto (default) | none. Never forced (some 2026 models reject forced tool use)
    MaxOutputTokens int             // required, > 0
    Temperature     *float64        // nil = provider default
    StopSequences   []string
    Schema          *ResponseSchema // {Name, Schema}: provider-enforced strict JSON Schema; Text is the one object. Unsupported: ErrSchemaUnsupported, never a prompt fallback
    CacheHint       bool            // driver adds cache markers where supported (Anthropic cache_control, Bedrock cachePoint)
    Metadata        map[string]string // run id, step id. Sent only where provider supports opaque metadata; never PII
    Timeout         StepTimeouts    // FirstToken, Idle, Total
}

type SystemBlock struct { Text string; Cacheable bool }

type ChatMessage struct {
    Role       ChatRole           // user | assistant | tool
    Parts      []ContentPart      // text only at MVP (images post-MVP)
    ToolCalls  []ToolCall         // assistant only
    ToolCallID string             // tool only
    ToolError  bool               // tool only: result is error (Anthropic `is_error`, Bedrock `status: error`, Gemini/OpenAI: text prefixed "ERROR: ")
}

type ToolSpec struct {
    Name        string            // ^[a-z][a-z0-9_]{0,47}$ (agent action names, actions.md; valid for every provider's tool-name rules)
    Description string            // max 1,100 chars (action description + Dimmi suffix)
    Schema      json.RawMessage   // JSON Schema object, additionalProperties false; portable subset (actions.md "Input schema"): drivers map
                                  // as-is (OpenAI `parameters`, Anthropic `input_schema`, Bedrock `toolSpec.inputSchema.json`,
                                  // Gemini `ParametersJsonSchema`); no driver-side schema rewriting beyond dropping `examples`/`title` where rejected
}

type ToolCall struct { ID, Name string; Arguments json.RawMessage }

type ChatResponse struct {
    Text         string
    ToolCalls    []ToolCall
    StopReason   StopReason       // end | max_tokens | tool_use | stop_sequence | content_filter | other
    Usage        LLMUsage
    Model        string           // model provider reports (OpenRouter may route to concrete model)
    RequestID    string           // provider request id header for support tickets
    Latency      time.Duration
}

type ChatStream interface {
    // Next blocks until next event. io.EOF after final EventDone.
    Next(ctx context.Context) (StreamEvent, error)
    Close() error
}

type StreamEvent struct {
    Kind      StreamEventKind     // text_delta | tool_call_delta | tool_call_done | usage | done
    Text      string
    ToolCall  *ToolCall           // on tool_call_done, arguments complete
    Usage     *LLMUsage           // on usage / done
    StopReason StopReason         // on done
}

type LLMUsage struct {
    InputTokens       int64 // uncached prompt tokens
    CachedInputTokens int64 // prompt tokens served from cache (read)
    CacheWriteTokens  int64 // prompt tokens written to cache
    OutputTokens      int64 // includes reasoning tokens
    ReasoningTokens   int64 // informational subset of OutputTokens
    Reported          bool  // false when provider sent no usage; pipeline then estimates (see Estimation)
}

type LLMModel struct { ID, DisplayName string; ContextTokens int; Deprecated bool }
type ProbeResult struct { Model string; Latency time.Duration; Usage LLMUsage }

type LLMCapabilities struct {
    Tools, Streaming, SystemPrompt, JSONSchema, PromptCache bool
    ContextTokens int
    MaxOutputTokens int
}
```

### Typed errors (`app/contracts/llm_errors.go`)

Drivers map provider responses to one `*LLMError{Kind, HTTPStatus, ProviderCode, RetryAfter, RequestID, Message}`. `Message` never contains key or prompt.

| Kind | Retry? | Maps from |
|---|---|---|
| `auth` | no; key marked `invalid` | 401 all providers; Bedrock `AccessDeniedException` on credentials; OpenAI "Incorrect API key" |
| `permission` | no; key stays valid, model flagged unavailable | 403 Anthropic `permission_error`; OpenAI 403 region; Bedrock 403 model access not granted; Gemini `PERMISSION_DENIED` |
| `quota` | no; key marked `error` with reason `quota` | OpenAI 429 with `error.code` in (`insufficient_quota`, `credit_balance_exhausted`, `*_spend_limit_exceeded`, `organization_usage_limit_exceeded`); Anthropic 402 `billing_error`, 400 spend-limit, 429 tier spend cap (no `retry-after`); OpenRouter 402 `openrouter_credits` / `openrouter_key_limit`, 403 budget |
| `rate_limited` | yes, honor `Retry-After` | 429 (non-quota) all; Bedrock `ThrottlingException`; Gemini `RESOURCE_EXHAUSTED`; OpenRouter 402 `openrouter_in_flight_budget` |
| `overloaded` | yes, backoff; fallback model allowed | Anthropic 529 `overloaded_error`; OpenAI 503 `server_is_overloaded`; Bedrock 503 `ServiceUnavailableException`, `ModelNotReadyException` (429) |
| `server` | yes | 500, 502, 504; Bedrock `InternalServerException`, `ModelStreamErrorException` (424) |
| `timeout` | yes once | our timeouts (first token, idle, total), Bedrock `ModelTimeoutException` (408) |
| `invalid_request` | no | 400/422 (bad model id, context too long, schema) |
| `model_not_found` | no; agent readiness flags model | 404 model, Bedrock `ResourceNotFoundException` |
| `content_filtered` | no; pipeline uses refusal path | Gemini `SAFETY` finish, OpenAI `content_filter` finish, Bedrock guardrail intervene |
| `network` | yes | DNS, TLS, reset, `a.HTTP` block (block is `blocked_host`, not retried) |
| `blocked_host` | no | `a.HTTP` refused private/metadata address |
| `canceled` | no | ctx canceled (superseded run, member reply) |

Rule: retry layer lives in `app/support/llmretry` (wraps any `contracts.LLM`; backoff `cenkalti/backoff/v5`, breaker `sony/gobreaker/v2`), not in drivers. Drivers make exactly one attempt. SDK built-in retries disabled (`option.WithMaxRetries(0)` for anthropic-sdk-go and openai-go, genai retry off, AWS `retry.AddWithMaxAttempts(..., 1)`).

## Manager: `app/managers/llm_manager.go`

```go
func (m *LLMManager) ForKey(ctx context.Context, key models.AiProviderKey) (contracts.LLM, error)
```
- Decrypts `credentials_encrypted` via `encryption.KeyManager` in memory, builds driver (official SDK client) with `a.HTTP.StdClient()` (per-key transport, connect 5s, TLS 5s, response header 30s, body streamed), drops plaintext after driver construction (driver holds it in unexported field; never logged; `String()`/`GoString()`/`LogValue()` on credential struct return `[redacted]`).
- Driver cache: LRU 256 entries keyed `(key_id, key.updated_at)`; rotation changes `updated_at` so stale driver never reused.
- Every decrypt counted (`dimmi_ai_key_decrypts_total`); cloud audits decrypt per run batch (one audit row per key per hour, not per call, to keep audit_logs small).
- Fake: `llm.Fake()` returns `*FakeLLM` scripted with responses, streams, errors, delays. Tests assert requests (`fake.AssertSent(t, func(req ChatRequest) bool)`).

## Retry and fallback wrapper (`app/support/llmretry`)

| Setting | Default | Notes |
|---|---|---|
| max attempts per step | 3 (1 + 2 retries) | only kinds marked retry |
| backoff | 400 ms, 1.2 s (full jitter) | `Retry-After` honored when <= remaining step budget |
| step budget widget | 12 s to first token | exceed: give up step, fallback model or handoff |
| step budget email/chat apps | 45 s | not latency sensitive |
| retry after first streamed token | never | partial answer discarded (`ai.reply_discarded`), whole generate step restarts at most once |
| fallback | `ai_agents.fallback_key_id` + `fallback_model` | used only for `rate_limited`, `overloaded`, `server`, `timeout` after retries exhausted. Never for `auth`, `quota`, `permission`, `invalid_request` |
| circuit breaker | 5 consecutive retryable failures within 60 s per key+model | open 60 s; calls fail fast with `overloaded` so pipeline goes straight to fallback; half-open after 60 s with 1 trial |

Breaker state in-process per replica (`gobreaker.TwoStepCircuitBreaker` per key+model, LRU otter). Good enough: worst case each replica learns separately (not Valkey: breaker is per-process health signal, cheap to relearn).

No silent fallback to anything workspace did not configure. Fallback key must belong to same workspace. Dimmi never owns a key that customer data could reach (cloud).

## Timeouts

| Step | First token | Idle between events | Total |
|---|---|---|---|
| classify (non-stream) | n/a | n/a | 8 s |
| generate (stream) | 12 s widget / 30 s other | 15 s | 60 s widget / 90 s other |
| verify, summarize, translate | n/a | n/a | 10 s |
| validation `ListModels` | n/a | n/a | 10 s |
| probe | n/a | n/a | 20 s |

Every outbound call also bound by job context (River job timeout 120 s).

## Drivers

All in `app/drivers/llm/<driver>/` (adapter zone, framework.md 7.3; only place official provider SDKs are imported). Each driver = thin wrapper over the provider's official Go SDK (research/build-vs-reuse.md 3.8): maps `ChatRequest` to SDK params, SDK stream events to `StreamEvent`, SDK errors to `LLMError`, usage to `LLMUsage`. Every SDK gets `a.HTTP.StdClient()` (SSRF guard, per-key transport, connect 5 s, TLS 5 s, response header 30 s, body streamed) and SDK retries off (one attempt per driver call; `llmretry` owns retries). All implement `Capabilities` from static table `capabilities.go` plus key config overrides.

| Driver | SDK (pinned) | Wire API | Auth | Default base URL | Validation (`ListModels`) | Notes |
|---|---|---|---|---|---|---|
| `openai` | `github.com/openai/openai-go/v3` v3.91.x (`option.WithHTTPClient`, `option.WithMaxRetries(0)`, `option.WithBaseURL`) | Chat Completions, SDK streaming with `stream_options.include_usage=true`; deltas collected by SDK `ChatCompletionAccumulator` | `Authorization: Bearer` (SDK), `option.WithOrganization` / `WithProject` from key config | `https://api.openai.com/v1` | SDK `Models.List` (free) | sends `max_completion_tokens` (reasoning models reject `max_tokens`); cached tokens from `usage.prompt_tokens_details.cached_tokens` (may be absent); reasoning from `completion_tokens_details.reasoning_tokens`. Responses API migration post-MVP behind same contract |
| `anthropic` | `github.com/anthropics/anthropic-sdk-go` v1.84.x (`option.WithHTTPClient`, `option.WithMaxRetries(0)`) | Messages, SDK streaming; `Message.Accumulate` builds final message incl. tool `input_json_delta` | `x-api-key` + `anthropic-version` (SDK; version pinned in config `ai.anthropic.version` via `option.WithHeader`) | `https://api.anthropic.com` | SDK `Models.ListAutoPaging` | system as top-level blocks with `cache_control: {type: ephemeral}` on cacheable block; usage `input_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens`, `output_tokens`; `request-id` captured from response; forced `tool_choice` only for `Schema` requests (one strict tool named after the schema; its input returned as `Text`); never sends assistant prefill (newer models reject); thinking param omitted (provider default) unless key config sets it |
| `gemini` | `google.golang.org/genai` v1.77.x (`ClientConfig.HTTPClient`, backend Gemini API) | `GenerateContentStream` / `GenerateContent` | API key in `x-goog-api-key` header (SDK; never `?key=` query) | `https://generativelanguage.googleapis.com` | SDK `Models.All` (paginated) | `SystemInstruction`; tools as `FunctionDeclarations` (whole `FunctionCall` per chunk, no delta assembly); usage from `UsageMetadata` (`PromptTokenCount`, `CachedContentTokenCount`, `CandidatesTokenCount`, `ThoughtsTokenCount`); `FinishReason SAFETY` maps `content_filtered`. SDK retry options disabled [verify option name at pin] |
| `openrouter` | openai-go/v3 with `option.WithBaseURL("https://openrouter.ai/api/v1")` | OpenAI-compatible chat completions | `Authorization: Bearer`; `option.WithHeader` `HTTP-Referer: <app.url>`, `X-Title: Dimmi` (attribution, no customer data) | `https://openrouter.ai/api/v1` | `GET /api/v1/key` via `a.HTTP` (valid key, `limit_remaining`, `usage_monthly`, `is_free_tier`) + SDK `Models.List` | 402 / 403 mapped via `error.metadata.limit_source` (raw error body from SDK `*openai.Error`); key credit info in key resource (`provider_info`); response `model` recorded (concrete routed model) for pricing |
| `bedrock` | `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` (+ `bedrock` control plane) | `ConverseStream` / `Converse` (SDK event-stream decoding) | IAM access key id + secret (+ optional session token) with SigV4, or Bedrock API key as `Authorization: Bearer` | region endpoint `bedrock-runtime.<region>.amazonaws.com` | `ListFoundationModels` (needs `bedrock:ListFoundationModels`); on `AccessDeniedException` fall back to `Probe` on chosen model (needs `bedrock:InvokeModel`) | model id may be inference profile id (`us.anthropic...`) or ARN; usage from `metadata.usage` (`inputTokens`, `outputTokens`, `cacheReadInputTokens`, `cacheWriteInputTokens`); `cachePoint` system block when model supports; `toolUse` input deltas joined by driver (SDK gives string fragments per content block index; only remaining assembly code, about 30 lines); AWS SDK retries disabled (`retry.AddWithMaxAttempts(..., 1)`); HTTP client = `a.HTTP.StdClient()`. Long-term Bedrock API keys allowed but UI warns AWS calls them exploration-only |
| `openaicompat` | openai-go/v3 with user base URL (Ollama, LM Studio, vLLM, any HTTPS gateway) | `POST {base}/chat/completions` via SDK | `Authorization: Bearer` (optional; Ollama/LM Studio ignore) + up to 5 extra headers from key config via `option.WithHeader` (names allowlisted `^[A-Za-z0-9-]{1,64}$`, never `Host`, `Cookie`) | required: user enters (`http://localhost:11434/v1` Ollama, `http://localhost:1234/v1` LM Studio, `http://localhost:8000/v1` vLLM) | SDK `Models.List`; 404/405 falls back to `Probe` | key config: `max_tokens_field` (`max_tokens` default, `max_completion_tokens`; set via `option.WithJSONSet` when needed), `supports_tools` (auto from probe: default false until tool probe passes), `supports_stream_usage` (default true; stream ends without usage chunk: `Reported=false`), `json_schema` (strict `response_format` json_schema when the server enforces it; default off). Private hosts need `http_client.allow_hosts` entry (README security). Compat server that breaks SDK stream parser: driver returns `invalid_request` `incompatible_server` with fix hint (non-stream fallback via `supports_stream=false` key config); no own SSE reader |

`voyage`: embeddings-only key type (no row in table above). No `LLM` driver: `managers.LLMManager` returns `invalid_request` for voyage key; agent roles reject it (`provider_key_embeddings_only`). Embedding driver `embeddings/voyage` owned by 08 (`specs/08-help-center/knowledge.md`). Key validation: no list endpoint; 07 validates via 08 embeddings driver tiny embed (`"dimmi"`, `input_type=query`); auth `Authorization: Bearer`; `models` cached from 08 static embedding model table. Catalog lists voyage under embeddings only.

Azure OpenAI: post-MVP driver (pricing.md lists it). Workaround at MVP: Azure exposes OpenAI-compatible v1 endpoint; user adds as `openai_compatible` with `api-key` extra header. Documented, not tested in CI.

### Streaming and tool calls (SDK-owned)

- No hand-written SSE reader, no `app/drivers/llm/internal/sse` (removed; framework.md 10.1 row `httpclient.SSE` not used by 07). SDKs parse SSE (`[DONE]`, keep-alive comments, Anthropic mid-stream `error` event) and Bedrock event-stream. Driver keeps only: per-event size guard (event over 4 MB: `invalid_request` `oversized_event`, stream closed), idle timer between events (step timeouts), partial UTF-8 buffering for text deltas.
- Tool-call delta assembly done by SDK accumulators (openai-go `ChatCompletionAccumulator.JustFinishedToolCall`, anthropic-sdk-go `Message.Accumulate`); Gemini sends whole `FunctionCall`; Bedrock joined by driver (above). Driver emits `tool_call_done` only when arguments parse as JSON object. Invalid JSON after stream end: `LLMError{Kind: invalid_request, ProviderCode: "malformed_tool_arguments"}`; pipeline treats as tool error result fed back once, then gives up tool.
- Error mapping reads SDK typed errors (`*openai.Error`, `*anthropic.Error`, `genai.APIError`, smithy `APIError`) for status, code, `Retry-After`, request id.

### Usage estimation

When `Reported=false` (some compat servers), pipeline estimates: `ceil(utf8 runes / 3.5)` per message for input, same for output, flags step `usage_estimated=true`. Cost from estimate shown with "~" in API (`cost_estimated: true`).

## Contract test suite (`tests/contracts/llm_test.go`)

Every driver runs same table against recorded fixtures from `tests/fixtures/llm/<driver>/*.http|*.sse` served through `httpclienttest.New(a.HTTP)` (SDKs share the faked `StdClient`; base URL pointed at fixture host); Bedrock: SDK stubbed via `smithy` middleware returning recorded event-stream frames. Suite proves SDK wrapping, not SDK internals.

Cases each driver must pass:
1. Chat text answer: text, stop reason `end`, usage mapped (incl. cached tokens where fixture has them).
2. Stream text: deltas concatenate to expected text; usage arrives; `io.EOF` after done.
3. Stream tool call: one tool call assembled with valid JSON args.
4. Two parallel tool calls in one turn (where provider supports).
5. 401 maps `auth`; 429 with `Retry-After: 3` maps `rate_limited` with `RetryAfter=3s`; quota 429/402 maps `quota`; 529/503 maps `overloaded`; 500 maps `server`.
6. Mid-stream error event maps typed error from `Next`.
7. Context cancel mid-stream: `Next` returns `canceled` within 100 ms; connection closed.
8. Idle timeout: fixture pauses 20 s (fake clock + slow reader); returns `timeout`.
9. Oversized event rejected without OOM (driver guard over SDK stream).
10. `ListModels` parses list and pagination.
11. Request body golden test: system blocks, cache markers, tool schema, max tokens field name.
12. Key never appears in error string, `fmt.Sprintf("%+v")` of driver, or slog output (log capture assertion).
13. Unicode: emoji and RTL text round trip intact across delta boundaries splitting multi-byte runes (driver must buffer partial UTF-8).
14. Tool round trip (agent actions): request with 3 tools (portable-subset schemas incl. `enum`, `maximum`, nested object, array) golden per provider; assistant tool call + `tool` message with `ToolError = true` mapped to provider error shape; 20-tool request accepted.

Fixture recording: `dimmi ai:fixtures:record <driver>` dev-only command (build tag `fixtures`, not compiled in release) records sanitized fixtures using developer's own key. Never run in CI.

## Sources

- Anthropic errors (status codes, 529, request-id, mid-stream errors): https://platform.claude.com/docs/en/api/errors
- Anthropic list models: https://platform.claude.com/docs/en/api/models-list
- OpenAI error codes (429 quota vs rate limit, 503 overloaded): https://developers.openai.com/api/docs/guides/error-codes
- Gemini OpenAI compatibility: https://ai.google.dev/gemini-api/docs/openai
- OpenRouter key and limits (`/api/v1/key`, 402 `limit_source`): https://openrouter.ai/docs/api-reference/limits
- Bedrock ConverseStream (events, usage, errors): https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ConverseStream.html
- Bedrock API keys (bearer, short vs long term): https://docs.aws.amazon.com/bedrock/latest/userguide/api-keys.html
- Ollama OpenAI compatibility (`/v1/models`, tools, `stream_options.include_usage`): https://docs.ollama.com/api/openai-compatibility
