---
title: Write articles in Markdown
description: Everything the help center's Markdown supports, from callouts and tables to code with syntax highlighting.
collection: help-center
author: ama
updated: 2026-10-22
---
Articles are written in Markdown, the same format as a README on GitHub. If you've written one of those, you already know how to write an article.

## Text

Use `**bold**` for **bold**, `_italic_` for _italic_ and backticks for `inline code`. Links to other articles can use their path, like `[Invite your team](/articles/invite-your-team)`, and keep working if the article moves.

## Callouts

Callouts use GitHub's syntax and come in five kinds:

> [!NOTE]
> Useful information customers should know, even when skimming.

> [!TIP]
> Advice that helps them do something better or faster.

> [!WARNING]
> Something that needs their attention to avoid a problem.

## Tables

| Syntax | Renders as |
|---|---|
| `# Title` | The article's title, set from the title field instead |
| `## Section` | A section, listed in the table of contents |
| `### Subsection` | A subsection, nested in the table of contents |

## Code

Fenced code blocks keep their formatting:

```json
{
  "app_id": "wgt_7f3k",
  "hide_launcher": false
}
```

## Task lists

- [x] Write the article
- [x] Add a description for search results
- [ ] Publish it
