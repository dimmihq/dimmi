# Market landscape (first pass)

Researched 2026-11-01. Broad scan before deep competitor dives in `competitors/`. Mark [unverified] = single source or sources disagree. [3P] = third-party or competitor-written.

## TL;DR

- Intercom rename to **Fin** May 2026. Salesforce sign deal to buy Fin for about $3.6B (2026-06-15). Close date conflict: one source say closed 2026-09-10, others expect close by Jan 2027 [unverified].
- Fin AI near $100M ARR. Fin bill $0.99 per "outcome". Since Mar 2026 outcome include handoff to human. Reported bill spikes $4k to $9k per month [3P].
- Open-source field weak. Chatwoot need 4GB+ RAM, keep SSO/SLA/audit/AI in proprietary folder. Libredesk closest in architecture (Go) but need Redis, no serious AI.
- Nobody ship "one binary + Postgres" with Intercom-level features. Gap for Dimmi.
- New front (Oct 2026): customers' own AI agents contact support. Sierra + Meta publish Personal Agent Protocol. Decagon open-source PACT (Apache-2.0). No helpdesk support either yet.

## Closed-source players

| Product | Notes |
|---|---|
| Fin (ex-Intercom) | Seats $29/$85/$132 annual plus $0.99/outcome. Procedures, Simulations, Skills (Fin 4, Oct 2026), CX Score, Monitors, Operator, Voice, MCP client and server, Fin CLI, Agent API. Proprietary Apex models. |
| Zendesk | Bought Forethought (Mar 2026). Relate 2026: AI agents GA, Agent Builder, "verified resolution" pricing, about $1.50 to $2.00 per resolution [3P]. |
| Decagon | $250M Series D at $4.5B (Jan 2026). AOPs, Duet Autopilot (proposes changes, human approve diffs), Experiment Hub (A/B in procedures), Voice 3. Median contract about $400k/year [3P]. |
| Sierra | $950M Series E [3P]. Ghostwriter, Agent SDK, Horizon (long-running goals), Compliance Console, tau-bench family. |
| Plain | API-first B2B inbox (GraphQL). Slack/Teams/Discord, tiers, SLAs, Linear/Jira links. |
| Pylon | B2B. Slack Connect, Account Intelligence ($10/account/month), CRM sync. |
| Help Scout | Email-first. Tried contacts pricing, reverted to per-user [unverified]. |
| Crisp | Per-workspace pricing. EU data residency pick. |
| Front, Freshdesk, HubSpot, Gorgias (Shopify), Ada, Parloa (voice), Crescendo | Adjacent. |

## Open-source players

| Project | Stack | Footprint | License | Status |
|---|---|---|---|---|
| Chatwoot | Rails + Vue, Sidekiq, Redis, Postgres+pgvector | Min 4GB RAM + 1GB swap (official). Third-party 1.1GB idle [unverified]. | MIT core + proprietary `enterprise/` (SAML, roles, audit, SLA, Captain AI, voice) | 38.7k stars, very active |
| Libredesk | Go + Vue, Postgres + Redis | Not measured | AGPL-3.0 | 3.1k stars, active (Zerodha) |
| Chaskiq | Rails + AnyCable-go + Redis | | AGPL + Commons Clause | Stale |
| Papercups | Elixir/Phoenix | | MIT | Dead since 2024 |
| Tiledesk | 15 containers (Mongo, RabbitMQ, Redis, Qdrant) | Heavy | MIT | Active |
| erxes | TypeScript, Mongo, Redis, RabbitMQ, Elasticsearch | 16GB eval [unverified] | AGPL + no-SaaS clause | Active |
| Zammad | Rails, Postgres, Redis, Elasticsearch | Min 6GB + 4-6GB for ES | AGPL | Active, ticketing |
| FreeScout | PHP/Laravel | About 122MB idle [unverified] | AGPL + paid modules | Email only |
| Cossistant | Bun + Hono + Next.js, Postgres + Redis | | Murky AGPL non-commercial | Small |
| Opencom | Convex (serverless) | | AGPL | 68 stars, new |

Why rivals heavy: Rails + Sidekiq boot framework twice. Elasticsearch JVM take half RAM. Mongo replica sets, RabbitMQ, separate websocket servers. Redis hard dependency nearly everywhere.

