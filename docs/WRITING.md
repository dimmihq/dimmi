# Writing the dimmi docs

How to decide what goes in the dimmi docs (`docs/`, served at trydimmi.com/docs) and how to write it. People and agents both follow it. Before you write or edit a page, also read the stop-slop skill in `.claude/skills/stop-slop/` and apply it.

Most documentation is too long. Readers are developers and support leads who can read a screen, a form and an error message without help. The docs exist for what the product cannot say for itself.

## 1. Decide what is worth writing

Write it only if the reader cannot get it from the product in under a minute.

Worth writing:

- **Code the reader must write or paste:** snippets, SDK calls, signature checks, config.
- **Facts the interface does not show:** limits, defaults, retry schedules, formats, what happens in the background.
- **How a feature behaves,** when that takes more than one screen to see: how assignment picks a teammate, when an SLA clock pauses.
- **Choices with consequences:** which mode to pick and what it changes.
- **Reference data:** events, options, commands.

Not worth writing. Link to the screen and stop:

- **Walking through a form or wizard.** Sign-up, onboarding, "create an API token", "add a webhook endpoint". Say where it is (**Settings → API tokens**) and name only the field whose meaning is not obvious.
- **What a labeled control does.** A toggle named **Show launcher** needs no sentence.
- **Error messages that already say the fix.** Document an error only when its fix is not in the message.
- **Summaries of a file the reader can open.** Link the file.
- **Test rigs and detours.** No throwaway test pages, no local web servers, no "check that it worked" for things that visibly work.
- **Generic knowledge.** What a webhook is, what CSP is, what OAuth is.
- **Internals and unshipped things.** Token prefixes nobody creates, settings nobody should touch, plans.
- **Self-hosting details on product pages.** Configuration keys and environment variables belong only under Self-hosting.

Tests before you keep a page, a section or a sentence:

1. **Delete test.** Remove it. Does a competent reader now fail or get something wrong? If not, it stays deleted.
2. **Screen test.** Does the product already show this on the screen where the reader needs it? Then link to the screen.
3. **Merge test.** A page under about 150 words belongs as a few bullets on its parent page.

## 2. Keep it short

Budgets. Going over means the page is doing too much: cut, don't squeeze.

| Thing                       | Limit                                                          |
| --------------------------- | -------------------------------------------------------------- |
| Paragraph                   | 1 to 2 sentences, about 40 words. Never more than 3 sentences. |
| Sentence                    | About 20 words.                                                |
| How-to or explanation       | 150 to 400 words of prose, plus code.                          |
| Overview or comparison page | 5 to 7 short paragraphs or lists in total.                     |
| Steps in one procedure      | 5. More means you are narrating a form.                        |
| Opening                     | One sentence. Then the content.                                |
| Callouts                    | At most one per page.                                          |
| Reference page              | As long as its data, with no prose beyond a line per group.    |

- Prefer a list or table to a paragraph whenever items are parallel.
- No "Before you begin", "Check that it worked", "Troubleshooting", "What you built" or "Next steps" section by default. Add one only when it passes the delete test. A prerequisite is usually one inline link.
- Say each fact once, on one page. Link to it from elsewhere.
- No recap of what the page said, and no preview of what it will say.

### Bold

Bold is for two things only:

- **Interface labels** the reader clicks or looks for: **Settings → Webhooks**, **Rotate**.
- **The lead word of a list item** in a list of definitions or options, when it helps scanning.

Never bold for emphasis, never bold a whole sentence, never bold product terms in running text. If a paragraph has more than two bold phrases, it is a form walkthrough: cut it.

## 3. Pick one page type

