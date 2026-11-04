---
title: Back up your data
description: What to back up on a self-hosted install, and how to restore it on a new server.
collection: self-hosting
author: jonas
updated: 2026-10-05
---
Everything dimmi stores lives in PostgreSQL, apart from attachments, which live in the file storage you configured. Back up both.

```bash
docker compose exec postgres pg_dump -U dimmi -Fc dimmi > dimmi-$(date +%F).dump
```

Valkey only holds caches and queued jobs, so it doesn't need a backup.
