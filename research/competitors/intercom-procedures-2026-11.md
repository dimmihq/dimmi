# Intercom Fin Procedures (research 2026-11-07)

Style: caveman. Input for the procedures build (07 `action-procedures`, 17 workflows).

## What they are

- SOP written in plain language, or pasted; Fin drafts a Procedure from it, grounded in past conversations, docs and data connectors.
- Steps: natural-language Instruction (default), Condition (only for big mutually exclusive branches; no nesting), Call data connector (live data; result readable by later steps, do not call twice), code steps for exact logic (dates, numbers, string match). Code and natural language never mixed in one condition.
- Sub-procedures: reusable blocks, callable many times.
- Handoff: internal note for the human; Fin may switch procedures when intent changes. Procedure does not resume after a workflow finishes.
- Simulations: test a procedure against simulated customers before going live.
- Data connectors can require email OTP first (6 digits, 10 min, 3 tries, 30 min window).

## Where dimmi can beat it

- Steps call dimmi actions (HTTP and MCP) with identity bound server side; writes still need the confirmation card and optional teammate approval.
- Email code proof already built (code never in the transcript, 24 h session proof).
- Conditions over the 18 rule language (typed, deterministic) instead of Python snippets.
- Procedure versions with drafts, simulation against real past conversations (17 engine already plans dry runs), per-step traces.
- Procedures usable inside a workflow "Let dimmi handle" step (17 reserved `procedure_ids`).

## Sources

- https://www.intercom.com/help/en/articles/13449439-building-fin-procedures
- https://www.intercom.com/help/en/articles/12599460-practical-examples-of-fin-procedures
- https://intercom.com/blog/procedures-simulations-updates
- https://www.intercom.com/help/en/articles/10741459-secure-data-connectors-with-one-time-passcode
