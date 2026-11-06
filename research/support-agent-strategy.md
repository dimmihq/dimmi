# Support agent strategy: what the leaders do and how dimmi wins

Research date: 2026-11-06. Builds on `research/competitors/fin.md` and `specs/07-dimmi-agent`. Sources are linked inline. Most vendor numbers are self-reported; treat them as claims, not measurements.

## 1. The short version

- Every serious support agent in 2026 has the same core: grounded answers with citations, natural-language rules, procedures with deterministic steps, authenticated actions, handoff with a summary, a test harness, and analytics that point at what to fix.
- Quality comes from the system around the model, not the model. Fin went from about 30% to about 70% resolution mostly through query rewriting, retrieval and reranking ([Chain of Thought, Fergal Reid](https://chainofthought.transistor.fm/episodes/how-intercom-cut-250k-month-by-ditching-gpt-for-qwen)).
- The market's weak spot is trust: bills charged on customer silence, "resolution" definitions the vendor changes, bots that won't hand over, and no way to see why the agent did what it did.
- dimmi wins by being the agent you can see inside and prove works: bring your own model, every step traced, resolution defined honestly, tests that run in CI, a human always reachable, and everything included in the open edition.

## 2. What the leaders do

### Fin (Intercom, now Salesforce)

- **Pipeline:** refine (safety filter, rewrite the conversation into a standalone query), retrieve (top 40 passages with their own fine-tuned multilingual embedder), rerank (own ModernBERT cross-encoder, replaced Cohere, 80% cheaper, p50 about 150 ms), generate, validate (does the reply answer the question and rest on sources; otherwise clarify or escalate) ([Fin AI Engine](https://fin.ai/help/en/articles/13975771-the-fin-ai-engine), [retrieval](https://fin.ai/research/finetuning-retrieval-for-fin/), [reranker](https://fin.ai/research/how-we-built-a-world-class-reranker-for-fin/)).
- **Own models:** Apex 1.0 and 2.0, post-trained on real resolution outcomes; claims 65% fewer hallucinations than Sonnet 4.6 at a fifth of the cost. Not available outside Fin.
- **Steering:** Guidance (short if/when/then rules, targeted by audience, channel and schedule, versioned, with per-rule stats). Since Sep 2026 a small model picks which rules, connectors and procedures apply to each message instead of putting all of them in the prompt ([research](https://fin.ai/research/how-fin-scales-customer-defined-behaviour)).
- **Procedures:** natural-language SOPs with conditions (plain language or Python), Python code blocks, HTTP and MCP actions, sub-procedures, ask-a-teammate approval, wait-for-webhook. Fin picks a procedure by intent and can skip or revisit steps when the customer changes topic. Skills (Fin 4) cover open-ended troubleshooting.
- **Escalation:** a small classifier (escalate now / offer / keep answering, plus the reason), 97.4% accurate, 0.5 s faster than the LLM router it replaced ([research](https://fin.ai/research/to-escalate-or-not-to-escalate-that-is-the-question/)).
- **Quality loop:** Simulations (an AI customer with a persona, pass/fail against written criteria, mocked tools, a saved suite), Topics (clustered questions with emerging-topic detection), CX Score (every conversation rated 1-5 with reasons), Monitors (custom QA scorecards for AI and humans), content suggestions drafted from teammates' replies.
- **Numbers:** 76% average resolution claimed across 12,000+ customers. Part of the recent rise is a July 2026 change that removed "constrained" conversations from the denominator ([Intercom](https://www.intercom.com/help/en/articles/15599377-update-to-fin-performance-metrics)).
- **Weak spots:** $0.99 billed on assumed resolutions (no reply within 24 hours); customers audited "resolved" conversations and found most were not. Reported loops and refused transfers ([SF Standard, Oct 2026](https://sfstandard.com/2026/10/15/anthropic-customer-service-human/)); California AB 1609 will require a human within 15 minutes from 2027. Many overlapping concepts (Guidance, Content Guidance, Procedures, Skills, Workflows, Custom Answers). Topics, CX Score and the Agent API are gated to higher plans. Closed models, nothing portable.

### AI-first agent companies (Sierra, Decagon, Maven, Lorikeet)

- **Hybrid control:** the model talks; consequential steps (refunds, identity checks, account changes) run as code that rechecks preconditions against real system state, not against what the model claims. Lorikeet: "the interface is permissive, the operation is strict" ([Lorikeet](https://www.lorikeetcx.ai/blog/ai-architecture-for-complex-cx)).
- **Supervisors:** a second model checks the draft for grounding, policy and tone, and can intercept on high-risk topics ([Sierra](https://sierra.ai/blog/defense-in-depth-in-the-age-of-agents)).
- **Agents as software:** Sierra releases are immutable snapshots (prompts, procedures, model versions, knowledge) with instant rollback; every reviewed bad conversation becomes a regression test; simulations gate releases in CI ([Sierra ADLC](https://sierra.ai/blog/agent-development-life-cycle)).
- **Reliability over best case:** τ-bench measures pass^k, whether an agent succeeds on all k runs of the same task. GPT-4o's retail pass^8 was under 25%, about 60% below its pass^1 ([arXiv 2406.12045](https://arxiv.org/abs/2406.12045)).
- **Self-improving loop with a human gate:** Decagon Duet Autopilot mines failures, drafts a procedure fix, validates it against the failing conversation plus a golden set, and shows a diff for approval ([Decagon](https://decagon.ai/blog/autopilot)).
- **QA on every conversation:** Sierra Monitors, Decagon Watchtower, Kustomer AI QA score 100% of AI and human conversations against plain-language rubrics with visible reasoning.
- **Pricing signal:** Decagon says most customers choose per-conversation pricing because "you never want to be arguing over what a resolution is".

### Helpdesk platforms (Zendesk, Agentforce, Ada, Gorgias, Kustomer, Front, Help Scout, Freshworks, HubSpot, Chatwoot)

- **Table stakes now:** answers from many knowledge sources with citations, natural-language rules, procedures, authenticated actions (commerce lookups above all), handoff with routing, chat and email, 40+ languages, a copilot for teammates, auto-triage, a playground, resolution reporting, knowledge-gap suggestions.
- **Leaders only:** batch simulation with CI gates (Agentforce Testing Center, Fin, Ada), voice on the same agent, QA on every conversation, self-improvement with approval, MCP client and server.
- **What customers hate:** bills charged on silence and quietly widened billing rules (Front now bills even when the customer asks for a human), prices changed or hidden (Kustomer, Front, Plain, Zendesk in 2026), invented policies (Air Canada was held liable for one), bot loops, opaque reasoning, services-heavy setup, QA and tools gated to enterprise plans.
- **Open-source rival:** Chatwoot Captain. Self-hosted it needs a paid licence and OpenAI only; no QA, no simulations, no failure analytics.

## 3. The engineering playbook (what actually moves quality)

1. **Rewrite the conversation into a standalone query** before searching (Fin's biggest single lever).
2. **Hybrid retrieval:** keyword (product names, error codes) plus embeddings, fused. Anthropic measured retrieval failures falling from 5.7% (embeddings) to 2.9% with contextual chunks plus BM25, and to 1.9% with a reranker ([Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval)).
3. **Contextual chunks:** chunk by heading, prefix title, breadcrumb and a generated one-line context.
4. **Rerank and keep few chunks:** 3 to 8 well-ranked chunks beat many.
5. **Answerability gate:** if the sources cannot answer, say so and offer a human; never answer from model memory.
6. **Verify before sending:** cheap similarity check first, an LLM judge only on borderline drafts, retry once, then hand off. DoorDash reports 90% fewer hallucinations this way ([DoorDash](https://careersatdoordash.com/?p=12568)).
7. **Bind identity on the server:** the model chooses the action, never whose account it acts on.
8. **Two-phase writes:** propose, explicit customer yes, then execute with an idempotency key.
9. **Plan before reading untrusted text:** tool choice fixed before emails or pages are read; a tool-less model reads untrusted content ([arXiv 2506.08837](https://arxiv.org/pdf/2506.08837)).
10. **Small models for routing:** intent, language, escalation and relevance on a cheap model; the large model only drafts and plans.
11. **Prompt caching:** static instructions and tool schemas first, retrieved context and the customer's turn last.
12. **Evaluate reliability:** replay each scenario at least 4 times and gate on pass^k; binary, per-criterion judges calibrated against human labels (Shopify took judge agreement from kappa 0.02 to 0.61 by iterating against expert labels).
13. **Count resolution honestly:** confirmed and assumed apart, minus re-contacts on the same topic within 7 days. Klarna's reversal shows deflection is not quality.

## 4. Strategy for the dimmi agent

### Positioning

**The support agent you can see inside and prove works.** Open source, any model, every decision traced, tested like code, honest about what it resolved, and a human is always one message away.

### Principles

1. **Trust before automation rate.** A wrong answer costs more than a handoff. Abstain, verify, hand off.
2. **Show the work.** Every reply links to its trace: query, retrieved passages and scores, rules applied, tools called, verifier verdict, cost.
3. **Honest outcomes.** Confirmed, assumed and false resolution reported separately; silence is never counted as success by default; the workspace defines the window.
4. **A human is always reachable.** "Talk to a human" works on every reply and every channel, loops escalate automatically, and the agent never refuses a transfer.
5. **Agents are code.** Rules, procedures and tests are plain text with versions, diffs, drafts, releases and rollback; tests run from the CLI and in CI.
6. **Any model, no markup.** Bring your own key or a local model; we compete on the pipeline, not on a proprietary model.
7. **One small set of concepts.** Knowledge, rules, procedures, actions, tests. Nothing else to learn.
8. **Everything included.** QA, topics, gaps and tests are not enterprise features.

### What we cannot copy, and what we do instead

| Leader advantage | dimmi answer |
|---|---|
| Fine-tuned answer model (Apex) | Let customers pick the best frontier or local model; win on retrieval, verification and tests, which are most of the quality |
| Fine-tuned embedder and reranker | Hybrid search (Postgres full text plus pgvector) with contextual chunks; rerank with the provider's rerank API when the key allows, else the cheap model |
| Small escalation classifier | Keyword fast path plus the cheap classifier already in the spec; measure it in the test suite |
| Millions of conversations for A/B tests | Per-workspace tests and replays; share prompt and pipeline improvements across every install through open releases |

## 5. Gaps between the current spec and this strategy

The spec (`specs/07-dimmi-agent`) already covers BYOK keys, the classify-retrieve-generate pipeline, citations enforced in code, handoff with summary, actions with identity binding and confirmation, traces, spend caps and the playground. Changes proposed:

| Area | Spec today | Proposal |
|---|---|---|
| Retrieval | Search through the help center contract, similarity threshold | Add contextual chunk headers, hybrid fusion as default, a rerank step, and approved snippets as a source |
| Verification | Optional `verify` mode | On by default: similarity check, judge on borderline, retry once, then hand off |
| Procedures | Post-MVP (`action-procedures`) | Into the first release, small: natural-language steps, conditions as expressions, action steps, confirmation, ask-a-teammate, handoff |
| Tests | Post-MVP (`dimmi test`, replays) | Into the first release: conversation tests and simulated customers, pass^k, CLI and CI, spend estimate and approval before every run |
| Releases | Config edited live | Draft and publish with versions and rollback for rules, procedures, model and prompt version |
| Resolution | Outcome per run | Confirmed, assumed and false resolution with a workspace-defined window and reopen tracking |
| Human access | Max AI replies, keyword and button | Add loop detection (same intent twice, repeated answer), a guaranteed path to a human, and a reported time-to-human |
| Live incidents | Not covered | A "known issues" source that outranks articles while active |
| Improvement loop | Gap report post-MVP | Phase 3: gap clusters, drafts from human replies, suggested rule and procedure changes as diffs a human approves |
| Teammate assist | Post-MVP | Phase 3: drafts and summaries from the same pipeline |

## 6. Phases

**Phase 1: answers you can trust**
- Keys, models and the playground (spec).
- Knowledge: help center, approved snippets, URLs, PDFs, with contextual chunks and hybrid retrieval plus rerank.
- Grounded answers with citations, verification on by default, honest "I don't know".
- Rules (guidance) with audience, channel and schedule targeting.
- Handoff guarantees, loop detection, summary to the teammate.
- Trace view on every AI reply; spend per conversation and month with caps.
- Outcome reporting: confirmed, assumed, false resolution.
- Messenger and email.

**Phase 2: does things, safely**
- Actions (spec): HTTP and MCP, identity bound on the server, read inline, writes confirmed by the customer, optional teammate approval.
- Procedures with conditions, action steps, approval and handoff.
- Tests: saved conversation tests, simulated customers, pass^k, CLI and CI, cost estimate before each run.
- Versions: draft, publish, rollback.

**Phase 3: gets better every week**
- Knowledge-gap clusters and article drafts from teammates' replies.
- Topics with emerging-issue alerts and a "known issues" source.
- QA scores on AI and human conversations (sampled by default, to keep spend predictable).
- Suggested rule and procedure changes as diffs, validated against the test suite, approved by a human.
- Teammate assist: drafts, summaries, translation.

**Later:** voice, memory across conversations, live A/B experiments, proactive outreach.

## 7. Open decisions

1. Bring procedures and tests into the first release (recommended), or keep them after it as the spec says.
2. Cloud edition: BYOK only at launch (spec), or also a managed key at cost so people without a key can start.
3. Rerank default: provider rerank APIs when the key supports them, or always the cheap model.

## 8. How customers and teammates experience it (agreed 2026-11-06)

### Decisions
1. The AI replies instantly to every new conversation by default. Settings offer two alternatives: only when the team is offline, or only when no teammate has replied within N minutes.
2. The AI never speaks first. It replies only when the customer writes; suggested questions on the messenger home screen do the inviting.
3. The business names the assistant and picks its avatar; it always carries a visible "AI" label.
4. AI conversations live in their own inbox view (Answering now, Resolved by AI, Handed off). Only handed-off conversations, and conversations the AI does not handle, reach the team's main inbox.
5. Teammate help (draft reply, summary, translation, ask a question) ships before the customer-facing agent.

### Customer in the messenger
- Opens the messenger: the usual home screen, optionally with suggested questions. No bot greeting.
- Asks a question: a reply streams in within a second or two from the named assistant with the AI label, with source cards under it and two buttons: That helped, Talk to a person.
- Vague question: the AI asks one clarifying question. Missing detail: it asks for it. Actions (later): a confirmation card; nothing happens until the customer confirms.
- Unknown: it says it does not know and offers a person. It never guesses.
- That helped: a short thanks and an optional 1 to 5 rating; the conversation closes as resolved by AI. Leaving without a word: it closes later as probably resolved, never counted as confirmed.
- Talk to a person (button, "human", frustration, repeated question, or a rule): one line saying a teammate is coming, then honest expectations (reply time when online; when the team is back and an email follow-up when away, asking for an email if anonymous). The AI then stays silent. The teammate arrives knowing the whole story.

### Customer by email
- AI reply within a minute or two, written as a normal email with article links, and a footer: reply "human" to reach the team. Anything it cannot handle goes to the team in the same thread.

### Teammate
- Setup (admin, once): key, name, avatar, tone, knowledge (help center plus snippets, pages, files), rules in plain sentences, channels, when it works, a test chat that shows which articles and rules each answer used, then an On switch.
- Inbox: an AI view with Answering now, Resolved by AI and Handed off; a badge on every conversation the AI touched.
- Handoff: a notification with the reason, and a summary card at the top of the conversation (what they want, what the AI said, who they are, why it handed off). The AI stays silent once a teammate replies.
- Take over any live AI conversation at any time; hand back to the AI when done.
- On any AI message: See why (articles and rules used) and Improve (write the better answer, saved as a snippet or rule).
- Composer help: Draft with AI, summarize, translate both ways, ask a question in a side panel.
- Weekly overview: handled, confirmed and probably resolved apart, handed off with reasons, cost, and grouped unanswered questions with "Write an answer" drafted from teammates' replies.
