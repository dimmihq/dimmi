<!--
Follow AGENTS.md before opening this PR:
- Use a lowercase Conventional Commits title (such as `fix: handle empty webhook payload`).
- Keep the description short and scannable: at most 3 sentences per paragraph, plus bullet lists.
- Do not include AI or model attribution anywhere (no Co-Authored-By trailers or agent/model names).
-->

## What changed

<!-- Summarize the change in 1 to 3 short sentences, followed by bullets if needed. -->

- 

## Linked issue

Closes #

## Checklist

- [ ] Targeted tests and lint checks pass for the files I changed.
- [ ] Updated `docs/` if this changes endpoints, fields, settings, defaults, or UI labels.
- [ ] Ran `make api-snapshot`, `make api-compat`, `make sdk-check`, and `make sdk-compat` if routes, requests, resources, or webhook events changed.
- [ ] Added screenshots or a short recording below for UI changes.
- [ ] PR title and commits use lowercase Conventional Commits, paragraphs stay short (at most 3 sentences), and there is no AI or model attribution ([AGENTS.md](https://github.com/dimmihq/dimmi/blob/main/AGENTS.md)).

## Content pages

Complete this section when touching route `meta`, `site/src/pages/vs/`, `site/src/vs/`, `site/src/data/rivals.ts`, or `site/content/`. Delete it otherwise.

- [ ] Signed off by `frantz-kati` (`reviewedBy` and `reviewedOn` set).
- [ ] Rival prices and facts are checked against current sources, with checked dates recorded.
- [ ] Real product proof capture is included, and images have `alt`, `width`, and `height`.
- [ ] Publish gate passes with 0 slop hits and no rival name in `<h1>`.
