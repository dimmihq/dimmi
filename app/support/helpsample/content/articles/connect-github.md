---
title: Connect GitHub
description: Draft changelog entries from merged pull requests and keep articles up to date when your code changes.
collection: integrations
author: jonas
updated: 2026-10-26
---
Connecting a GitHub repository lets dimmi draft changelog entries from merged pull requests and flag articles that mention code that just changed.

## Connect a repository

1. Create a fine-grained personal access token with read access to the repository's contents and pull requests.
2. Open **Settings → Integrations → GitHub**, paste the token and choose the repository.
3. dimmi installs a webhook on the repository when the token allows it, or shows you how to add one.

dimmi never writes to your repository.
