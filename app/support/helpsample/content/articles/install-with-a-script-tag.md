---
title: Install with a script tag
description: Paste one snippet before the closing body tag of any site, whatever it's built with.
collection: messenger-install
author: jonas
updated: 2026-10-12
---
If your site isn't built with a framework we have a package for, paste this snippet before `</body>` on every page:

```html
<script async src="https://cdn.trydimmi.com/widget/v1/loader.js" data-app-id="wgt_7f3k"></script>
```

The loader is under 2 KB. It waits until your page has loaded, then fetches the messenger in the background.

## Content security policy

If your site sends a content security policy, allow dimmi's hosts:

```
script-src https://cdn.trydimmi.com;
connect-src https://api.trydimmi.com;
```

The messenger never needs `unsafe-inline` or `unsafe-eval`.
