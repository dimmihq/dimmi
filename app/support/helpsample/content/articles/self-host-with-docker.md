---
title: Self-host dimmi with Docker
description: Run dimmi on your own server as a single binary next to PostgreSQL and Valkey, with Docker Compose.
collection: self-hosting
author: jonas
updated: 2026-10-31
---
dimmi ships as a single binary. Run it on any Linux server with Docker, next to PostgreSQL for data and Valkey for caching and queues.

## Requirements

- A server with 2 CPUs and 4 GB of memory for up to 50 teammates
- Docker 24 or later with Compose
- A domain pointed at the server

## Start it

Save this as `compose.yaml`:

```yaml
services:
  dimmi:
    image: ghcr.io/dimmihq/dimmi:1.6
    ports: ["443:443", "80:80"]
    environment:
      DIMMI_APP__URL: https://support.acme.com
      DATABASE_URL: postgres://dimmi:secret@postgres/dimmi
      REDIS_URL: redis://valkey:6379
    depends_on: [postgres, valkey]
  postgres:
    image: postgres:17
    environment:
      POSTGRES_USER: dimmi
      POSTGRES_PASSWORD: secret
    volumes: ["postgres:/var/lib/postgresql/data"]
  valkey:
    image: valkey/valkey:8
volumes:
  postgres:
```

Then start it:

```bash
docker compose up -d
```

Open your domain and create the first account, which becomes the workspace's owner.

> [!WARNING]
> Change the database password before you start the stack for the first time, and keep `compose.yaml` out of version control.
