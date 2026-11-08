# Procedures (07 `action-procedures`, first cut 2026-11-08)

Style: caveman. Research: `research/competitors/intercom-procedures-2026-11.md`.

## Built

- Table `procedures`: name (unique per workspace), `status` draft | live, `draft` jsonb working copy, `live` jsonb copy dimmi reads, `lock_version`, `published_at`, soft delete. Prefix `prc`.
- Definition (`app/support/procedure`): `{description, steps: [{id, kind: instruction|condition|action, text, action_id?}], handover: [text]}`. Unknown keys refused. Publish needs a description (1 to 500), 1 to 30 steps each written (1 to 1,000), handovers 1 to 300 each (max 10), every action step naming a workspace action (`aac_`) that exists and is `published`. Errors come back together as `procedure_invalid` with `errors[]` pointing at steps.
- API: `GET/POST /procedures`, `GET/PATCH/DELETE /procedures/{prc}`, `POST/DELETE /procedures/{prc}/publications`. Read `ai_actions:read` (ability `ai:read`), change `ai_agent:manage` (ability `ai:write`). PATCH names `lock_version`; stale saves get 409 `edit_conflict`. `uses` = conversations whose agent runs followed the procedure in the last 30 days.
- Agent: when the workspace has live procedures, the turn offers tool `follow_procedure` (read effect). Its description lists each live procedure `prc_…: Name. Use when: …`; the call returns the procedure rendered as numbered steps ("Check:" for conditions, "Do: … (call the <tool> tool)" for actions, or "not available right now: hand over instead" when the action is not offered this turn) plus the handover list. The model calls it again on later turns while following it. The run is flagged `procedure:<uuid>`. A reply in a turn that followed a procedure counts as `answered` (team-grounded, like action-grounded replies).
- Triage-only mode does not offer the tool.
- UI: `/d/procedures` list (status, description, 30-day use) and editor (name, when to use, steps add/move/remove/kind, action picker of published actions, handovers, publish, turn off, delete), autosaved like workflows.

## Not yet

- Simulations against past conversations (the prototype's "Tests"): needs the 17 dry-run engine; every run costs model spend, so it ships with explicit run approval.
- Sub-procedures, typed conditions over the 18 rule language, procedure selection inside a workflow dimmi step (`procedure_ids`), versions history.
