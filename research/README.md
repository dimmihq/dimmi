# Dimmi research

Open-source Intercom/Fin alternative. Domain: trydimmi.com. Research date: 2026-11-01. Style: caveman.

Start with `roadmap.md`. Everything else is evidence.

## Files

| File | What |
|---|---|
| [roadmap.md](roadmap.md) | **Synthesis.** Launch must-haves, post-MVP priority list, standout marketing features, build notes, open decisions. |
| [competitors/fin.md](competitors/fin.md) | Fin (ex-Intercom): Fin AI agent plus Intercom helpdesk. Pricing, features, Fin CLI, Salesforce fallout. |
| [competitors/helpdesk.md](competitors/helpdesk.md) | HelpDesk (helpdesk.com) and Text suite (LiveChat, ChatBot, KnowledgeBase). |
| [competitors/featurebase.md](competitors/featurebase.md) | Featurebase: inbox, Messenger, help center, feedback, Fibi agent. Indie-hacker go-to-market. |
| [competitors/productlane.md](competitors/productlane.md) | Productlane: Linear-native support, self-updating help center, review queue. |
| [tech-stack.md](tech-stack.md) | Language, database, frontend, monolith strategy, "don't build" library list, risks. |
| [pricing.md](pricing.md) | Per-conversation pricing, billable conversation definition, unit economics, bill comparisons, BYOK design, billing stack. |
| [landscape-2026-11.md](landscape-2026-11.md) | First broad market scan: closed and open-source players, self-updating help centers, agent protocols. |
| [naming.md](naming.md) | Why Dimmi. Trademark, domain, GitHub, npm checks. |

## Key decisions so far

- Name: Dimmi. Domain: trydimmi.com.
- Monolith, not serverless. One Go binary plus PostgreSQL 18. No Redis, no Elasticsearch, no broker.
- Every external service behind interface. AWS adapters default (S3, SES, KMS, Bedrock). Local defaults for self-host.
- AI is BYOK. Dimmi ships no inference.
- Pricing: one plan, 100 conversations free, then $0.03 / $0.015 / $0.01 graduated, talk to us above 50,000. 80% gross margin floor. Unlimited seats, teams, inboxes, integrations.
- Launch channels: web widget, email, Slack, Discord, Telegram, WhatsApp.
- First users: vibe coders, solo devs, early adopters.

## Conventions

- [unverified] = single source, or sources disagree.
- [3P] = third-party or competitor-written claim.
- Sources listed at bottom of each file.
- Known conflict: Salesforce/Fin deal close status (`fin.md` says closed 2026-09-10, `pricing.md` says pending to early 2027).
