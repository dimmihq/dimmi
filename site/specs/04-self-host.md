# Self-hosted page spec (`/self-host`)

`/self-host` is a dedicated marketing page in `site/src/pages/self-host.tsx` that ranks for "self-hosted help desk" and "self hosted live chat". It gives engineers the real install command, server costs, Docker Compose path, and a candid Cloud versus self-hosted breakdown.

## Metadata and SEO

- **Route:** `/self-host` in `site/src/routes.ts` with `PageMeta` (`type: "self-host"`, `reviewedBy: "frantz-kati"`), plus `/self-host /self-host/*` in `deploy/k8s/apps/base/edge/Caddyfile`.
- **Title:** `Self-hosted help desk and live chat, MIT licensed | dimmi` (57 characters, within the 60-character limit).
- **Description:** `Run dimmi on your own Linux server in about 5 minutes. The same MIT code as dimmi Cloud: shared inbox, AI agents and help center. You pay us $0.` (144 characters).
- **Primary heading (`<h1>`):** 1 `<h1>` focused on owning your support stack, with zero rival names.
- **Schema:** `SoftwareApplication` (`SOFTWARE_SCHEMA` in `site/src/seo.ts`) and `BreadcrumbList` (`Home`, `Self-host`). Include real published and updated dates in HTML, JSON-LD, and `sitemap.xml`. Zero `FAQPage` schema.

## Single sources of truth

Reuse existing canonical data so nothing drifts:

- **Core pitch:** reuse `site/src/sections/self-host.tsx` concepts while keeping the page body at least 65% unique from `/`.
- **Install command:** match the exact one-line installer from `README.md`:

```bash
curl -fsSL https://github.com/dimmihq/dimmi/releases/latest/download/install.sh \
  | bash -s -- --domain support.example.com --email ops@example.com
```

- **Server sizes and costs:** import `SERVERS` directly from `site/src/pricing/servers.ts` (`$24` Minimum, `$48` Recommended, `$96` Large on DigitalOcean, with Hetzner equivalents). Never hardcode server prices in JSX.
- **Docker Compose path:** reference `deploy/selfhost/compose.yaml` and the container image `ghcr.io/dimmihq/dimmi`.

## Required sections

1. **Direct answer in the first 50 words:** state that dimmi self-hosts on one Linux server under the MIT license for `$0` to dimmi, with the same features as dimmi Cloud.
2. **One-line install block:** copyable command with a copy button that fires `track("install_copy", { location: "self-host" })`.
3. **Server sizing and costs:** table or cards driven by `SERVERS` from `site/src/pricing/servers.ts`.
4. **dimmi Cloud vs self-hosted table:** factual comparison of hosting, email, Slack, AI keys, upgrades (`dimmictl`), and cost, linking to `/docs/cloud-vs-self-hosted/`.
5. **Docker Compose option:** short snippet pointing to `deploy/selfhost/compose.yaml` and `ghcr.io/dimmihq/dimmi`.
6. **Real proof:** at least 1 real capture or terminal output listed in `meta.proof` and verified on disk.
7. **Visible questions (no `FAQPage` schema):** answer what hardware is needed, how upgrades work via `dimmictl`, whether any features are held back (none), and Kubernetes support (state plainly that there is no public Helm chart today).

## Internal linking and navigation

- Point the "Self-host" link in `site-nav.tsx`, `site-nav-mobile.tsx`, and `site-footer.tsx` to `/self-host` (keep the `#self-host` section on `/`).
- Link out from `<main>` to `/docs/self-hosting/`, `/docs/cloud-vs-self-hosted/`, `/pricing`, and GitHub (with the star button).
- Pass all publish gate checks, including 65% uniqueness, 0 slop hits, and `scripts/check-site-paths.sh`.