## Self-updating help centers

- **Ferndesk** (ferndesk.com): closest to "auto-updating help center connected to GitHub". Pro $149/mo. Agent "Fern" draft from merged PRs, audits, Linear, Slack, Intercom/Zendesk tickets. Claim-level article verification against code, live app, support replies. Auto screenshots behind login. Never auto-publish. No versioning.
- **Productlane**: self-updating help center (Jun 2026). Best review UI: inline highlighted diffs, +/- counts, accept/reject per edit.
- **Mintlify Autopilot**: review PR to code owner via Slack DM. Pro $450/mo.
- **Promptless** (YC W25): citations per suggestion, learn from rejections.
- **GitBook Agent, Doc Holiday, Dosu, DeepWiki, Kapa, Inkeep, Brainfish**: adjacent.

## Customer-agent protocols (Oct 2026)

- Personal Agent Protocol (Sierra + Meta): OAuth-based, MCP/OpenAPI transport. Partners Shopify, Stripe, Walmart.
- PACT (Decagon, Apache-2.0): A2A + OAuth 2.0. Separate agent identity from permissions. Spec, reference implementation, conformance tests.
- Opportunity: Dimmi first helpdesk to support both.

## World-class bar (merged ideas)

- Messenger: spaces (Home, Messages, Tickets, Help, News, Tasks), JWT identity verification (Intercom deprecated HMAC Aug 2026), loader under 100kb.
- Inbox: Cmd+K, single-key actions, live saved views, SLA sort, collision detection, shared drafts.
- Tickets: customer, back-office, tracker types. Type change in place. Required-on-close fields.
- Knowledge: drift detection from many sources, claim verification, PR-style review, self-healing screenshots, gap engine, llms.txt + MCP, versioned docs.
- AI agent: procedures (prose + deterministic steps), git-style versioning, simulations, confidence-based approval, traces, A/B tests.
- Agents first-class: every admin action via API, MCP, CLI. Built-in agents use same public tools. Bring-your-own-agent. AI agents as teammates with scoped permissions.
- Trust: SSO, roles, audit, PII redaction all open. Data residency. Auditable billing.
- Migration: import from Intercom, Zendesk, Help Scout, Chatwoot, keep help-center URLs.

## Architecture direction (first pass, superseded by `tech-stack.md`)

- Go backend, TypeScript frontend embedded via go:embed. One binary with roles (web, worker, scheduler).
- Minimal install: one binary + Postgres with pgvector. River queue, LISTEN/NOTIFY realtime (IDs only, outside hot transactions), Postgres full-text search.
- Every external service behind interface. AWS default (S3, SES, KMS, Bedrock), community adapters.
- License: MIT (PO decision 2026-11-02), no enterprise folder.

## Sources

- https://en.wikipedia.org/wiki/Fin_(company)
- https://www.salesforceben.com/salesforce-acquires-fin-formerly-intercom-adding-30k-ai-customers/
- https://fin.ai/blog/introducing-fin-4
- https://community.intercom.com/ask-the-intercom-team-about-fin-54/fin-s-flawed-assumed-resolved-pricing-design-8929
- https://decagon.ai/blog/autopilot
- https://decagon.ai/blog/introducing-the-personal-agent-consent-trust-protocol-pact
- https://sierra.ai/blog/introducing-personal-agent-protocol
- https://www.zendesk.com/newsroom/articles/relate-2026/
- https://www.usepylon.com/pricing
- https://www.plain.com/product
- https://ferndesk.com
- https://productlane.com/changelog/2026-06-25-self-updating-help-center
- https://www.mintlify.com/docs/autopilot.md
- https://developers.chatwoot.com/self-hosted/deployment/requirements
- https://github.com/chatwoot/chatwoot/blob/develop/LICENSE
- https://docs.zammad.org/en/latest/prerequisites/hardware.html
- https://github.com/tiledesk/tiledesk-deployment
- https://listmonk.app
- https://recall.ai/blog/postgres-listen-notify-does-not-scale
- https://www.intercom.com/help/en/articles/183-enable-identity-verification-for-web-and-mobile
