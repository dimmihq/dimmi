/*
 * node --test scripts/generate-sitemap.test.ts
 */
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { describe, it } from "node:test";

import { generateSitemap } from "./generate-sitemap.ts";

function writeFile(root: string, rel: string, content: string): void {
  const full = join(root, rel);
  mkdirSync(dirname(full), { recursive: true });
  writeFileSync(full, content, "utf8");
}

function git(cwd: string, args: string[], date?: string): void {
  const env = date ? { ...process.env, GIT_AUTHOR_DATE: date, GIT_COMMITTER_DATE: date } : process.env;
  execFileSync("git", args, { cwd, env, stdio: "ignore" });
}

function parseSitemap(xml: string): Record<string, string> {
  const entries: Record<string, string> = {};
  const re = /<url>\s*<loc>([^<]+)<\/loc>\s*<lastmod>([^<]+)<\/lastmod>\s*<\/url>/g;
  for (const match of xml.matchAll(re)) {
    entries[match[1]] = match[2];
  }
  return entries;
}

function createFixtureRepo(): string {
  const repo = mkdtempSync(join(tmpdir(), "dimmi-sitemap-test-"));
  git(repo, ["init", "-b", "main"]);
  git(repo, ["config", "user.name", "Test"]);
  git(repo, ["config", "user.email", "test@example.com"]);

  // Commit 1 (2026-01-10): base pages, pricing, shared product-page, docs
  writeFile(
    repo,
    "site/src/routes.ts",
    `import { HomePage } from "./pages/home";
import { PricingPage } from "./pages/pricing";
import { ComparePage } from "./pages/compare";

export const routes = [
  { path: "/", title: "Home", description: "Home", page: HomePage },
  {
    path: "/pricing",
    title: "Pricing",
    description: "Pricing",
    meta: { type: "pricing", updated: "2026-01-10" },
    page: PricingPage,
  },
  { path: "/compare", title: "Compare", description: "Compare", page: ComparePage },
];
`,
  );
  writeFile(repo, "site/src/components/product-page.tsx", `export function Questions() { return null; }\n`);
  writeFile(repo, "site/src/sections/hero.tsx", `export function Hero() { return "v1"; }\n`);
  writeFile(
    repo,
    "site/src/pages/home.tsx",
    `import { Hero } from "../sections/hero";\nexport function HomePage() { return Hero(); }\n`,
  );
  writeFile(
    repo,
    "site/src/pages/pricing.tsx",
    `import { Questions } from "../components/product-page";\nexport function PricingPage() { return Questions(); }\n`,
  );
  writeFile(repo, "site/src/pages/compare.tsx", `export function ComparePage() { return "compare"; }\n`);
  writeFile(repo, "docs/src/content/docs/index.mdx", `# Docs\n`);
  writeFile(repo, "docs/src/content/docs/quickstart.mdx", `# Quickstart\n`);
  writeFile(repo, "tests/contracts/api_v1.openapi.json", `{"openapi":"3.1.0"}\n`);
  git(repo, ["add", "."]);
  git(repo, ["commit", "-m", "initial"], "2026-01-10T12:00:00Z");

  // Commit 2 (2026-02-14): update imported hero section (should bump / only)
  writeFile(repo, "site/src/sections/hero.tsx", `export function Hero() { return "v2"; }\n`);
  git(repo, ["add", "site/src/sections/hero.tsx"]);
  git(repo, ["commit", "-m", "update hero"], "2026-02-14T12:00:00Z");

  // Commit 3 (2026-03-05): update docs quickstart
  writeFile(repo, "docs/src/content/docs/quickstart.mdx", `# Quickstart v2\n`);
  git(repo, ["add", "docs/src/content/docs/quickstart.mdx"]);
  git(repo, ["commit", "-m", "update quickstart"], "2026-03-05T12:00:00Z");

  // Commit 4 (2026-04-20): update openapi contract and shared product-page component
  writeFile(repo, "tests/contracts/api_v1.openapi.json", `{"openapi":"3.1.0","info":{"version":"2"}}\n`);
  writeFile(repo, "site/src/components/product-page.tsx", `export function Questions() { return "no schema"; }\n`);
  git(repo, ["add", "tests/contracts/api_v1.openapi.json", "site/src/components/product-page.tsx"]);
  git(repo, ["commit", "-m", "update openapi and shared component"], "2026-04-20T12:00:00Z");

  return repo;
}

