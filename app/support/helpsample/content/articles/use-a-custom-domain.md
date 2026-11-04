---
title: Use a custom domain
description: Serve your help center and changelog from your own domain, like help.acme.com, with a free certificate.
collection: help-center
author: jonas
updated: 2026-10-10
---
Your help center starts at a dimmi subdomain. Move it to your own domain so links look like yours and search engines credit your site.

## Add the domain

1. Open **Settings → Help center → Domain** and enter the domain, like `help.acme.com`.
2. Add the two records dimmi shows you at your DNS provider: a `TXT` record that proves the domain is yours, and a `CNAME` that points it at dimmi.
3. Wait for both checks to turn green. Most providers take a few minutes.

dimmi issues a certificate as soon as the records are in place and renews it automatically.

> [!NOTE]
> Your changelog moves with the help center, to `help.acme.com/changelog`. Old links to the dimmi subdomain redirect to the new domain.