Every page is one of four [Diátaxis](https://diataxis.fr) types. Keep a page to its type and to one topic.

| Type        | The reader wants to                      | Title pattern                              | Template                    |
| ----------- | ---------------------------------------- | ------------------------------------------ | --------------------------- |
| Tutorial    | Build one working thing, start to finish | "Build your first action"                  | `templates/tutorial.mdx`    |
| How-to      | Reach one goal they already have         | Verb + object: "Verify webhook signatures" | `templates/how-to.mdx`      |
| Reference   | Look up a fact                           | Noun: "Webhook events"                     | `templates/reference.mdx`   |
| Explanation | Understand how something works           | "How assignment works"                     | `templates/explanation.mdx` |

Tutorials are rare: write one only where the reader builds something with code. Background goes on an explanation page and options on a reference page, linked from the how-to.

The REST API reference is generated from the API contract. Never write it by hand, and never restate its fields on other pages: link to the endpoint.

## 4. Get the facts from the code

Docs describe what the code does today. Read the code; never write from memory, a design or the marketing site. Every name, default, limit and command on a page is one you checked.

| Fact                                              | Source of truth                                                                                                                                                                   |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| REST endpoints, fields, types, required abilities | `tests/contracts/api_v1.openapi.json`, generated from `routes/*.go`                                                                                                               |
| Webhook events and payloads                       | `tests/contracts/webhooks_v1.json`                                                                                                                                                |
| Messenger API (the messenger's HTTP API)          | `tests/contracts/widget_v1.openapi.json`; messenger code in `resources/widget/`                                                                                                   |
| Messenger JavaScript API                          | `resources/widget/core.ts` (commands), `sdk/browser/src/`                                                                                                                         |
| SDK usage                                         | `sdk/<language>/` and each SDK's README. Generated code is not committed; run `make sdk-generate` to read it.                                                                     |
| Configuration keys and environment variables      | `config/*.go` and `framework/foundation/config.go` (the `koanf` tags and `default` values), `deploy/selfhost/.env.example`, `deploy/selfhost/references/`                         |
| What a request actually does                      | `routes/*.go`, then the controller in `app/http/controllers/`, then the action in `app/actions/`                                                                                  |
| Background work and its timing                    | `app/jobs/`, `app/listeners/`                                                                                                                                                     |
| Words in the app's interface                      | `resources/js/features/shell/navigation.ts` (every area and settings page, with its one-line description), the screens in `resources/js/features/`, `resources/js/lang/en/*.json` |
| Permissions and roles                             | `app/auth/permissions.go`, `app/auth/abilities.go`                                                                                                                                |
| Self-hosting                                      | `deploy/selfhost/` (`install.sh` is `dimmictl`, `compose.yaml`, `Caddyfile`, `references/`)                                                                                       |
| Cloud vs self-hosted differences                  | `config/edition_*.go` and the `settle()` functions in `config/*.go`                                                                                                               |
| API compatibility promises                        | `contracts/VERSIONING.md`                                                                                                                                                         |

- Operations marked `x-internal: true` exist for dimmi's own app. Never document them.
- Copy interface labels exactly as the dashboard shows them.
- Don't document what isn't shipped or isn't reachable by users.
- When code and a comment or README disagree, the code wins. Report the stale text.
- On self-hosting pages, a key `auth.sessions.idle_lifetime` is the variable `DIMMI_AUTH__SESSIONS__IDLE_LIFETIME`.

## 5. Use one name for each thing

Use the term in the first column, always. When the API names it differently, give the API name once, in code: "a customer (`contact` in the API)".

| Use                      | Not                               | Meaning                                                                                                                                |
| ------------------------ | --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| dimmi                    | Dimmi, DIMMI                      | The product. Always lower case, even at the start of a sentence.                                                                       |
| workspace                | organization, account, team       | One company's dimmi: its inboxes, customers, settings and members.                                                                     |
| customer                 | user, visitor, end user           | A person who writes to your team. `contact` in the API. A visitor is a customer we know nothing about yet.                             |
| teammate                 | agent, operator, staff, user      | A person on your team who answers customers. Use **member** only for workspace membership: invitations, roles, **Settings → Members**. |
| conversation             | chat, thread, ticket              | One exchange with a customer, on any channel.                                                                                          |
| inbox                    | queue, mailbox                    | A container conversations are routed into, with its own team and assignment rules.                                                     |
| view                     | filter, folder                    | A saved way to list conversations: **My inbox**, **Unassigned**, or one you create.                                                    |
| ticket                   | case, issue                       | Tracked work with a state the customer can see. Kinds: customer ticket, back-office ticket, tracker.                                   |
| tracker                  | incident, master ticket           | A ticket for one problem many customers report, updated once for all of them. Not Linear or Jira.                                      |
| linked issue             | tracker, external ticket          | A ticket's link to Linear, Jira or GitHub.                                                                                             |
| messenger                | widget, chat bubble, live chat    | The chat on your website and app. Its HTTP API is the Messenger API.                                                                   |
| dimmi agent              | bot, AI assistant, Fin            | The AI that answers customers and hands over to teammates.                                                                             |
| Operator                 | copilot, assistant                | The AI your team chats with inside dimmi. It proposes changes as changesets.                                                           |
| handover                 | handoff, escalation, transfer     | The dimmi agent passing a conversation to a teammate.                                                                                  |
| saved reply              | macro, canned response, snippet   | Reusable reply text, with variables and actions.                                                                                       |
| office hours             | business hours, opening hours     | The workspace schedules used by SLAs and the dimmi agent.                                                                              |
| SLA policy               | SLA rule                          | Reply and resolution targets for conversations and tickets.                                                                            |
| workflow                 | automation, rule, flow            | An automation with a trigger and steps.                                                                                                |
| procedure                | playbook, runbook                 | Step-by-step instructions the dimmi agent follows for a kind of request.                                                               |
| action                   | tool, integration, function       | A call the dimmi agent can make to your own system.                                                                                    |
| knowledge source         | data source, content source       | Where the dimmi agent learns answers: a website, files, GitHub, Notion, Intercom, Zendesk.                                             |
| help center              | knowledge base, docs site         | Your public articles, collections and search.                                                                                          |
| feature requests         | feedback, feedback board, roadmap | The public board of ideas customers post and vote on. `feedback-*` in the API.                                                         |
| broadcast                | campaign, blast                   | A one-off message to many customers.                                                                                                   |
| series                   | journey, drip, sequence           | An automated, multi-step outbound journey.                                                                                             |
| API token                | API key, personal access token    | The credential for the REST API. Its permissions are **abilities**; OAuth calls them scopes.                                           |
| identity verification    | user hash, HMAC                   | Signing a customer's identity so the messenger can trust it.                                                                           |
| sign in, sign-in         | log in, login                     | The verb and the noun.                                                                                                                 |
| dimmi Cloud, self-hosted | SaaS, on-prem, enterprise         | The two ways to run dimmi.                                                                                                             |

When the product renames something, update this table and every page in the same change.

## 6. Write like this

- State the fact or the action. No lead-in, no restating the title.
- Address the reader as "you". Active voice, present tense: "dimmi retries 12 times over 24 hours".
- Condition before instruction: "To rotate the secret, open **Settings → Webhooks**".
- Name things: "pass the `session_token` to `createSession()`", not "pass this value".
- Exact numbers and names from the code. Never "several", "a few", "some settings".
- Plain words, American spelling, no idioms. Many readers read English as a second language.
- "dimmi" is lower case, always.

Never write:

- "simply", "just", "easy", "quickly", "obviously", "please", "we'll", "let's", exclamation marks.
- "In this section", "the above", "as mentioned", "note that", "it's worth noting", "keep in mind".
- "coming soon", "will support".
- Em dashes. Use a comma, a colon or a period.
- Anything on the stop-slop lists (`.claude/skills/stop-slop/references/`), with one exception: docs may use passive voice and "never" where they are the exact truth.

### Titles, sidebar labels, descriptions, headings

- **Sidebar label** (`label` in `docs/src/nav.ts`): one to three words. The group gives the context: under Messenger, "Install", not "Install the messenger in your app". Under Webhooks, "Signatures", "Retries", "Events".
- **Title:** specific enough to stand alone in search and for agents: "Verify webhook signatures". It can be longer than the sidebar label.
- **Description:** one sentence, under 160 characters, saying what the page gives the reader. Never "Learn about …", never a list of everything on the page.
- **Headings:** sentence case, unique on the page, meaningful alone: "Rotate the secret", not "More options".

### Code samples

- Language on every block, and a file name when there is one: ` ```ts title="app/layout.tsx" `.
- Complete enough to paste and run: imports and setup included.
- Only the languages and frameworks the reader needs. When the same task differs per framework, use `<Tabs>`.
- Placeholders in `<angle_brackets>`, each explained in a few words after the block. Realistic fake values: Acme Inc, `ada@example.com`.
- No shell prompts. Output goes in its own block, only when it isn't obvious.
- Never a real secret, token or customer.

## 7. Make pages work for agents

Agents fetch one page, or one section of it, as Markdown (`/docs/<page>.md`, `/docs/llms.txt`, `/docs/llms-full.txt`) and act on it. Short, exact pages serve them best.

- A page makes sense alone. Link a prerequisite in one line instead of assuming the previous page.
- A section makes sense from its heading: name the feature again instead of "it".
- Exact names, numbers and units, spelled as in the code.
- Nothing important only in a picture. Every screenshot has alt text that says what it shows.
- Agents read every tab. Put shared content before the tabs.
- When a page moves, add a redirect in `docs/astro.config.mjs`.
- Everything in `docs/` is public. Internal notes stay out of `docs/src/content/`.

## 8. Screenshots

Use a screenshot only where a picture saves the reader a search: a screen that is hard to find, or a result they must recognize. Most pages need none. Pictures are real captures, made by `docs/shots/capture.ts` (see `docs/shots/README.md`), shown with `<Screenshot name="…" alt="…" />`. Mark one still to capture with `{/* SHOT: name | path | what to show | alt */}`.

## 9. Components

Import from `~docs/components/mdx`. Use one only when it helps.

| Component                            | Use for                                                                                               | Avoid                                                                    |
| ------------------------------------ | ----------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| `<Steps>`                            | The numbered steps of a tutorial or how-to                                                            | Anything that is not a sequence                                          |
| `<Tabs>` / `<TabItem label>`         | The same task in several languages or platforms                                                       | Different information in each tab; content shared by all tabs            |
| `<Callout type>`                     | `note` (useful aside), `tip` (a better way), `caution` (can go wrong), `danger` (data loss, security) | More than two per page; content the reader must read belongs in the text |
| `<Card>`, `<CardGrid>`, `<LinkCard>` | Index and overview pages that route readers onward                                                    | Inside how-tos and reference                                             |
| `<Endpoint method path>`, `<Param>`  | Showing one endpoint inside a how-to or explanation                                                   | Rebuilding the API reference by hand                                     |
| `<FileTree>`                         | A folder layout the reader must create or find                                                        | Lists that are not files                                                 |
| `<Accordion>`                        | Frequently asked questions                                                                            | Steps or required information                                            |
| `<Badge>`                            | Marking status, such as "Beta", in running text                                                       | Emphasis                                                                 |

## 10. Checklist

- [ ] The page passes the delete, screen and merge tests (section 1).
- [ ] Every paragraph is 1 to 3 sentences. The page is inside its budget (section 2).
- [ ] Bold only on interface labels and list lead words.
- [ ] No form walkthroughs, no restated error messages, no generic explanations, no self-hosting configuration on product pages.
- [ ] One type, one topic. The sidebar label is one to three words.
- [ ] Every name, default, limit and command was checked in the code.
- [ ] Terms match section 5. Labels match the dashboard.
- [ ] Code samples paste and run. No real secrets.
- [ ] No banned words (section 6) and nothing from the stop-slop lists.
- [ ] The page is in `docs/src/nav.ts`, and `pnpm docs:build` passes.

## 11. Keep the docs true

A change that alters what a reader must know (an endpoint, a field, an event, a limit, a default, an install step) updates the docs in the same commit. When a feature is removed, remove its page and redirect the URL. If the docs are wrong, the product is wrong for the reader.

To test the docs with agents, keep a saved set of real customer questions with the facts a good answer needs, and rerun it after big changes. It calls real models, so it runs only when the owner asks.
