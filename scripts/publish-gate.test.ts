import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, it } from "node:test";

import { runPublishGate } from "./publish-gate.ts";

function writeFile(root: string, rel: string, content: string): void {
  const full = join(root, rel);
  mkdirSync(join(full, ".."), { recursive: true });
  writeFileSync(full, content, "utf8");
}

function makeUniqueWords(prefix: string, count: number): string {
  const words: string[] = [];
  for (let i = 0; i < count; i += 1) {
    words.push(`${prefix}w${i}token`);
  }
  return words.join(" ");
}

function buildHtml(opts: {
  title: string;
  description: string;
  h1: string;
  mainBody: string;
  schemas?: Record<string, unknown>[];
  published?: string;
  updated?: string;
}): string {
  const ld = (opts.schemas ?? [])
    .map((s) => `<script type="application/ld+json">${JSON.stringify(s)}</script>`)
    .join("\n");
  const dates =
    opts.published && opts.updated
      ? `<time datetime="${opts.published}">${opts.published}</time><time datetime="${opts.updated}">${opts.updated}</time>`
      : "";
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>${opts.title}</title>
  <meta name="description" content="${opts.description}" />
  ${ld}
</head>
<body>
  <main>
    <h1>${opts.h1}</h1>
    ${dates}
    ${opts.mainBody}
  </main>
</body>
</html>`;
}

function makeFixture(): { root: string; dist: string; cleanup: () => void } {
  const root = mkdtempSync(join(tmpdir(), "dimmi-publish-gate-"));
  const dist = join(root, "site", "dist");
  mkdirSync(dist, { recursive: true });
  writeFile(root, "site/public/shots/proof.webp", "fake-webp");
  writeFile(root, "site/pace.json", JSON.stringify({ mode: "start", maxPerRelease: 20, maxPerWeek: 5 }));
  writeFile(root, "site/publish-gate-allowlist.json", "{}");

  const homeBody = `<p>${makeUniqueWords("home", 260)}</p>
    <a href="/pricing">Pricing</a>
    <a href="/vs/acme">Compare Acme</a>
    <a href="/self-host">Self-host</a>`;
  const pricingBody = `<p>${makeUniqueWords("pricing", 260)}</p>
    <a href="/">Home</a>
    <a href="/vs/acme">Compare Acme</a>
    <a href="/self-host">Self-host</a>`;
  const selfHostBody = `<p>Run dimmi on your own Linux server for $0 under MIT. ${makeUniqueWords("selfhost", 260)}</p>
    <img src="/shots/proof.webp" alt="Self-host install screen" width="1200" height="720" />
    <a href="/">Home</a>
    <a href="/pricing">Pricing</a>
    <a href="/vs/acme">Compare Acme</a>`;

  writeFile(
    dist,
    "index.html",
    buildHtml({
      title: "dimmi: open-source customer support platform",
      description:
        "Open-source MIT customer support platform with a shared inbox, AI support agents, help center, workflows, and free seats for teams on cloud or self-hosted.",
      h1: "The open customer support platform",
      mainBody: homeBody,
    }),
  );
  writeFile(
    dist,
    "pricing/index.html",
    buildHtml({
      title: "Pricing: pay per conversation, never per seat | dimmi",
      description:
        "Unlimited free seats, 100 free conversations every month, and bring your own AI model keys on dimmi Cloud. Free to self-host under the MIT license forever.",
      h1: "Pay for conversations, never for seats",
      mainBody: pricingBody,
    }),
  );
  writeFile(
    dist,
    "self-host/index.html",
    buildHtml({
      title: "Self-hosted help desk and live chat under MIT | dimmi",
      description:
        "Run dimmi on your own Linux server in about 5 minutes with the same MIT code as dimmi Cloud: shared inbox, AI agents, help center, and workflows for $0.",
      h1: "Self-host your customer support stack for $0",
      mainBody: selfHostBody,
      published: "2026-11-27",
      updated: "2026-11-27",
      schemas: [
        {
          "@context": "https://schema.org",
          "@type": "WebPage",
          datePublished: "2026-11-27",
          dateModified: "2026-11-27",
          author: { "@type": "Person", name: "Frantz Kati" },
        },
      ],
    }),
  );

  return {
    root,
    dist,
    cleanup: () => rmSync(root, { recursive: true, force: true }),
  };
}

function makeCleanVsPage(
  _root: string,
  dist: string,
  slug: string,
  prefix: string,
  extra?: {
    bodyPrefix?: string;
    schemas?: Record<string, unknown>[];
    description?: string;
    h1?: string;
    proof?: string[];
    reviewedBy?: string;
    copyFromBody?: string;
  },
) {
  const answer = "Pick Acme if you need legacy phone routing. Pick dimmi for free seats and your own AI key.";
  const uniqueCopy = extra?.copyFromBody ?? makeUniqueWords(prefix, 950);
  const body = `<p>${extra?.bodyPrefix ?? answer}</p>
    <p>${uniqueCopy}</p>
    <img src="/shots/proof.webp" alt="Usage settings screen in dimmi" width="1200" height="720" />
    <a href="/">Home</a>
    <a href="/pricing">Pricing</a>
    <a href="/self-host">Self-host</a>`;
  const schemas = extra?.schemas ?? [
    {
      "@context": "https://schema.org",
      "@type": "WebPage",
      datePublished: "2026-11-27",
      dateModified: "2026-11-27",
      author: { "@type": "Person", name: "Frantz Kati" },
      reviewedBy: { "@type": "Person", name: "Frantz Kati" },
    },
    {
      "@context": "https://schema.org",
      "@type": "BreadcrumbList",
      itemListElement: [],
    },
  ];
  writeFile(
    dist,
    `vs/${slug}/index.html`,
    buildHtml({
      title: `dimmi vs ${slug}: open-source alternative`,
      description:
        extra?.description ??
        `Compare dimmi and ${slug} side by side: seat pricing, AI usage costs, open-source self-hosting, and real monthly bills for 1, 5, and 25 person teams.`,
      h1: extra?.h1 ?? "Free seats and your own AI model key",
      mainBody: body,
      schemas,
      published: "2026-11-27",
      updated: "2026-11-27",
    }),
  );
  return {
    path: `/vs/${slug}`,
    title: `dimmi vs ${slug}: open-source alternative`,
    description:
      extra?.description ??
      `Compare dimmi and ${slug} side by side: seat pricing, AI usage costs, open-source self-hosting, and real monthly bills for 1, 5, and 25 person teams.`,
    meta: {
      type: "vs",
      keyword: `${slug} alternative`,
      answer,
      author: "frantz-kati",
      published: "2026-11-27",
      updated: "2026-11-27",
      status: "published",
      reviewedBy: extra?.reviewedBy ?? "frantz-kati",
      reviewedOn: "2026-11-27",
      rivals: [{ name: slug, checkedOn: "2026-11-27", source: `https://${slug}.example/pricing` }],
      proof: extra?.proof ?? ["public/shots/proof.webp"],
    },
  };
}

