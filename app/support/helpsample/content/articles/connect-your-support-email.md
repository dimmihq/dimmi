---
title: Connect your support email
description: Forward your support address to dimmi so emails arrive in the inbox and replies go out from your own domain.
collection: getting-started
author: jonas
updated: 2026-10-14
---
Customers who email `support@acme.com` should get the same answers as customers who chat. Connecting your support address brings their emails into the shared inbox, threaded with any chats from the same person.

## Forward incoming email

Every workspace has its own forwarding address, shown in **Settings → Email**. It looks like this:

```
acme-7f3k@inbound.trydimmi.com
```

Set up a forwarding rule from your support mailbox to that address. In Google Workspace, open **Gmail settings → Forwarding and POP/IMAP**, add the address and confirm the verification email, which appears in your dimmi inbox within a few seconds.

## Send replies from your domain

Replies go out from `support@acme.com` once you verify your domain. Add these records at your DNS provider:

| Type | Name | Value |
|---|---|---|
| TXT | `@` | `v=spf1 include:mail.trydimmi.com ~all` |
| CNAME | `dimmi._domainkey` | `dkim.trydimmi.com` |
| TXT | `_dmarc` | `v=DMARC1; p=none` |

dimmi checks the records every few minutes and shows a green check next to each one when it's in place.

> [!WARNING]
> Keep your existing SPF record and add dimmi's `include` to it. A domain with two SPF records fails both checks, and your replies may land in spam.
