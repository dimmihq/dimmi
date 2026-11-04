---
title: Fixes and polish
summary: A round of fixes for email threading, search and the messenger on older Safari.
labels: [fixed]
version: v1.3.3
published: 2026-10-02
authors: [maya, jonas]
reactions: {"👍": 11, "❤️": 2, "🎉": 0, "🚀": 1, "👀": 0}
---
- Replies to old email threads now join the right conversation when the customer's mail client drops the `References` header.
- Search finds conversations by the customer's company name again.
- The messenger opens on Safari 15, which some customers' kiosks still run.
- Snoozed conversations no longer come back an hour early after a daylight saving change.
- Attachments over 10 MB show a clear error instead of an endless spinner.