function writeSitemapAndManifest(dist: string, pages: unknown[], urls: string[]): void {
  const selfHostEntry = {
    path: "/self-host",
    title: "Self-hosted help desk and live chat under MIT | dimmi",
    description:
      "Run dimmi on your own Linux server in about 5 minutes with the same MIT code as dimmi Cloud: shared inbox, AI agents, help center, and workflows for $0.",
    meta: {
      type: "self-host",
      keyword: "self-hosted help desk",
      answer: "Run dimmi on your own Linux server for $0 under MIT.",
      author: "frantz-kati",
      published: "2026-11-27",
      updated: "2026-11-27",
      status: "published",
      reviewedBy: "frantz-kati",
      reviewedOn: "2026-11-27",
      proof: ["public/shots/proof.webp"],
    },
  };
  writeFile(dist, "pages.json", JSON.stringify([selfHostEntry, ...pages], null, 2));
  const xml = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls
    .map((u) => `  <url><loc>https://trydimmi.com${u}</loc><lastmod>2026-11-27</lastmod></url>`)
    .join("\n")}\n</urlset>\n`;
  writeFile(dist, "sitemap.xml", xml);
}

describe("publish-gate", () => {
  it("passes a clean sample /vs page and records uniqueness metrics", async () => {
    const fx = makeFixture();
    try {
      const vs = makeCleanVsPage(fx.root, fx.dist, "acme", "acme");
      writeSitemapAndManifest(fx.dist, [vs], ["/", "/pricing", "/self-host", "/vs/acme"]);

      const report = await runPublishGate(fx.dist, {
        repoRoot: fx.root,
        today: "2026-11-27",
        silent: true,
        writeReport: true,
      });

      assert.equal(report.summary.fail, 0, JSON.stringify(report.results.filter((r) => r.status === "FAIL")));
      const vsMetric = report.uniqueness.find((u) => u.route === "/vs/acme");
      assert.ok(vsMetric);
      assert.ok(vsMetric.score >= 65);
      assert.ok(vsMetric.uniqueWords >= 900);
    } finally {
      fx.cleanup();
    }
  });

  it("fails a /vs page with no proof, FAQPage schema, copied description, rival in H1, or bad reviewer", async () => {
    const fx = makeFixture();
    try {
      const copiedDesc =
        "Unlimited free seats, 100 free conversations every month, and bring your own AI model keys on dimmi Cloud. Free to self-host under the MIT license forever.";
      const vs = makeCleanVsPage(fx.root, fx.dist, "acme", "acme", {
        proof: [],
        description: copiedDesc,
        h1: "Why teams leave Intercom for dimmi",
        reviewedBy: "bot-account",
        schemas: [
          {
            "@context": "https://schema.org",
            "@type": "FAQPage",
            mainEntity: [],
          },
        ],
      });
      writeSitemapAndManifest(fx.dist, [vs], ["/", "/pricing", "/self-host", "/vs/acme"]);

      const report = await runPublishGate(fx.dist, {
        repoRoot: fx.root,
        today: "2026-11-27",
        silent: true,
      });

      const failedChecks = new Set(report.results.filter((r) => r.status === "FAIL").map((r) => r.check));
      assert.ok(failedChecks.has("proof"), "expected missing proof to fail");
      assert.ok(failedChecks.has("faq-schema"), "expected FAQPage schema to fail");
      assert.ok(failedChecks.has("duplicate-description"), "expected copied description to fail");
      assert.ok(failedChecks.has("h1-rival"), "expected rival name in H1 to fail");
      assert.ok(failedChecks.has("signoff"), "expected non-frantz-kati reviewer to fail");
    } finally {
      fx.cleanup();
    }
  });

  it("fails a /vs page under 65% unique and reports score, unique words, template words, and closest page", async () => {
    const fx = makeFixture();
    try {
      const sharedText = makeUniqueWords("sharedvs", 950);
      const vs1 = makeCleanVsPage(fx.root, fx.dist, "acme", "acme", { copyFromBody: sharedText });
      const vs2 = makeCleanVsPage(fx.root, fx.dist, "beta", "beta", { copyFromBody: sharedText });
      writeSitemapAndManifest(fx.dist, [vs1, vs2], ["/", "/pricing", "/self-host", "/vs/acme", "/vs/beta"]);

      const report = await runPublishGate(fx.dist, {
        repoRoot: fx.root,
        today: "2026-11-27",
        silent: true,
      });

      const uniqFail = report.results.find((r) => r.check === "uniqueness" && r.status === "FAIL");
      assert.ok(uniqFail, "expected uniqueness check to fail on duplicate /vs pages");
      assert.match(uniqFail.message, /score=/);
      assert.match(uniqFail.message, /uniqueWords=/);
      assert.match(uniqFail.message, /templateWords=/);
      assert.match(uniqFail.message, /closest=/);
    } finally {
      fx.cleanup();
    }
  });

  it("warns only on generated API reference uniqueness while still failing shared descriptions", async () => {
    const fx = makeFixture();
    try {
      const sharedApiBody = `<p>${makeUniqueWords("apiref", 200)}</p>
        <a href="/">Home</a><a href="/pricing">Pricing</a><a href="/self-host">Self-host</a>`;
      const sharedDesc =
        "Manage workspace conversations via the dimmi REST API reference endpoint with bearer token authentication and cursor pagination support.";
      writeFile(
        fx.dist,
        "docs/api-reference/conversations/list/index.html",
        buildHtml({
          title: "List conversations | dimmi API",
          description: sharedDesc,
          h1: "List conversations",
          mainBody: sharedApiBody,
        }),
      );
      writeFile(
        fx.dist,
        "docs/api-reference/conversations/get/index.html",
        buildHtml({
          title: "Get conversation | dimmi API",
          description: sharedDesc,
          h1: "Get conversation",
          mainBody: sharedApiBody,
        }),
      );
      writeSitemapAndManifest(
        fx.dist,
        [],
        [
          "/",
          "/pricing",
          "/self-host",
          "/docs/api-reference/conversations/list/",
          "/docs/api-reference/conversations/get/",
        ],
      );

      const report = await runPublishGate(fx.dist, {
        repoRoot: fx.root,
        today: "2026-11-27",
        silent: true,
      });

      const apiUniqFails = report.results.filter(
        (r) => r.check === "uniqueness" && r.route?.startsWith("/docs/api-reference/") && r.status === "FAIL",
      );
      const apiUniqWarns = report.results.filter(
        (r) => r.check === "uniqueness" && r.route?.startsWith("/docs/api-reference/") && r.status === "WARN",
      );
      const apiDescFails = report.results.filter(
        (r) =>
          r.check === "duplicate-description" && r.route?.startsWith("/docs/api-reference/") && r.status === "FAIL",
      );
      assert.equal(apiUniqFails.length, 0, "API reference uniqueness should WARN, not FAIL");
      assert.ok(apiUniqWarns.length > 0, "API reference under 65% unique should emit WARN");
      assert.ok(apiDescFails.length > 0, "shared description on API reference pages must still FAIL");
    } finally {
      fx.cleanup();
    }
  });
});
