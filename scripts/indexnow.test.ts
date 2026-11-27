/*
 * node --test scripts/indexnow.test.ts
 */
import assert from "node:assert/strict";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, it } from "node:test";

import { chunkUrls, parseCliArgs, parseSitemapXml, readIndexNowKey, selectUrls, submitIndexNow } from "./indexnow.ts";

const SAMPLE_KEY = "0123456789abcdef0123456789abcdef";

const CURRENT_SITEMAP = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://trydimmi.com/</loc>
    <lastmod>2026-11-27</lastmod>
  </url>
  <url>
    <loc>https://trydimmi.com/pricing</loc>
    <lastmod>2026-11-20</lastmod>
  </url>
  <url>
    <loc>https://trydimmi.com/docs/quickstart/</loc>
    <lastmod>2026-11-27</lastmod>
  </url>
</urlset>
`;

const PREVIOUS_SITEMAP = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://trydimmi.com/</loc>
    <lastmod>2026-11-15</lastmod>
  </url>
  <url>
    <loc>https://trydimmi.com/pricing</loc>
    <lastmod>2026-11-20</lastmod>
  </url>
</urlset>
`;

describe("indexnow", () => {
  it("parses sitemap entries and deduplicates URLs", () => {
    const xml = `
      <urlset>
        <url><loc>https://trydimmi.com/</loc><lastmod>2026-11-27</lastmod></url>
        <url><loc>https://trydimmi.com/</loc><lastmod>2026-11-20</lastmod></url>
        <url><loc>https://trydimmi.com/search?a=1&amp;b=2</loc></url>
      </urlset>
    `;
    const entries = parseSitemapXml(xml);
    assert.deepEqual(entries, [
      { loc: "https://trydimmi.com/", lastmod: "2026-11-27" },
      { loc: "https://trydimmi.com/search?a=1&b=2", lastmod: null },
    ]);
  });

  it("filters only new or changed URLs against a previous sitemap and supports --since", () => {
    const current = parseSitemapXml(CURRENT_SITEMAP);
    const previous = parseSitemapXml(PREVIOUS_SITEMAP);

    assert.deepEqual(selectUrls(current, { previous, host: "trydimmi.com" }), [
      "https://trydimmi.com/",
      "https://trydimmi.com/docs/quickstart/",
    ]);

    assert.deepEqual(selectUrls(current, { since: "2026-11-25", host: "trydimmi.com" }), [
      "https://trydimmi.com/",
      "https://trydimmi.com/docs/quickstart/",
    ]);
  });

  it("chunks URLs into batches of the requested max size", () => {
    assert.deepEqual(chunkUrls(["a", "b", "c", "d", "e"], 2), [["a", "b"], ["c", "d"], ["e"]]);
  });

  it("reads the committed IndexNow key file from site/public", () => {
    const key = readIndexNowKey();
    assert.match(key, /^[0-9a-f]{32}$/);
  });

  it("parses CLI flags cleanly", () => {
    const parsed = parseCliArgs([
      "--sitemap",
      "https://trydimmi.com/sitemap.xml",
      "--previous-sitemap",
      "/tmp/prev.xml",
      "--since",
      "2026-11-27",
      "--dry-run",
    ]);
    assert.deepEqual(parsed, {
      sitemap: "https://trydimmi.com/sitemap.xml",
      previousSitemap: "/tmp/prev.xml",
      since: "2026-11-27",
      dryRun: true,
    });
  });

  it("supports dry-run without making network requests", async () => {
    const dir = mkdtempSync(join(tmpdir(), "dimmi-indexnow-dry-"));
    try {
      const sitemapPath = join(dir, "sitemap.xml");
      writeFileSync(sitemapPath, CURRENT_SITEMAP, "utf8");
      const logs: string[] = [];
      let fetchCalls = 0;

      const result = await submitIndexNow({
        sitemap: sitemapPath,
        key: SAMPLE_KEY,
        dryRun: true,
        log: (msg) => logs.push(msg),
        fetchImpl: (async () => {
          fetchCalls += 1;
          return new Response(null, { status: 200 });
        }) as typeof fetch,
      });

      assert.equal(fetchCalls, 0);
      assert.equal(result.submitted, 3);
      assert.equal(result.batches, 1);
      assert.deepEqual(result.payloads[0], {
        host: "trydimmi.com",
        key: SAMPLE_KEY,
        keyLocation: `https://trydimmi.com/${SAMPLE_KEY}.txt`,
        urlList: ["https://trydimmi.com/", "https://trydimmi.com/pricing", "https://trydimmi.com/docs/quickstart/"],
      });
      assert.match(logs[0] ?? "", /dry run/i);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("fetches remote sitemaps, diffs lastmod, batches POSTs, and logs 200/202 statuses", async () => {
    const dir = mkdtempSync(join(tmpdir(), "dimmi-indexnow-post-"));
    try {
      const prevPath = join(dir, "previous-sitemap.xml");
      writeFileSync(prevPath, PREVIOUS_SITEMAP, "utf8");

      const fetched: string[] = [];
      const posted: { url: string; body: unknown }[] = [];
      const logs: string[] = [];

      const mockFetch = (async (input: string | URL | Request, init?: RequestInit) => {
        const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
        if (init?.method !== "POST") {
          fetched.push(url);
          return new Response(CURRENT_SITEMAP, { status: 200 });
        }
        posted.push({ url, body: JSON.parse(String(init.body)) });
        return new Response(null, { status: posted.length === 1 ? 200 : 202 });
      }) as typeof fetch;

      const result = await submitIndexNow({
        sitemap: "https://trydimmi.com/sitemap.xml",
        previousSitemap: prevPath,
        key: SAMPLE_KEY,
        batchSize: 1,
        fetchImpl: mockFetch,
        log: (msg) => logs.push(msg),
      });

      assert.deepEqual(fetched, ["https://trydimmi.com/sitemap.xml"]);
      assert.equal(result.submitted, 2);
      assert.equal(result.batches, 2);
      assert.deepEqual(result.statuses, [200, 202]);
      assert.deepEqual(posted, [
        {
          url: "https://api.indexnow.org/indexnow",
          body: {
            host: "trydimmi.com",
            key: SAMPLE_KEY,
            keyLocation: `https://trydimmi.com/${SAMPLE_KEY}.txt`,
            urlList: ["https://trydimmi.com/"],
          },
        },
        {
          url: "https://api.indexnow.org/indexnow",
          body: {
            host: "trydimmi.com",
            key: SAMPLE_KEY,
            keyLocation: `https://trydimmi.com/${SAMPLE_KEY}.txt`,
            urlList: ["https://trydimmi.com/docs/quickstart/"],
          },
        },
      ]);
      assert.match(logs[0] ?? "", /HTTP 200/);
      assert.match(logs[1] ?? "", /HTTP 202/);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("skips submission when no URLs changed", async () => {
    const dir = mkdtempSync(join(tmpdir(), "dimmi-indexnow-noop-"));
    try {
      const sitemapPath = join(dir, "sitemap.xml");
      writeFileSync(sitemapPath, CURRENT_SITEMAP, "utf8");
      const logs: string[] = [];
      let postCalls = 0;

      const result = await submitIndexNow({
        sitemap: sitemapPath,
        previousSitemap: sitemapPath,
        key: SAMPLE_KEY,
        log: (msg) => logs.push(msg),
        fetchImpl: (async () => {
          postCalls += 1;
          return new Response(null, { status: 200 });
        }) as typeof fetch,
      });

      assert.equal(postCalls, 0);
      assert.equal(result.submitted, 0);
      assert.equal(result.batches, 0);
      assert.match(logs[0] ?? "", /no changed URLs/i);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
});
