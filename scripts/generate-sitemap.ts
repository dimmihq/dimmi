/*
 * Builds site/dist/sitemap.xml from every prerendered HTML page in site/dist,
 * covering both the marketing site (/) and the Astro docs (/docs/) when built.
 */
import { readdirSync, statSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const SITE_URL = "https://trydimmi.com";

function walkHtml(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      walkHtml(full, out);
    } else if (name === "index.html") {
      out.push(full);
    }
  }
  return out;
}

function toUrl(distDir: string, file: string): string | null {
  const rel = relative(distDir, file).replace(/\\/g, "/");
  if (rel === "index.html") {
    return `${SITE_URL}/`;
  }
  const route = `/${rel.replace(/\/index\.html$/, "")}`;
  if (route === "/404" || route === "/docs/404") {
    return null;
  }
  // Astro docs routes end with a trailing slash; marketing routes do not.
  if (route === "/docs" || route.startsWith("/docs/")) {
    return `${SITE_URL}${route}/`;
  }
  return `${SITE_URL}${route}`;
}

export function generateSitemap(distDir = fileURLToPath(new URL("../site/dist", import.meta.url))): number {
  const urls = walkHtml(distDir)
    .map((file) => toUrl(distDir, file))
    .filter((u): u is string => u !== null)
    .sort((a, b) => {
      if (a === `${SITE_URL}/`) {
        return -1;
      }
      if (b === `${SITE_URL}/`) {
        return 1;
      }
      const aDocs = a.includes("/docs/");
      const bDocs = b.includes("/docs/");
      if (aDocs !== bDocs) {
        return aDocs ? 1 : -1;
      }
      return a.localeCompare(b);
    });

  const lastmod = new Date().toISOString().slice(0, 10);
  const body = urls
    .map((loc) => `  <url>\n    <loc>${loc}</loc>\n    <lastmod>${lastmod}</lastmod>\n  </url>`)
    .join("\n");
  const xml = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${body}\n</urlset>\n`;
  writeFileSync(join(distDir, "sitemap.xml"), xml, "utf8");
  return urls.length;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const count = generateSitemap();
  process.stdout.write(`sitemap: wrote ${count} URLs to site/dist/sitemap.xml\n`);
}