function populateDist(distDir: string): void {
  writeFile(distDir, "index.html", "<html></html>");
  writeFile(distDir, "pricing/index.html", "<html></html>");
  writeFile(distDir, "compare/index.html", "<html></html>");
  writeFile(distDir, "404/index.html", "<html></html>");
  writeFile(distDir, "docs/index.html", "<html></html>");
  writeFile(distDir, "docs/quickstart/index.html", "<html></html>");
  writeFile(distDir, "docs/api-reference/conversations/list-conversations/index.html", "<html></html>");
}

describe("generateSitemap", () => {
  it("resolves per-URL lastmod from pages.json, marketing imports, docs, and openapi", () => {
    const repo = createFixtureRepo();
    try {
      const distDir = join(repo, "site", "dist");
      populateDist(distDir);
      writeFile(distDir, "pages.json", JSON.stringify([{ path: "/compare", meta: { updated: "2026-05-01" } }]));

      const warnings: string[] = [];
      const count = generateSitemap(distDir, {
        repoRoot: repo,
        buildDate: "2026-06-01",
        warn: (msg) => warnings.push(msg),
      });

      assert.equal(count, 6);
      assert.deepEqual(warnings, []);

      const xml1 = readFileSync(join(distDir, "sitemap.xml"), "utf8");
      const map = parseSitemap(xml1);

      // Home picks up imported sections/hero.tsx commit date
      assert.equal(map["https://trydimmi.com/"], "2026-02-14");
      // Pricing keeps its own page commit date (not bumped by shared components/product-page.tsx)
      assert.equal(map["https://trydimmi.com/pricing"], "2026-01-10");
      // pages.json meta.updated overrides git history
      assert.equal(map["https://trydimmi.com/compare"], "2026-05-01");
      // Docs use their respective .mdx files
      assert.equal(map["https://trydimmi.com/docs/"], "2026-01-10");
      assert.equal(map["https://trydimmi.com/docs/quickstart/"], "2026-03-05");
      // Generated API reference pages use tests/contracts/api_v1.openapi.json
      assert.equal(map["https://trydimmi.com/docs/api-reference/conversations/list-conversations/"], "2026-04-20");

      // Second run produces byte-for-byte identical output
      const warnings2: string[] = [];
      generateSitemap(distDir, {
        repoRoot: repo,
        buildDate: "2026-06-01",
        warn: (msg) => warnings2.push(msg),
      });
      assert.deepEqual(warnings2, []);
      const xml2 = readFileSync(join(distDir, "sitemap.xml"), "utf8");
      assert.equal(xml2, xml1);
    } finally {
      rmSync(repo, { recursive: true, force: true });
    }
  });

  it("warns and falls back to buildDate on a fetch-depth 1 shallow clone", () => {
    const origin = createFixtureRepo();
    const shallow = mkdtempSync(join(tmpdir(), "dimmi-sitemap-shallow-"));
    try {
      git(shallow, ["clone", "--depth=1", `file://${origin}`, "."]);
      const distDir = join(shallow, "site", "dist");
      populateDist(distDir);
      writeFile(distDir, "pages.json", JSON.stringify([{ path: "/compare", meta: { updated: "2026-05-01" } }]));

      const warnings: string[] = [];
      generateSitemap(distDir, {
        repoRoot: shallow,
        buildDate: "2026-06-01",
        warn: (msg) => warnings.push(msg),
      });

      assert.equal(warnings.length, 1);
      assert.match(warnings[0] ?? "", /shallow git history/i);

      const map = parseSitemap(readFileSync(join(distDir, "sitemap.xml"), "utf8"));
      assert.equal(map["https://trydimmi.com/"], "2026-06-01");
      assert.equal(map["https://trydimmi.com/pricing"], "2026-06-01");
      assert.equal(map["https://trydimmi.com/compare"], "2026-05-01");
    } finally {
      rmSync(origin, { recursive: true, force: true });
      rmSync(shallow, { recursive: true, force: true });
    }
  });

  it("warns and falls back to buildDate when git is unavailable", () => {
    const dir = mkdtempSync(join(tmpdir(), "dimmi-sitemap-nogit-"));
    try {
      const distDir = join(dir, "site", "dist");
      populateDist(distDir);

      const warnings: string[] = [];
      generateSitemap(distDir, {
        repoRoot: dir,
        buildDate: "2026-06-01",
        warn: (msg) => warnings.push(msg),
      });

      assert.equal(warnings.length, 1);
      assert.match(warnings[0] ?? "", /git history is unavailable/i);

      const map = parseSitemap(readFileSync(join(distDir, "sitemap.xml"), "utf8"));
      assert.equal(map["https://trydimmi.com/"], "2026-06-01");
      assert.equal(map["https://trydimmi.com/docs/quickstart/"], "2026-06-01");
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
});
