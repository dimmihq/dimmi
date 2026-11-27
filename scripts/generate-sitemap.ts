/*
 * Builds site/dist/sitemap.xml from every prerendered HTML page in site/dist,
 * covering both the marketing site (/) and the Astro docs (/docs/) when built.
 *
 * Each URL gets its own <lastmod> date:
 * - Pages with `meta.updated` in site/dist/pages.json use that date.
 * - Other marketing pages use the last git commit to the page file and the
 *   local content/section files it imports inside site/src/.
 * - Docs pages use the last git commit to their file in docs/src/content/docs/.
 * - Generated API reference pages use the last git commit to
 *   tests/contracts/api_v1.openapi.json.
 * - If git history is shallow or unavailable, affected URLs fall back to the
 *   build date and emit a clear warning.
 */
import { execFileSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { dirname, isAbsolute, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const SITE_URL = "https://trydimmi.com";
const OPENAPI_CONTRACT_REL = "tests/contracts/api_v1.openapi.json";
const ISO_DATE_RE = /^\d{4}-\d{2}-\d{2}/;

// Shared chrome/primitive files under site/src/ whose edits should not stamp
// every unrelated page with a new <lastmod> date.
const SHARED_SITE_FILES = new Set([
  "site/src/channels.ts",
  "site/src/flows.json",
  "site/src/links.ts",
  "site/src/products.ts",
  "site/src/shots.ts",
]);

export type GenerateSitemapOptions = {
  repoRoot?: string;
  buildDate?: string;
  warn?: (message: string) => void;
};

type UrlEntry = {
  route: string;
  loc: string;
};

type GitContext = {
  available: boolean;
  singleCommitShallow: boolean;
  shallowCommits: Set<string>;
};

type GitDateResult = {
  date: string | null;
  hitShallow: boolean;
};

function toPosix(pathValue: string): string {
  return pathValue.replace(/\\/g, "/");
}

function normalizeRoute(input: string): string {
  let route = input.trim();
  if (/^https?:\/\//i.test(route)) {
    try {
      const parsed = new URL(route);
      if (parsed.origin === SITE_URL) {
        route = parsed.pathname;
      }
    } catch {
      // Keep raw input when URL parsing fails.
    }
  }
  route = route.split(/[?#]/)[0] ?? "";
  if (!route.startsWith("/")) {
    route = `/${route}`;
  }
  route = route.replace(/\/+$/, "");
  return route || "/";
}

function parseIsoDate(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const match = ISO_DATE_RE.exec(value.trim());
  return match ? match[0] : null;
}

function isFile(filePath: string): boolean {
  return existsSync(filePath) && statSync(filePath).isFile();
}

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

function toUrlEntry(distDir: string, file: string): UrlEntry | null {
  const rel = toPosix(relative(distDir, file));
  if (rel === "index.html") {
    return { route: "/", loc: `${SITE_URL}/` };
  }
  const route = `/${rel.replace(/\/index\.html$/, "")}`;
  if (route === "/404" || route === "/docs/404") {
    return null;
  }
  // Astro docs routes end with a trailing slash; marketing routes do not.
  if (route === "/docs" || route.startsWith("/docs/")) {
    return { route, loc: `${SITE_URL}${route}/` };
  }
  return { route, loc: `${SITE_URL}${route}` };
}

function compareEntries(a: UrlEntry, b: UrlEntry): number {
  if (a.loc === `${SITE_URL}/`) {
    return -1;
  }
  if (b.loc === `${SITE_URL}/`) {
    return 1;
  }
  const aDocs = a.loc.includes("/docs/");
  const bDocs = b.loc.includes("/docs/");
  if (aDocs !== bDocs) {
    return aDocs ? 1 : -1;
  }
  return a.loc.localeCompare(b.loc);
}

function extractUpdatedFromRecord(record: unknown): string | null {
  if (!record || typeof record !== "object") {
    return null;
  }
  const obj = record as Record<string, unknown>;
  if (obj.meta && typeof obj.meta === "object") {
    const metaUpdated = parseIsoDate((obj.meta as Record<string, unknown>).updated);
    if (metaUpdated) {
      return metaUpdated;
    }
  }
  return parseIsoDate(obj.updated);
}

function extractRouteFromRecord(obj: Record<string, unknown>): string | null {
  for (const key of ["path", "route", "url"]) {
    if (typeof obj[key] === "string") {
      return obj[key];
    }
  }
  return null;
}

function collectUpdatedFromArray(items: unknown[], out: Map<string, string>): void {
  for (const item of items) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const rawRoute = extractRouteFromRecord(item as Record<string, unknown>);
    const updated = extractUpdatedFromRecord(item);
    if (rawRoute && updated) {
      out.set(normalizeRoute(rawRoute), updated);
    }
  }
}

/*
 * Reads optional site/dist/pages.json produced by the site build (issue #78)
 * and extracts route -> YYYY-MM-DD from `meta.updated`.
 */
function readPagesUpdatedMap(distDir: string): Map<string, string> {
  const map = new Map<string, string>();
  const pagesJsonPath = join(distDir, "pages.json");
  if (!existsSync(pagesJsonPath)) {
    return map;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(readFileSync(pagesJsonPath, "utf8"));
  } catch {
    return map;
  }

  if (Array.isArray(parsed)) {
    collectUpdatedFromArray(parsed, map);
    return map;
  }
  if (!parsed || typeof parsed !== "object") {
    return map;
  }
  const obj = parsed as Record<string, unknown>;
  if (Array.isArray(obj.pages)) {
    collectUpdatedFromArray(obj.pages, map);
    return map;
  }
  for (const [key, value] of Object.entries(obj)) {
    const updated = extractUpdatedFromRecord(value);
    if (updated) {
      map.set(normalizeRoute(key), updated);
    }
  }
  return map;
}

function stripComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/(^|[^:])\/\/.*$/gm, "$1");
}

function resolveModuleFile(baseDir: string, specifier: string, siteSrcDir: string): string | null {
  let targetBase: string;
  if (specifier.startsWith(".")) {
    targetBase = resolve(baseDir, specifier);
  } else if (specifier.startsWith("~site/")) {
    targetBase = resolve(siteSrcDir, specifier.slice("~site/".length));
  } else {
    return null;
  }

  const candidates = [
    targetBase,
    `${targetBase}.tsx`,
    `${targetBase}.ts`,
    `${targetBase}.mdx`,
    `${targetBase}.md`,
    join(targetBase, "index.tsx"),
    join(targetBase, "index.ts"),
  ];
  return candidates.find(isFile) ?? null;
}

function extractImportedBindings(clause: string): string[] {
  const names: string[] = [];
  const namedMatch = /\{([\s\S]*?)\}/.exec(clause);
  if (namedMatch) {
    for (const part of namedMatch[1].split(",")) {
      const cleaned = part
        .replace(/\btype\b/g, "")
        .trim()
        .split(/\s+as\s+/);
      const localName = (cleaned[1] ?? cleaned[0] ?? "").trim();
      if (localName) {
        names.push(localName);
      }
    }
  }
  const defaultPart = clause
    .replace(/\{[\s\S]*?\}/, "")
    .replace(/,/g, "")
    .trim();
  if (defaultPart && /^[A-Za-z_$][\w$]*$/.test(defaultPart)) {
    names.push(defaultPart);
  }
  return names;
}

function parseImportedPages(source: string, siteSrcDir: string): Map<string, string> {
  const importedPages = new Map<string, string>();
  const importRe = /\bimport\s+(?!type\b)([\s\S]*?)\bfrom\s*["']([^"']+)["']/g;
  for (const match of source.matchAll(importRe)) {
    const resolved = resolveModuleFile(siteSrcDir, match[2] ?? "", siteSrcDir);
    if (!resolved) {
      continue;
    }
    for (const name of extractImportedBindings(match[1] ?? "")) {
      importedPages.set(name, resolved);
    }
  }
  return importedPages;
}

function skipStringLiteral(source: string, start: number): number {
  const quote = source[start];
  for (let i = start + 1; i < source.length; i += 1) {
    if (source[i] === "\\") {
      i += 1;
    } else if (source[i] === quote) {
      return i;
    }
  }
  return source.length - 1;
}

function findMatchingBrace(source: string, openIndex: number): number {
  let depth = 0;
  for (let i = openIndex; i < source.length; i += 1) {
    const ch = source[i];
    if (ch === '"' || ch === "'" || ch === "`") {
      i = skipStringLiteral(source, i);
    } else if (ch === "{") {
      depth += 1;
    } else if (ch === "}") {
      depth -= 1;
      if (depth === 0) {
        return i;
      }
    }
  }
  return -1;
}

function extractRouteBlocks(source: string): string[] {
  const routesStart = /\bexport\s+const\s+routes\b[\s\S]*?=\s*\[/.exec(source);
  const scanFrom = routesStart ? routesStart.index + routesStart[0].length : 0;
  const blocks: string[] = [];

  for (let i = scanFrom; i < source.length; i += 1) {
    const ch = source[i];
    if (ch === '"' || ch === "'" || ch === "`") {
      i = skipStringLiteral(source, i);
    } else if (ch === "]" && routesStart) {
      break;
    } else if (ch === "{") {
      const end = findMatchingBrace(source, i);
      if (end === -1) {
        break;
      }
      blocks.push(source.slice(i, end + 1));
      i = end;
    }
  }
  return blocks;
}

function parseRoutesMap(repoRoot: string): Map<string, string> {
  const map = new Map<string, string>();
  const siteSrcDir = join(repoRoot, "site", "src");
  const routesFile = join(siteSrcDir, "routes.ts");
  if (!existsSync(routesFile)) {
    return map;
  }

  const source = stripComments(readFileSync(routesFile, "utf8"));
  const importedPages = parseImportedPages(source, siteSrcDir);

  for (const block of extractRouteBlocks(source)) {
    const pathMatch = /\bpath\s*:\s*["']([^"']+)["']/.exec(block);
    const pageMatch = /\bpage\s*:\s*([A-Za-z_$][\w$]*)/.exec(block);
    const pageFile = pageMatch ? importedPages.get(pageMatch[1]) : undefined;
    if (pathMatch && pageFile) {
      map.set(normalizeRoute(pathMatch[1]), pageFile);
    }
  }
  return map;
}

function isSharedSiteFile(repoRel: string): boolean {
  return repoRel.startsWith("site/src/components/") || SHARED_SITE_FILES.has(repoRel);
}

function isLocalSpecifier(spec: string | undefined): spec is string {
  return Boolean(spec && (spec.startsWith(".") || spec.startsWith("~site/")));
}

function extractLocalSpecifiers(filePath: string): string[] {
  const source = stripComments(readFileSync(filePath, "utf8"));
  const specifiers: string[] = [];
  const importFromRe = /\b(?:import|export)\s+(?!type\b)[^;]*?\bfrom\s*["']([^"']+)["']/g;
  for (const match of source.matchAll(importFromRe)) {
    if (isLocalSpecifier(match[1])) {
      specifiers.push(match[1]);
    }
  }
  const sideEffectImportRe = /\bimport\s*["']([^"']+)["']/g;
  for (const match of source.matchAll(sideEffectImportRe)) {
    if (isLocalSpecifier(match[1])) {
      specifiers.push(match[1]);
    }
  }
  return specifiers;
}

function enqueueLocalImports(filePath: string, siteSrcDir: string, queue: string[]): void {
  if (!/\.(?:tsx?|jsx?|mjs|cjs)$/.test(filePath)) {
    return;
  }
  for (const spec of extractLocalSpecifiers(filePath)) {
    const target = resolveModuleFile(dirname(filePath), spec, siteSrcDir);
    if (target) {
      queue.push(resolve(target));
    }
  }
}

/*
 * Collects the page file plus every local content/section file it imports
 * inside site/src/ (recursively), skipping shared layout primitives under
 * site/src/components/ so a site-wide template tweak does not rewrite every
 * page's <lastmod>.
 */
function collectMarketingSourceFiles(pageFile: string, repoRoot: string): string[] {
  const siteSrcDir = join(repoRoot, "site", "src");
  const collected = new Set<string>();
  const queue = [resolve(pageFile)];

  while (queue.length > 0) {
    const current = queue.shift();
    if (!current) {
      continue;
    }
    const repoRel = toPosix(relative(repoRoot, current));
    const inSite = repoRel.startsWith("site/src/") || repoRel.startsWith("site/content/");
    if (!inSite || collected.has(repoRel) || (collected.size > 0 && isSharedSiteFile(repoRel))) {
      continue;
    }
    collected.add(repoRel);
    enqueueLocalImports(current, siteSrcDir, queue);
  }

  return [...collected].sort();
}

function findDocsSourceFiles(route: string, repoRoot: string): string[] {
  const docsRoot = join(repoRoot, "docs", "src", "content", "docs");
  const slug = route === "/docs" ? "" : route.replace(/^\/docs\//, "");

  const candidates =
    slug === ""
      ? [join(docsRoot, "index.mdx"), join(docsRoot, "index.md")]
      : [
          join(docsRoot, `${slug}.mdx`),
          join(docsRoot, `${slug}.md`),
          join(docsRoot, slug, "index.mdx"),
          join(docsRoot, slug, "index.md"),
        ];

  const found = candidates.find(isFile);
  if (found) {
    return [toPosix(relative(repoRoot, found))];
  }

  if (route === "/docs/api-reference" || route.startsWith("/docs/api-reference/")) {
    const openapiAbs = join(repoRoot, OPENAPI_CONTRACT_REL);
    if (existsSync(openapiAbs)) {
      return [OPENAPI_CONTRACT_REL];
    }
  }

  return [];
}

function findMarketingFallbackPage(route: string, repoRoot: string): string | null {
  const clean = route === "/" ? "home" : route.slice(1);
  const candidates = [
    join(repoRoot, "site", "src", "pages", `${clean}.tsx`),
    join(repoRoot, "site", "src", "pages", `${clean}.ts`),
    join(repoRoot, "site", "src", "pages", clean, "index.tsx"),
    join(repoRoot, "site", "src", "pages", clean, "index.ts"),
    join(repoRoot, "site", "content", `${clean}.mdx`),
    join(repoRoot, "site", "content", `${clean}.md`),
    join(repoRoot, "site", "content", clean, "index.mdx"),
    join(repoRoot, "site", "content", clean, "index.md"),
  ];
  return candidates.find(isFile) ?? null;
}

function runGit(repoRoot: string, args: string[]): string | null {
  try {
    return execFileSync("git", args, {
      cwd: repoRoot,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return null;
  }
}

function readShallowCommits(repoRoot: string): Set<string> {
  const shallowCommits = new Set<string>();
  const shallowPath = runGit(repoRoot, ["rev-parse", "--git-path", "shallow"]);
  if (!shallowPath) {
    return shallowCommits;
  }
  const absShallow = isAbsolute(shallowPath) ? shallowPath : resolve(repoRoot, shallowPath);
  if (!existsSync(absShallow)) {
    return shallowCommits;
  }
  for (const line of readFileSync(absShallow, "utf8").split(/\r?\n/)) {
    const sha = line.trim();
    if (sha) {
      shallowCommits.add(sha);
    }
  }
  return shallowCommits;
}

function inspectGit(repoRoot: string): GitContext {
  const inside = runGit(repoRoot, ["rev-parse", "--is-inside-work-tree"]);
  const head = inside === "true" ? runGit(repoRoot, ["rev-parse", "HEAD"]) : null;
  if (!head) {
    return { available: false, singleCommitShallow: false, shallowCommits: new Set() };
  }

  const isShallow = runGit(repoRoot, ["rev-parse", "--is-shallow-repository"]) === "true";
  const shallowCommits = isShallow ? readShallowCommits(repoRoot) : new Set<string>();
  return {
    available: true,
    singleCommitShallow: isShallow && shallowCommits.has(head),
    shallowCommits,
  };
}

function parseGitLogLine(output: string | null, shallowCommits: Set<string>): GitDateResult {
  if (!output) {
    return { date: null, hitShallow: false };
  }
  const [hash = "", rawDate = ""] = output.split(/\s+/, 2);
  const date = parseIsoDate(rawDate);
  if (!date) {
    return { date: null, hitShallow: false };
  }
  if (shallowCommits.has(hash)) {
    return { date: null, hitShallow: true };
  }
  return { date, hitShallow: false };
}

function resolveRouteFiles(route: string, repoRoot: string, routesMap: Map<string, string>): string[] {
  if (route === "/docs" || route.startsWith("/docs/")) {
    return findDocsSourceFiles(route, repoRoot);
  }
  const pageFile = routesMap.get(route) ?? findMarketingFallbackPage(route, repoRoot);
  return pageFile ? collectMarketingSourceFiles(pageFile, repoRoot) : [];
}

export function generateSitemap(
  distDir = fileURLToPath(new URL("../site/dist", import.meta.url)),
  options: GenerateSitemapOptions = {},
): number {
  const repoRoot = options.repoRoot ?? fileURLToPath(new URL("..", import.meta.url));
  const buildDate = options.buildDate ?? new Date().toISOString().slice(0, 10);
  const warn = options.warn ?? ((message: string) => process.stderr.write(`${message}\n`));

  const entries = walkHtml(distDir)
    .map((file) => toUrlEntry(distDir, file))
    .filter((entry): entry is UrlEntry => entry !== null)
    .sort(compareEntries);

  const pagesUpdated = readPagesUpdatedMap(distDir);
  const routesMap = parseRoutesMap(repoRoot);
  const git = inspectGit(repoRoot);

  if (!git.available) {
    warn(`sitemap: warning: git history is unavailable; falling back to build date (${buildDate}).`);
  } else if (git.singleCommitShallow) {
    warn(
      `sitemap: warning: shallow git history detected (fetch-depth: 1); falling back to build date (${buildDate}). Use fetch-depth: 0 in CI for real dates.`,
    );
  }

  const gitDateCache = new Map<string, GitDateResult>();
  let shallowHitCount = 0;
  let missingHistoryCount = 0;

  const lookupGitDate = (files: string[]): GitDateResult => {
    if (files.length === 0) {
      return { date: null, hitShallow: false };
    }
    const key = files.join("\0");
    let cached = gitDateCache.get(key);
    if (!cached) {
      const output = runGit(repoRoot, ["log", "-1", "--format=%H %cs", "--", ...files]);
      cached = parseGitLogLine(output, git.shallowCommits);
      gitDateCache.set(key, cached);
    }
    return cached;
  };

  const resolveLastmod = (entry: UrlEntry): string => {
    const metaUpdated = pagesUpdated.get(entry.route);
    if (metaUpdated) {
      return metaUpdated;
    }
    if (!git.available || git.singleCommitShallow) {
      return buildDate;
    }
    const files = resolveRouteFiles(entry.route, repoRoot, routesMap);
    const result = lookupGitDate(files);
    if (result.date) {
      return result.date;
    }
    if (result.hitShallow) {
      shallowHitCount += 1;
    } else {
      missingHistoryCount += 1;
    }
    return buildDate;
  };

  const body = entries
    .map((entry) => `  <url>\n    <loc>${entry.loc}</loc>\n    <lastmod>${resolveLastmod(entry)}</lastmod>\n  </url>`)
    .join("\n");

  if (shallowHitCount > 0) {
    warn(
      `sitemap: warning: shallow git history reached for ${shallowHitCount} URL(s); falling back to build date (${buildDate}) for them. Use fetch-depth: 0 for full history.`,
    );
  }
  if (missingHistoryCount > 0) {
    warn(
      `sitemap: warning: git commit date missing for ${missingHistoryCount} URL(s); falling back to build date (${buildDate}) for them.`,
    );
  }

  const xml = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${body}\n</urlset>\n`;
  writeFileSync(join(distDir, "sitemap.xml"), xml, "utf8");
  return entries.length;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const count = generateSitemap();
  process.stdout.write(`sitemap: wrote ${count} URLs to site/dist/sitemap.xml\n`);
}
