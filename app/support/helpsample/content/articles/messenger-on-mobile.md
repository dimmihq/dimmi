---
title: Use the messenger in a mobile app
description: Show the messenger inside your iOS or Android app with a web view, and open it from your own buttons.
collection: messenger
author: jonas
updated: 2026-09-30
---
The messenger runs in any web view, so you can bring it into a mobile app today without a native SDK.

## Open it in a web view

Point a web view at your workspace's hosted messenger page:

```
https://messenger.trydimmi.com/wgt_7f3k?mode=fullscreen
```

Pass an identity token in the page's fragment to sign the customer in, the same way as on the web. The fragment never reaches a server log.

## Open it from your own button

Hide the launcher with `hide_launcher: true` and call `Dimmi("show")` from any button in your app. Use `Dimmi("show", "messages")` to land straight in the conversation list.
