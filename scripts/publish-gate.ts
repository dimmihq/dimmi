/*
 * Publish gate for the built marketing site and docs (site/dist).
 *
 * Checks sign-off metadata, shingle uniqueness, titles, descriptions, dates,
 * JSON-LD schema types, H1 rules, proof files, image dimensions, internal
 * links, rival check dates, slop patterns, canonical constants, and rollout
 * pace. Writes a detailed report to <distDir>/publish-gate.json and exits
 * non-zero on any FAIL unless --report-only is passed.
 *
 *   node scripts/publish-gate.ts [site/dist] [--baseline <url-or-file>] [--report-only]
 */
import { existsSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const SITE_URL = "https://trydimmi.com";
const ALLOWED_REVIEWER = "frantz-kati";
const ISO_DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/;
const MS_PER_DAY = 86_400_000;

const ALLOWED_SCHEMA_TYPES = new Set([
  "Organization",
  "WebSite",
  "SoftwareApplication",
  "WebPage",
  "BreadcrumbList",
  "BlogPosting",
  "WebApplication",
]);

const CONTENT_PAGE_TYPES = new Set(["vs", "alternatives", "blog", "self-host", "tool"]);

const NEVER_ALLOWLIST = new Set([
  "faq-schema",
  "schema-json",
  "duplicate-title",
  "duplicate-description",
  "signoff",
  "h1-rival",
  "proof",
]);

const RIVAL_NORMALIZE_RE =
  /\b(help\s+scout|tawk\.to|intercom|zendesk|helpscout|freshdesk|chatwoot|crisp|tawkto|helply|chatbase|front)\b/gi;
const RIVAL_H1_CI_RE =
  /\b(help\s+scout|tawk\.to|intercom|zendesk|helpscout|freshdesk|chatwoot|crisp|tawkto|helply|chatbase)\b/i;
const RIVAL_H1_FRONT_RE = /\bFront\b|\bvs\.?\s+front\b|\bfront\s+alternative\b/u;

const SLOP_RE = new RegExp(
  [
    "[\u2013\u2014\u201c\u201d\u2018\u2019]",
    "(?<=[A-Za-z0-9,)]) - (?=[A-Za-z0-9(])",
    "\\bnot (?:just|only) [^.]{1,60}?(?:\\bbut\\b|[,;] it'?s\\b)",
    "\\bit'?s not [^.,]{1,40}, it'?s\\b",
    "\\b(?:delve\\w*|pivotal|tapestry|landscape|testament|underscor\\w*|foster\\w*|garner\\w*" +
      "|interplay|intricate|crucial|vibrant|showcas\\w*|utiliz\\w*|leverag\\w*|seamless\\w*" +
      "|paradigm|bedrock|substrate|north star|flywheel|nexus|locus" +
      "|blazing-fast|groundbreaking|game-changing|cutting-edge|revolutionary|world-class|next-generation|robust" +
      "|serves as|stands as|boasts|in order to|due to the fact that|facilitat\\w*|commenc\\w*" +
      "|significantly|easily|simply|experts (?:believe|say|agree)|industry reports|studies show|many teams find" +
      "|from startups to|in this (?:article|guide|post)|let'?s dive|i hope this helps|as we have seen|in conclusion" +
      "|in today'?s fast-paced|moreover|furthermore|additionally)\\b",
  ].join("|"),
  "gi",
);

const APP_ROUTE_PREFIXES = ["/signup", "/login", "/logout", "/d", "/api", "/widget", "/mcp", "/oauth"];

const MONTHS_SHORT = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const MONTHS_LONG = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

export type CheckStatus = "PASS" | "WARN" | "FAIL";

export type PageRivalMeta = {
  name: string;
  checkedOn: string;
  source: string;
};

export type PageMetaRecord = {
  type: string;
  keyword: string;
  answer: string;
  author: string;
  published: string;
  updated: string;
  status: string;
  reviewedBy: string;
  reviewedOn: string;
  batch?: string;
  rivals?: PageRivalMeta[];
  proof?: string[];
};

export type PageManifestEntry = {
  path: string;
  title?: string;
  description?: string;
  meta?: PageMetaRecord;
};

export type PaceConfig = {
  mode: string;
  maxPerRelease: number;
  maxPerWeek: number;
};

export type AllowlistEntry = {
  reviewBy: string;
  reason: string;
  checks: string[];
};

export type AllowlistConfig = Record<string, AllowlistEntry>;

export type CheckResult = {
  check: string;
  status: CheckStatus;
  route?: string;
  message: string;
  details?: Record<string, unknown>;
};

export type UniquenessMetrics = {
  route: string;
  type: string;
  score: number;
  uniqueWords: number;
  templateWords: number;
  totalWords: number;
  closestPage: string | null;
};

export type PublishGateReport = {
  generatedAt: string;
  distDir: string;
  reportOnly: boolean;
  summary: {
    pagesChecked: number;
    pass: number;
    warn: number;
    fail: number;
  };
  uniqueness: UniquenessMetrics[];
  results: CheckResult[];
};

export type PublishGateOptions = {
  repoRoot?: string;
  baseline?: string;
  reportOnly?: boolean;
  allowlistPath?: string;
  pacePath?: string;
  today?: string;
  silent?: boolean;
  writeReport?: boolean;
};

type ParsedPage = {
  route: string;
  file: string;
  type: string;
  html: string;
  mainHtml: string;
  title: string;
  description: string;
  h1s: string[];
  jsonLdRaw: string[];
  images: ImageTagInfo[];
  allInternalLinks: string[];
  mainInternalLinks: string[];
  meta?: PageMetaRecord;
  tokens: string[];
  shingles: string[];
  shingleSet: Set<string>;
};

type ImageTagInfo = {
  src: string;
  alt: string | null;
  width: string | null;
  height: string | null;
  ariaHidden: boolean;
};

type CanonicalConstants = {
  freeConversations: number;
  sdkCount: number;
  installUrl: string;
};

function toPosix(value: string): string {
  return value.replace(/\\/g, "/");
}

function normalizeRoute(input: string): string {
  let route = input.trim();
  if (route.startsWith(SITE_URL)) {
    route = route.slice(SITE_URL.length);
  }
  route = (route.split(/[?#]/)[0] ?? "").trim();
  if (!route.startsWith("/")) {
    route = `/${route}`;
  }
  route = route.replace(/\/+$/, "");
  return route || "/";
}

function isValidIsoDate(value: unknown): value is string {
  if (typeof value !== "string") {
    return false;
  }
  const match = ISO_DATE_RE.exec(value.trim());
  if (!match) {
    return false;
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
}

function parseDateMs(value: string): number {
  const match = ISO_DATE_RE.exec(value.trim());
  if (!match) {
    return Number.NaN;
  }
  return Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
}

function daysBetween(later: string, earlier: string): number {
  return Math.floor((parseDateMs(later) - parseDateMs(earlier)) / MS_PER_DAY);
}

function isHttpUrl(value: unknown): boolean {
  if (typeof value !== "string") {
    return false;
  }
  try {
    const parsed = new URL(value);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

function decodeHtmlEntities(input: string): string {
  return input
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&quot;/gi, '"')
    .replace(/&#39;|&apos;/gi, "'")
    .replace(/&nbsp;/gi, " ")
    .replace(/&mdash;/gi, "\u2014")
    .replace(/&ndash;/gi, "\u2013")
    .replace(/&ldquo;/gi, "\u201c")
    .replace(/&rdquo;/gi, "\u201d")
    .replace(/&lsquo;/gi, "\u2018")
    .replace(/&rsquo;/gi, "\u2019")
    .replace(/&#x([0-9a-f]+);/gi, (_m, hex: string) => String.fromCodePoint(Number.parseInt(hex, 16)))
    .replace(/&#(\d+);/g, (_m, num: string) => String.fromCodePoint(Number.parseInt(num, 10)));
}

function stripTags(html: string): string {
  return decodeHtmlEntities(html.replace(/<[^>]+>/g, " "))
    .replace(/\s+/g, " ")
    .trim();
}

function extractMainHtml(html: string): string {
  const mainMatch = /<main\b[^>]*>([\s\S]*?)<\/main>/i.exec(html);
  if (mainMatch) {
    return mainMatch[1] ?? "";
  }
  const bodyMatch = /<body\b[^>]*>([\s\S]*?)<\/body>/i.exec(html);
  return bodyMatch ? (bodyMatch[1] ?? "") : html;
}

function extractTitle(html: string): string {
  const match = /<title\b[^>]*>([\s\S]*?)<\/title>/i.exec(html);
  return match ? stripTags(match[1] ?? "") : "";
}

function getAttr(tag: string, attrName: string): string | null {
  const re = new RegExp(`\\b${attrName}\\s*=\\s*(?:"([^"]*)"|'([^']*)'|([^\\s>]+))`, "i");
  const match = re.exec(tag);
  if (!match) {
    return null;
  }
  const raw = match[1] ?? match[2] ?? match[3] ?? "";
  return decodeHtmlEntities(raw);
}

function extractDescription(html: string): string {
  for (const match of html.matchAll(/<meta\b[^>]*>/gi)) {
    const tag = match[0];
    const name = getAttr(tag, "name");
    if (name?.toLowerCase() === "description") {
      return (getAttr(tag, "content") ?? "").trim();
    }
  }
  return "";
}

function extractH1s(html: string): string[] {
  const out: string[] = [];
  for (const match of html.matchAll(/<h1\b[^>]*>([\s\S]*?)<\/h1>/gi)) {
    out.push(stripTags(match[1] ?? ""));
  }
  return out;
}

function extractJsonLd(html: string): string[] {
  const blocks: string[] = [];
  for (const match of html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/gi)) {
    const attrs = match[1] ?? "";
    const type = getAttr(`<script ${attrs}>`, "type");
    if (type?.toLowerCase() === "application/ld+json") {
      blocks.push((match[2] ?? "").trim());
    }
  }
  return blocks;
}

function extractImages(html: string): ImageTagInfo[] {
  const images: ImageTagInfo[] = [];
  for (const match of html.matchAll(/<img\b[^>]*>/gi)) {
    const tag = match[0];
    images.push({
      src: getAttr(tag, "src") ?? "",
      alt: getAttr(tag, "alt"),
      width: getAttr(tag, "width"),
      height: getAttr(tag, "height"),
      ariaHidden: getAttr(tag, "aria-hidden")?.toLowerCase() === "true",
    });
  }
  return images;
}

function isAppRoute(route: string): boolean {
  return APP_ROUTE_PREFIXES.some((prefix) => route === prefix || route.startsWith(`${prefix}/`));
}

function extractInternalLinks(html: string): string[] {
  const links = new Set<string>();
  for (const match of html.matchAll(/<a\b[^>]*>/gi)) {
    const rawHref = getAttr(match[0], "href");
    if (!rawHref) {
      continue;
    }
    const href = rawHref.trim();
    if (
      href.startsWith("#") ||
      href.startsWith("mailto:") ||
      href.startsWith("tel:") ||
      href.startsWith("javascript:")
    ) {
      continue;
    }
    if (href.startsWith("http://") || href.startsWith("https://")) {
      if (!href.startsWith(SITE_URL)) {
        continue;
      }
      links.add(normalizeRoute(href));
      continue;
    }
    if (href.startsWith("/")) {
      links.add(normalizeRoute(href));
    }
  }
  return [...links];
}

function extractVisibleText(html: string, stripCode: boolean): string {
  let cleaned = html.replace(/<(script|style|svg|noscript)\b[^>]*>[\s\S]*?<\/\1>/gi, " ");
  if (stripCode) {
    cleaned = cleaned.replace(/<(code|pre)\b[^>]*>[\s\S]*?<\/\1>/gi, " ");
  }
  return stripTags(cleaned);
}

function tokenizeForShingles(text: string): string[] {
  const normalized = text
    .toLowerCase()
    .replace(RIVAL_NORMALIZE_RE, " __RIVAL__ ")
    .replace(/[$€£¥]?\d+(?:[.,]\d+)*(?:[%¢kKmM])?/g, " __NUM__ ");
  const matches = normalized.match(/__rival__|__num__|[a-z0-9]+/gi);
  return matches ? matches.map((token) => token.toLowerCase()) : [];
}

function buildShingles(tokens: string[], size = 5): string[] {
  if (tokens.length < size) {
    return [];
  }
  const shingles: string[] = [];
  for (let i = 0; i <= tokens.length - size; i += 1) {
    shingles.push(tokens.slice(i, i + size).join(" "));
  }
  return shingles;
}

function inferPageType(route: string, meta?: PageMetaRecord): string {
  if (meta?.type) {
    return meta.type;
  }
  if (route === "/docs/api-reference" || route.startsWith("/docs/api-reference/")) {
    return "api-reference";
  }
  if (route === "/docs" || route.startsWith("/docs/")) {
    return "docs";
  }
  if (route === "/vs" || route.startsWith("/vs/")) {
    return "vs";
  }
  if (route === "/alternatives" || route.startsWith("/alternatives/") || route === "/open-source-intercom") {
    return "alternatives";
  }
  if (route === "/blog" || route.startsWith("/blog/")) {
    return "blog";
  }
  if (route === "/self-host") {
    return "self-host";
  }
  if (route === "/tools" || route.startsWith("/tools/")) {
    return "tool";
  }
  return "marketing";
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

function fileToRoute(distDir: string, file: string): string | null {
  const rel = toPosix(relative(distDir, file));
  if (rel === "index.html") {
    return "/";
  }
  const route = `/${rel.replace(/\/index\.html$/, "")}`;
  if (route === "/404" || route === "/docs/404") {
    return null;
  }
  return route;
}

function readPagesManifest(distDir: string): Map<string, PageManifestEntry> {
  const map = new Map<string, PageManifestEntry>();
  const filePath = join(distDir, "pages.json");
  if (!existsSync(filePath)) {
    return map;
  }
  try {
    const raw = JSON.parse(readFileSync(filePath, "utf8")) as unknown;
    const entries: unknown[] = Array.isArray(raw)
      ? raw
      : raw && typeof raw === "object" && Array.isArray((raw as { pages?: unknown[] }).pages)
        ? ((raw as { pages: unknown[] }).pages ?? [])
        : [];
    for (const item of entries) {
      if (!item || typeof item !== "object") {
        continue;
      }
      const entry = item as PageManifestEntry;
      if (typeof entry.path === "string") {
        map.set(normalizeRoute(entry.path), entry);
      }
    }
  } catch {
    return map;
  }
  return map;
}

function parseSitemapXml(xml: string): Map<string, string> {
  const map = new Map<string, string>();
  const blockRe = /<url>([\s\S]*?)<\/url>/gi;
  for (const match of xml.matchAll(blockRe)) {
    const block = match[1] ?? "";
    const locMatch = /<loc>([^<]+)<\/loc>/i.exec(block);
    const lastmodMatch = /<lastmod>([^<]+)<\/lastmod>/i.exec(block);
    if (locMatch) {
      map.set(normalizeRoute(locMatch[1] ?? ""), (lastmodMatch?.[1] ?? "").trim());
    }
  }
  return map;
}

function readLocalSitemap(distDir: string): Map<string, string> {
  const filePath = join(distDir, "sitemap.xml");
  if (!existsSync(filePath)) {
    return new Map();
  }
  return parseSitemapXml(readFileSync(filePath, "utf8"));
}

function readAllowlist(filePath: string | undefined): AllowlistConfig {
  if (!filePath || !existsSync(filePath)) {
    return {};
  }
  try {
    const parsed = JSON.parse(readFileSync(filePath, "utf8")) as Record<string, unknown>;
    const out: AllowlistConfig = {};
    for (const [key, value] of Object.entries(parsed)) {
      if (key.startsWith("$") || key.startsWith("_")) {
        continue;
      }
      if (value && typeof value === "object" && Array.isArray((value as AllowlistEntry).checks)) {
        out[normalizeRoute(key)] = value as AllowlistEntry;
      }
    }
    return out;
  } catch {
    return {};
  }
}

function readPaceConfig(filePath: string | undefined): PaceConfig {
  const fallback: PaceConfig = { mode: "start", maxPerRelease: 20, maxPerWeek: 5 };
  if (!filePath || !existsSync(filePath)) {
    return fallback;
  }
  try {
    const parsed = JSON.parse(readFileSync(filePath, "utf8")) as Partial<PaceConfig>;
    return {
      mode: typeof parsed.mode === "string" ? parsed.mode : fallback.mode,
      maxPerRelease: typeof parsed.maxPerRelease === "number" ? parsed.maxPerRelease : fallback.maxPerRelease,
      maxPerWeek: typeof parsed.maxPerWeek === "number" ? parsed.maxPerWeek : fallback.maxPerWeek,
    };
  } catch {
    return fallback;
  }
}

function readCanonicalConstants(repoRoot: string): CanonicalConstants {
  let freeConversations = 100;
  const pricesPath = join(repoRoot, "site", "src", "pricing", "prices.ts");
  if (existsSync(pricesPath)) {
    const src = readFileSync(pricesPath, "utf8");
    const match = /FREE_CONVERSATIONS\s*=\s*(\d+)/.exec(src);
    if (match) {
      freeConversations = Number(match[1]);
    }
  }

  let sdkCount = 12;
  const sdkDir = join(repoRoot, "sdk");
  if (existsSync(sdkDir) && statSync(sdkDir).isDirectory()) {
    const manifests = ["package.json", "go.mod", "composer.json", "pyproject.toml"];
    const found = readdirSync(sdkDir).filter((name) => {
      const full = join(sdkDir, name);
      if (!statSync(full).isDirectory() || name.startsWith(".")) {
        return false;
      }
      return manifests.some((manifest) => existsSync(join(full, manifest)));
    });
    if (found.length > 0) {
      sdkCount = found.length;
    }
  }

  let installUrl = "https://github.com/dimmihq/dimmi/releases/latest/download/install.sh";
  const readmePath = join(repoRoot, "README.md");
  if (existsSync(readmePath)) {
    const src = readFileSync(readmePath, "utf8");
    const match = /(https:\/\/[^\s"'`]+install\.sh)/.exec(src);
    if (match) {
      installUrl = match[1] ?? installUrl;
    }
  }

  return { freeConversations, sdkCount, installUrl };
}

function parsePages(distDir: string, manifest: Map<string, PageManifestEntry>): ParsedPage[] {
  const files = walkHtml(distDir).sort();
  const pages: ParsedPage[] = [];

  for (const file of files) {
    const route = fileToRoute(distDir, file);
    if (!route) {
      continue;
    }
    const html = readFileSync(file, "utf8");
    const mainHtml = extractMainHtml(html);
    const meta = manifest.get(route)?.meta;
    const type = inferPageType(route, meta);
    const visibleMain = extractVisibleText(mainHtml, false);
    const tokens = tokenizeForShingles(visibleMain);
    const shingles = buildShingles(tokens, 5);

    pages.push({
      route,
      file,
      type,
      html,
      mainHtml,
      title: extractTitle(html),
      description: extractDescription(html),
      h1s: extractH1s(mainHtml.length > 0 ? mainHtml : html),
      jsonLdRaw: extractJsonLd(html),
      images: extractImages(mainHtml),
      allInternalLinks: extractInternalLinks(html),
      mainInternalLinks: extractInternalLinks(mainHtml),
      meta,
      tokens,
      shingles,
      shingleSet: new Set(shingles),
    });
  }

  return pages;
}

class GateCollector {
  readonly results: CheckResult[] = [];
  private readonly allowlist: AllowlistConfig;

  constructor(allowlist: AllowlistConfig) {
    this.allowlist = allowlist;
  }

  record(check: string, status: CheckStatus, message: string, route?: string, details?: Record<string, unknown>): void {
    if (status === "FAIL" && route && !NEVER_ALLOWLIST.has(check)) {
      const entry = this.allowlist[route];
      if (entry?.checks.includes(check)) {
        this.results.push({
          check,
          status: "WARN",
          route,
          message: `[allowlisted until ${entry.reviewBy}: ${entry.reason}] ${message}`,
          details,
        });
        return;
      }
    }
    this.results.push({ check, status, route, message, details });
  }
}

function wordsOf(text: string): string[] {
  return (text.toLowerCase().match(/[a-z0-9]+/g) ?? []).filter(Boolean);
}

function answerAppearsInWindow(answerWords: string[], windowWords: string[]): boolean {
  if (answerWords.length === 0) {
    return false;
  }
  const joinedWindow = windowWords.join(" ");
  const joinedAnswer = answerWords.join(" ");
  if (joinedWindow.includes(joinedAnswer)) {
    return true;
  }
  const windowSet = new Set(windowWords);
  let matched = 0;
  for (const word of answerWords) {
    if (windowSet.has(word)) {
      matched += 1;
    }
  }
  return matched / answerWords.length >= 0.85;
}

function collectMetaSignoffIssues(meta: PageMetaRecord): string[] {
  const issues: string[] = [];
  if (meta.status !== "published") {
    issues.push(`status is "${meta.status}" (draft pages must not be built)`);
  }
  if (meta.reviewedBy !== ALLOWED_REVIEWER) {
    issues.push(`reviewedBy must be "${ALLOWED_REVIEWER}", got "${meta.reviewedBy}"`);
  }
  if (!isValidIsoDate(meta.published)) {
    issues.push(`invalid published date "${meta.published}"`);
  }
  if (!isValidIsoDate(meta.updated)) {
    issues.push(`invalid updated date "${meta.updated}"`);
  }
  if (!isValidIsoDate(meta.reviewedOn)) {
    issues.push(`invalid reviewedOn date "${meta.reviewedOn}"`);
  }
  if (isValidIsoDate(meta.published) && isValidIsoDate(meta.updated) && meta.updated < meta.published) {
    issues.push(`updated (${meta.updated}) is before published (${meta.published})`);
  }
  if (!meta.author?.trim()) {
    issues.push("missing author");
  }
  if (!meta.keyword?.trim()) {
    issues.push("missing keyword");
  }
  return issues;
}

function checkSignoffAndAnswer(page: ParsedPage, collector: GateCollector): void {
  const needsMeta = Boolean(page.meta) || CONTENT_PAGE_TYPES.has(page.type);
  if (!needsMeta) {
    return;
  }
  const meta = page.meta;
  if (!meta) {
    collector.record("signoff", "FAIL", `missing PageMetaRecord in pages.json for ${page.type} page`, page.route);
    return;
  }

  const issues = collectMetaSignoffIssues(meta);
  if (issues.length > 0) {
    collector.record("signoff", "FAIL", issues.join("; "), page.route);
  } else {
    collector.record("signoff", "PASS", `signed off by ${meta.reviewedBy} on ${meta.reviewedOn}`, page.route);
  }

  const answerWords = wordsOf(meta.answer ?? "");
  const first50Main = wordsOf(extractVisibleText(page.mainHtml, false)).slice(0, 50);
  const afterH1Html = page.mainHtml.replace(/^[\s\S]*?<\/h1>/i, "");
  const first50AfterH1 = wordsOf(extractVisibleText(afterH1Html, false)).slice(0, 50);

  if (answerAppearsInWindow(answerWords, first50Main) || answerAppearsInWindow(answerWords, first50AfterH1)) {
    collector.record("answer", "PASS", "direct answer appears in the first 50 words", page.route);
  } else {
    collector.record("answer", "FAIL", "meta.answer is missing from the first 50 words of <main>", page.route);
  }
}

function findClosestPage(shingleSet: Set<string>, pool: ParsedPage[]): string | null {
  let closestPage: string | null = null;
  let maxOverlap = 0;
  for (const other of pool) {
    let overlap = 0;
    for (const shingle of shingleSet) {
      if (other.shingleSet.has(shingle)) {
        overlap += 1;
      }
    }
    if (overlap > maxOverlap) {
      maxOverlap = overlap;
      closestPage = other.route;
    }
  }
  return closestPage;
}

function markSharedShingles(shingles: string[], poolUnion: Set<string>, tokenCount: number) {
  const sharedWordMask = new Array<boolean>(tokenCount).fill(false);
  let sharedShingleCount = 0;
  for (let i = 0; i < shingles.length; i += 1) {
    const shingle = shingles[i];
    if (!shingle || !poolUnion.has(shingle)) {
      continue;
    }
    sharedShingleCount += 1;
    for (let k = i; k < i + 5 && k < sharedWordMask.length; k += 1) {
      sharedWordMask[k] = true;
    }
  }
  return { sharedShingleCount, templateWords: sharedWordMask.filter(Boolean).length };
}

function computePageUniqueness(page: ParsedPage, allPages: ParsedPage[]): UniquenessMetrics {
  const pool = allPages.filter(
    (other) =>
      other.route !== page.route && (other.type === page.type || other.route === "/" || other.route === "/pricing"),
  );
  const poolUnion = new Set<string>();
  for (const other of pool) {
    for (const shingle of other.shingleSet) {
      poolUnion.add(shingle);
    }
  }

  const { sharedShingleCount, templateWords } = markSharedShingles(page.shingles, poolUnion, page.tokens.length);
  const closestPage = findClosestPage(page.shingleSet, pool);
  const totalWords = page.tokens.length;
  const uniqueWords = Math.max(0, totalWords - templateWords);
  const score =
    page.shingles.length === 0
      ? 100
      : Math.round(((page.shingles.length - sharedShingleCount) / page.shingles.length) * 1000) / 10;

  return {
    route: page.route,
    type: page.type,
    score,
    uniqueWords,
    templateWords,
    totalWords,
    closestPage,
  };
}

function evaluatePageUniqueness(page: ParsedPage, m: UniquenessMetrics, collector: GateCollector): void {
  const summary = `score=${m.score.toFixed(1)}% uniqueWords=${m.uniqueWords} templateWords=${m.templateWords} closest=${m.closestPage ?? "none"}`;

  if (page.type === "api-reference") {
    const status = m.score < 65 ? "WARN" : "PASS";
    const msg = m.score < 65 ? `generated API reference below 65% (${summary})` : summary;
    collector.record("uniqueness", status, msg, page.route, m);
    return;
  }

  const failures: string[] = [];
  if (m.score < 65) {
    failures.push(`score ${m.score.toFixed(1)}% < 65%`);
  }
  if (page.type === "vs" && m.uniqueWords < 900) {
    failures.push(`uniqueWords ${m.uniqueWords} < 900`);
  }
  if ((page.type === "vs" || page.type === "marketing") && m.templateWords > 300) {
    failures.push(`templateWords ${m.templateWords} > 300`);
  }

  if (failures.length > 0) {
    collector.record("uniqueness", "FAIL", `${failures.join(", ")} (${summary})`, page.route, m);
  } else {
    collector.record("uniqueness", "PASS", summary, page.route, m);
  }
}

function checkUniqueness(pages: ParsedPage[], collector: GateCollector): UniquenessMetrics[] {
  const metricsList: UniquenessMetrics[] = [];
  for (const page of pages) {
    const m = computePageUniqueness(page, pages);
    metricsList.push(m);
    evaluatePageUniqueness(page, m, collector);
  }
  return metricsList;
}

function pushGrouped(map: Map<string, string[]>, key: string, route: string): void {
  const list = map.get(key) ?? [];
  list.push(route);
  map.set(key, list);
}

function checkSingleTitleAndDesc(
  page: ParsedPage,
  titles: Map<string, string[]>,
  descriptions: Map<string, string[]>,
  collector: GateCollector,
): void {
  if (!page.title) {
    collector.record("title", "FAIL", "missing <title>", page.route);
  } else if (page.title.length > 60) {
    collector.record("title", "FAIL", `title is ${page.title.length} chars (max 60): "${page.title}"`, page.route);
  } else {
    collector.record("title", "PASS", `title length ${page.title.length} chars`, page.route);
    pushGrouped(titles, page.title, page.route);
  }

  if (!page.description) {
    collector.record("description", "FAIL", 'missing <meta name="description">', page.route);
    return;
  }
  pushGrouped(descriptions, page.description, page.route);

  const isDocs = page.type === "docs" || page.type === "api-reference";
  if (page.description.length > 160 || (!isDocs && page.description.length < 140)) {
    collector.record(
      "description-length",
      "FAIL",
      `description is ${page.description.length} chars (expected ${isDocs ? "1 to 160" : "140 to 160"})`,
      page.route,
    );
  } else {
    collector.record("description-length", "PASS", `description length ${page.description.length} chars`, page.route);
  }
}

function checkTitlesAndDescriptions(pages: ParsedPage[], collector: GateCollector): void {
  const titles = new Map<string, string[]>();
  const descriptions = new Map<string, string[]>();

  for (const page of pages) {
    checkSingleTitleAndDesc(page, titles, descriptions, collector);
  }

  for (const [title, routesList] of titles) {
    if (routesList.length <= 1) {
      continue;
    }
    for (const route of routesList) {
      collector.record("duplicate-title", "FAIL", `shared <title> "${title}" with ${routesList.join(", ")}`, route);
    }
  }

  for (const [desc, routesList] of descriptions) {
    if (routesList.length <= 1) {
      continue;
    }
    for (const route of routesList) {
      collector.record(
        "duplicate-description",
        "FAIL",
        `shared <meta name="description"> ("${desc.slice(0, 60)}...") with ${routesList.join(", ")}`,
        route,
      );
    }
  }
}

function collectSchemaTypes(node: unknown, out: string[]): void {
  if (Array.isArray(node)) {
    for (const item of node) {
      collectSchemaTypes(item, out);
    }
    return;
  }
  if (!node || typeof node !== "object") {
    return;
  }
  const obj = node as Record<string, unknown>;
  const rawType = obj["@type"];
  if (typeof rawType === "string") {
    out.push(rawType);
  } else if (Array.isArray(rawType)) {
    for (const t of rawType) {
      if (typeof t === "string") {
        out.push(t);
      }
    }
  }
  for (const value of Object.values(obj)) {
    collectSchemaTypes(value, out);
  }
}

function topLevelObjects(parsed: unknown): Record<string, unknown>[] {
  if (Array.isArray(parsed)) {
    return parsed.filter((item): item is Record<string, unknown> => Boolean(item && typeof item === "object"));
  }
  if (!parsed || typeof parsed !== "object") {
    return [];
  }
  const obj = parsed as Record<string, unknown>;
  if (Array.isArray(obj["@graph"])) {
    return obj["@graph"].filter((item): item is Record<string, unknown> => Boolean(item && typeof item === "object"));
  }
  return [obj];
}

function isValidPersonAuthor(author: unknown): boolean {
  if (Array.isArray(author)) {
    return author.length > 0 && author.every(isValidPersonAuthor);
  }
  if (!author || typeof author !== "object") {
    return false;
  }
  const obj = author as Record<string, unknown>;
  return obj["@type"] === "Person" && typeof obj.name === "string" && obj.name.trim().length > 0;
}

function htmlContainsDate(html: string, isoDate: string): boolean {
  if (html.includes(isoDate)) {
    return true;
  }
  const match = ISO_DATE_RE.exec(isoDate);
  if (!match) {
    return false;
  }
  const year = Number(match[1]);
  const monthIdx = Number(match[2]) - 1;
  const day = Number(match[3]);
  const shortMonth = MONTHS_SHORT[monthIdx];
  const longMonth = MONTHS_LONG[monthIdx];
  if (!shortMonth || !longMonth) {
    return false;
  }
  const patterns = [
    `${shortMonth} ${day}, ${year}`,
    `${longMonth} ${day}, ${year}`,
    `${day} ${shortMonth} ${year}`,
    `${day} ${longMonth} ${year}`,
  ];
  return patterns.some((pattern) => html.includes(pattern));
}

function extractObjectTypes(obj: Record<string, unknown>): string[] {
  const t = obj["@type"];
  if (typeof t === "string") {
    return [t];
  }
  if (Array.isArray(t)) {
    return t.filter((v): v is string => typeof v === "string");
  }
  return [];
}

function inspectSchemaObject(
  obj: Record<string, unknown>,
  disallowedTypes: Set<string>,
  authorIssues: string[],
): boolean {
  let hasFaq = false;
  for (const name of extractObjectTypes(obj)) {
    if (name === "FAQPage") {
      hasFaq = true;
    } else if (!ALLOWED_SCHEMA_TYPES.has(name)) {
      disallowedTypes.add(name);
    }
  }
  if ("author" in obj && !isValidPersonAuthor(obj.author)) {
    authorIssues.push(`JSON-LD ${String(obj["@type"])} author must be @type: "Person" with a name`);
  }
  return hasFaq;
}

function parseJsonLdBlocks(page: ParsedPage, collector: GateCollector) {
  const parsedTopLevel: Record<string, unknown>[] = [];
  let hasFaq = false;
  const disallowedTypes = new Set<string>();
  const authorIssues: string[] = [];

  for (const raw of page.jsonLdRaw) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(raw);
    } catch {
      collector.record("schema-json", "FAIL", 'invalid JSON inside <script type="application/ld+json">', page.route);
      continue;
    }

    const nestedTypes: string[] = [];
    collectSchemaTypes(parsed, nestedTypes);
    if (nestedTypes.includes("FAQPage")) {
      hasFaq = true;
    }

    for (const obj of topLevelObjects(parsed)) {
      parsedTopLevel.push(obj);
      if (inspectSchemaObject(obj, disallowedTypes, authorIssues)) {
        hasFaq = true;
      }
    }
  }

  return { parsedTopLevel, hasFaq, disallowedTypes, authorIssues };
}

function checkMetaDates(
  page: ParsedPage,
  meta: PageMetaRecord,
  parsedTopLevel: Record<string, unknown>[],
  sitemapDates: Map<string, string>,
  collector: GateCollector,
): void {
  const dateObj = parsedTopLevel.find((obj) => "datePublished" in obj || "dateModified" in obj);
  const dateIssues: string[] = [];
  if (!dateObj) {
    dateIssues.push("JSON-LD missing datePublished and dateModified");
  } else {
    if (dateObj.datePublished !== meta.published) {
      dateIssues.push(`JSON-LD datePublished (${String(dateObj.datePublished)}) != meta.published (${meta.published})`);
    }
    if (dateObj.dateModified !== meta.updated) {
      dateIssues.push(`JSON-LD dateModified (${String(dateObj.dateModified)}) != meta.updated (${meta.updated})`);
    }
  }

  if (!htmlContainsDate(page.mainHtml, meta.updated)) {
    dateIssues.push(`<main> HTML does not show meta.updated (${meta.updated})`);
  }

  if (sitemapDates.size > 0) {
    const sitemapDate = sitemapDates.get(page.route);
    if (!sitemapDate) {
      dateIssues.push("route missing from sitemap.xml");
    } else if (sitemapDate !== meta.updated) {
      dateIssues.push(`sitemap.xml <lastmod> (${sitemapDate}) != meta.updated (${meta.updated})`);
    }
  }

  if (dateIssues.length > 0) {
    collector.record("dates", "FAIL", dateIssues.join("; "), page.route);
  } else {
    collector.record("dates", "PASS", `HTML, JSON-LD, and sitemap match ${meta.updated}`, page.route);
  }
}

function checkSchemaAndDates(page: ParsedPage, sitemapDates: Map<string, string>, collector: GateCollector): void {
  const { parsedTopLevel, hasFaq, disallowedTypes, authorIssues } = parseJsonLdBlocks(page, collector);

  if (hasFaq) {
    collector.record("faq-schema", "FAIL", "FAQPage JSON-LD schema is forbidden", page.route);
  }
  if (disallowedTypes.size > 0) {
    collector.record("schema", "FAIL", `disallowed JSON-LD @type: ${[...disallowedTypes].join(", ")}`, page.route);
  } else if (!hasFaq) {
    collector.record("schema", "PASS", `${parsedTopLevel.length} valid JSON-LD block(s)`, page.route);
  }

  const meta = page.meta;
  if (!meta) {
    if (authorIssues.length > 0) {
      collector.record("author", "FAIL", authorIssues.join("; "), page.route);
    }
    return;
  }

  checkMetaDates(page, meta, parsedTopLevel, sitemapDates, collector);

  const hasPersonAuthor = parsedTopLevel.some((obj) => "author" in obj && isValidPersonAuthor(obj.author));
  const requiresPersonAuthor = CONTENT_PAGE_TYPES.has(page.type) && page.type !== "self-host" && page.type !== "tool";
  if (authorIssues.length > 0) {
    collector.record("author", "FAIL", authorIssues.join("; "), page.route);
  } else if (!hasPersonAuthor && requiresPersonAuthor) {
    collector.record("author", "FAIL", 'JSON-LD missing author with @type: "Person"', page.route);
  } else {
    collector.record("author", "PASS", "JSON-LD author check passed", page.route);
  }
}

function checkH1(page: ParsedPage, collector: GateCollector): void {
  if (page.h1s.length !== 1) {
    collector.record("h1", "FAIL", `expected 1 <h1>, found ${page.h1s.length}`, page.route);
    return;
  }
  const heading = page.h1s[0] ?? "";
  if (page.type !== "docs" && page.type !== "api-reference") {
    const ciMatch = RIVAL_H1_CI_RE.exec(heading);
    const frontMatch = RIVAL_H1_FRONT_RE.exec(heading);
    const matchedRival = ciMatch?.[0] ?? frontMatch?.[0];
    if (matchedRival) {
      collector.record(
        "h1-rival",
        "FAIL",
        `<h1> contains rival name "${matchedRival}" (WRITING.md rule 4): "${heading}"`,
        page.route,
      );
      return;
    }
  }
  collector.record("h1", "PASS", `1 clean <h1>: "${heading}"`, page.route);
}

function proofFileExists(repoRoot: string, distDir: string, rel: string): boolean {
  const clean = rel.trim().replace(/^\/+/, "");
  if (!clean) {
    return false;
  }
  const candidates = [
    resolve(repoRoot, "site", "public", clean),
    resolve(repoRoot, "site", clean),
    resolve(distDir, clean),
  ];
  return candidates.some((candidate) => existsSync(candidate) && statSync(candidate).isFile());
}

function isPositiveIntString(value: string | null): boolean {
  if (!value) {
    return false;
  }
  return /^[1-9]\d*$/.test(value.trim());
}

function checkPageProof(page: ParsedPage, repoRoot: string, distDir: string, collector: GateCollector): void {
  const needsProof = page.type === "vs" || page.type === "alternatives" || page.type === "blog" || page.type === "tool";
  const proofList = page.meta?.proof ?? [];
  if (needsProof && proofList.length === 0) {
    collector.record("proof", "FAIL", `${page.type} page has no meta.proof files`, page.route);
    return;
  }
  if (proofList.length === 0) {
    return;
  }
  const missing = proofList.filter((item) => !proofFileExists(repoRoot, distDir, item));
  if (missing.length > 0) {
    collector.record("proof", "FAIL", `missing proof file(s): ${missing.join(", ")}`, page.route);
  } else {
    collector.record("proof", "PASS", `${proofList.length} proof file(s) verified`, page.route);
  }
}

function findMissingImgAttrs(img: ParsedPage["images"][number]): string[] {
  const missingAttrs: string[] = [];
  if (!img.ariaHidden && !img.alt?.trim()) {
    missingAttrs.push("alt");
  }
  if (!isPositiveIntString(img.width)) {
    missingAttrs.push("width");
  }
  if (!isPositiveIntString(img.height)) {
    missingAttrs.push("height");
  }
  return missingAttrs;
}

function checkProofAndImages(page: ParsedPage, repoRoot: string, distDir: string, collector: GateCollector): void {
  checkPageProof(page, repoRoot, distDir, collector);

  const badImages: string[] = [];
  for (const img of page.images) {
    const missingAttrs = findMissingImgAttrs(img);
    if (missingAttrs.length > 0) {
      badImages.push(`${img.src || "(inline)"} missing ${missingAttrs.join(",")}`);
    }
  }

  if (badImages.length > 0) {
    collector.record(
      "images",
      "FAIL",
      `${badImages.length} <img> tag(s) missing attributes (${badImages.slice(0, 3).join("; ")})`,
      page.route,
    );
  } else {
    collector.record("images", "PASS", `${page.images.length} <img> tag(s) checked`, page.route);
  }
}

function internalTargetExists(target: string, builtRoutes: Set<string>, distDir: string, docsBuilt: boolean): boolean {
  if (isAppRoute(target)) {
    return true;
  }
  if (!docsBuilt && (target === "/docs" || target.startsWith("/docs/"))) {
    return true;
  }
  if (builtRoutes.has(target)) {
    return true;
  }
  const rel = target.replace(/^\/+/, "");
  if (!rel) {
    return existsSync(join(distDir, "index.html"));
  }
  const direct = join(distDir, rel);
  if (existsSync(direct) && statSync(direct).isFile()) {
    return true;
  }
  return existsSync(join(distDir, rel, "index.html"));
}

function checkPageOutboundLinks(
  page: ParsedPage,
  builtRoutes: Set<string>,
  distDir: string,
  docsBuilt: boolean,
  inboundCount: Map<string, Set<string>>,
  collector: GateCollector,
): void {
  const broken = page.allInternalLinks.filter(
    (target) => !internalTargetExists(target, builtRoutes, distDir, docsBuilt),
  );
  if (broken.length > 0) {
    collector.record("links-broken", "FAIL", `broken internal link(s): ${broken.join(", ")}`, page.route);
  } else {
    collector.record("links-broken", "PASS", `${page.allInternalLinks.length} internal link(s) valid`, page.route);
  }

  const outMain = page.mainInternalLinks.filter((target) => target !== page.route && !isAppRoute(target));
  for (const target of outMain) {
    const callers = inboundCount.get(target) ?? new Set<string>();
    callers.add(page.route);
    inboundCount.set(target, callers);
  }

  if (page.type === "docs" || page.type === "api-reference") {
    return;
  }
  if (outMain.length < 3) {
    collector.record(
      "links-out",
      "FAIL",
      `only ${outMain.length} internal link(s) out of <main> (need >= 3)`,
      page.route,
    );
  } else {
    collector.record("links-out", "PASS", `${outMain.length} internal links out of <main>`, page.route);
  }
}

function checkLinks(pages: ParsedPage[], distDir: string, collector: GateCollector): void {
  const builtRoutes = new Set(pages.map((p) => p.route));
  const docsBuilt = existsSync(join(distDir, "docs"));
  const inboundCount = new Map<string, Set<string>>();

  for (const page of pages) {
    checkPageOutboundLinks(page, builtRoutes, distDir, docsBuilt, inboundCount, collector);
  }

  for (const page of pages) {
    const needsInbound = page.route === "/pricing" || (page.route.startsWith("/vs/") && page.route !== "/vs");
    if (!needsInbound) {
      continue;
    }
    const inbound = inboundCount.get(page.route)?.size ?? 0;
    if (inbound < 3) {
      collector.record(
        "links-in",
        "FAIL",
        `only ${inbound} contextual inbound link(s) from <main> (need >= 3)`,
        page.route,
      );
    } else {
      collector.record("links-in", "PASS", `${inbound} contextual inbound links`, page.route);
    }
  }
}

function evaluateSingleRival(
  rival: NonNullable<PageMetaRecord["rivals"]>[number],
  updated: string,
  today: string,
): { status: "PASS" | "WARN" | "FAIL"; note?: string } {
  if (!isValidIsoDate(rival.checkedOn) || !isHttpUrl(rival.source)) {
    return { status: "FAIL", note: `${rival.name}: invalid checkedOn or source` };
  }
  if (isValidIsoDate(updated)) {
    const lag = daysBetween(updated, rival.checkedOn);
    if (lag > 45) {
      return {
        status: "FAIL",
        note: `${rival.name}: checkedOn ${rival.checkedOn} is ${lag}d older than updated ${updated} (max 45d)`,
      };
    }
  }
  const age = daysBetween(today, rival.checkedOn);
  if (age > 90) {
    return { status: "WARN", note: `${rival.name}: checkedOn ${rival.checkedOn} is ${age}d old (> 90d)` };
  }
  return { status: "PASS" };
}

function checkRivals(page: ParsedPage, today: string, collector: GateCollector): void {
  const rivals = page.meta?.rivals ?? [];
  if (page.type === "vs" && rivals.length === 0) {
    collector.record("rivals", "FAIL", "/vs page must list checked rival facts in meta.rivals", page.route);
    return;
  }
  if (rivals.length === 0) {
    return;
  }

  const updated = page.meta?.updated ?? today;
  let failed = false;
  let warned = false;
  const notes: string[] = [];

  for (const rival of rivals) {
    const res = evaluateSingleRival(rival, updated, today);
    if (res.status === "FAIL") {
      failed = true;
    } else if (res.status === "WARN") {
      warned = true;
    }
    if (res.note) {
      notes.push(res.note);
    }
  }

  if (failed) {
    collector.record("rivals", "FAIL", notes.join("; "), page.route);
  } else if (warned) {
    collector.record("rivals", "WARN", notes.join("; "), page.route);
  } else {
    collector.record("rivals", "PASS", `${rivals.length} rival check date(s) fresh`, page.route);
  }
}

function checkSlop(page: ParsedPage, collector: GateCollector): void {
  if (page.type === "api-reference") {
    return;
  }
  let prose = extractVisibleText(page.mainHtml, true);
  if (page.type === "docs") {
    // Astro's Markdown smartypants turns straight ASCII quotes into typographic quotes in HTML.
    prose = prose.replace(/[\u201c\u201d]/g, '"').replace(/[\u2018\u2019]/g, "'");
  }
  const hits: string[] = [];
  for (const match of prose.matchAll(SLOP_RE)) {
    const token = match[0];
    if (token) {
      hits.push(token);
    }
  }
  if (hits.length > 0) {
    const preview = [...new Set(hits)].slice(0, 6).join(", ");
    collector.record("slop", "FAIL", `${hits.length} slop hit(s): ${preview}`, page.route);
  } else {
    collector.record("slop", "PASS", "0 slop hits", page.route);
  }
}

function checkConstants(page: ParsedPage, constants: CanonicalConstants, collector: GateCollector): void {
  if (page.type === "docs" || page.type === "api-reference") {
    return;
  }
  const text = extractVisibleText(page.mainHtml, false);
  const issues: string[] = [];

  for (const match of text.matchAll(/\b(\d+)\s+free\s+conversations\b/gi)) {
    const found = Number(match[1]);
    if (found !== constants.freeConversations) {
      issues.push(`found "${match[0]}" (canonical FREE_CONVERSATIONS is ${constants.freeConversations})`);
    }
  }

  for (const match of text.matchAll(/\b(\d+)\s+(?:typed\s+)?SDKs\b/g)) {
    const found = Number(match[1]);
    if (found !== constants.sdkCount) {
      issues.push(`found "${match[0]}" (canonical SDK count in sdk/ is ${constants.sdkCount})`);
    }
  }

  for (const match of page.mainHtml.matchAll(/https?:\/\/[^\s"'<>]+install\.sh/gi)) {
    const foundUrl = match[0];
    if (foundUrl !== constants.installUrl) {
      issues.push(`install URL "${foundUrl}" does not match README (${constants.installUrl})`);
    }
  }

  if (issues.length > 0) {
    collector.record("constants", "FAIL", issues.join("; "), page.route);
  } else {
    collector.record("constants", "PASS", "canonical constants match", page.route);
  }
}

async function loadBaselineSitemap(baseline: string): Promise<Map<string, string> | null> {
  if (baseline.startsWith("http://") || baseline.startsWith("https://")) {
    try {
      const response = await fetch(baseline, { signal: AbortSignal.timeout(5000) });
      if (!response.ok) {
        return null;
      }
      return parseSitemapXml(await response.text());
    } catch {
      return null;
    }
  }
  if (!existsSync(baseline)) {
    return null;
  }
  return parseSitemapXml(readFileSync(baseline, "utf8"));
}

async function checkPace(
  pages: ParsedPage[],
  currentSitemap: Map<string, string>,
  pace: PaceConfig,
  baseline: string | undefined,
  today: string,
  collector: GateCollector,
): Promise<void> {
  if (!baseline) {
    collector.record("pace", "PASS", "no --baseline provided; pace check skipped");
    return;
  }
  const baselineMap = await loadBaselineSitemap(baseline);
  if (!baselineMap) {
    collector.record("pace", "WARN", `baseline "${baseline}" unreachable; skipping rollout pace check`);
    return;
  }

  const currentRoutes = currentSitemap.size > 0 ? [...currentSitemap.keys()] : pages.map((p) => p.route);
  const newRoutes = currentRoutes.filter((route) => !baselineMap.has(route));

  if (newRoutes.length > pace.maxPerRelease) {
    collector.record(
      "pace",
      "FAIL",
      `${newRoutes.length} new URL(s) exceed maxPerRelease (${pace.maxPerRelease})`,
      undefined,
      { newRoutes },
    );
    return;
  }

  if (pace.mode === "start") {
    const pageByRoute = new Map(pages.map((p) => [p.route, p]));
    const recentNew = newRoutes.filter((route) => {
      const published = pageByRoute.get(route)?.meta?.published;
      if (!published || !isValidIsoDate(published)) {
        return true;
      }
      const age = daysBetween(today, published);
      return age >= 0 && age < 7;
    });
    if (recentNew.length > pace.maxPerWeek) {
      collector.record(
        "pace",
        "FAIL",
        `${recentNew.length} new URL(s) this week exceed maxPerWeek (${pace.maxPerWeek}) while pace.json mode is "start"`,
        undefined,
        { recentNew },
      );
      return;
    }
  }

  collector.record(
    "pace",
    "PASS",
    `${newRoutes.length} new URL(s) within pace limits (mode=${pace.mode}, maxPerRelease=${pace.maxPerRelease}, maxPerWeek=${pace.maxPerWeek})`,
  );
}

function printReport(report: PublishGateReport): void {
  for (const m of report.uniqueness) {
    process.stdout.write(
      `UNIQUE ${m.route} (${m.type}): score=${m.score.toFixed(1)}% uniqueWords=${m.uniqueWords} templateWords=${m.templateWords} closest=${m.closestPage ?? "none"}\n`,
    );
  }
  for (const result of report.results) {
    if (result.status === "PASS") {
      continue;
    }
    const target = result.route ? ` ${result.route}` : "";
    process.stdout.write(`${result.status} [${result.check}]${target}: ${result.message}\n`);
  }
  const { pagesChecked, pass, warn, fail } = report.summary;
  process.stdout.write(
    `publish-gate: checked ${pagesChecked} page(s) -> PASS=${pass} WARN=${warn} FAIL=${fail}${report.reportOnly ? " (report-only)" : ""}\n`,
  );
}

export async function runPublishGate(
  distDir = fileURLToPath(new URL("../site/dist", import.meta.url)),
  options: PublishGateOptions = {},
): Promise<PublishGateReport> {
  const repoRoot = options.repoRoot ?? fileURLToPath(new URL("..", import.meta.url));
  const today = options.today ?? new Date().toISOString().slice(0, 10);
  const reportOnly = Boolean(options.reportOnly);
  const allowlistPath = options.allowlistPath ?? join(repoRoot, "site", "publish-gate-allowlist.json");
  const pacePath = options.pacePath ?? join(repoRoot, "site", "pace.json");

  const manifest = readPagesManifest(distDir);
  const sitemapDates = readLocalSitemap(distDir);
  const allowlist = readAllowlist(allowlistPath);
  const pace = readPaceConfig(pacePath);
  const constants = readCanonicalConstants(repoRoot);
  const pages = parsePages(distDir, manifest);
  const collector = new GateCollector(allowlist);

  for (const page of pages) {
    checkSignoffAndAnswer(page, collector);
    checkSchemaAndDates(page, sitemapDates, collector);
    checkH1(page, collector);
    checkProofAndImages(page, repoRoot, distDir, collector);
    checkRivals(page, today, collector);
    checkSlop(page, collector);
    checkConstants(page, constants, collector);
  }

  const uniqueness = checkUniqueness(pages, collector);
  checkTitlesAndDescriptions(pages, collector);
  checkLinks(pages, distDir, collector);
  await checkPace(pages, sitemapDates, pace, options.baseline, today, collector);

  const pass = collector.results.filter((r) => r.status === "PASS").length;
  const warn = collector.results.filter((r) => r.status === "WARN").length;
  const fail = collector.results.filter((r) => r.status === "FAIL").length;

  const report: PublishGateReport = {
    generatedAt: new Date().toISOString(),
    distDir,
    reportOnly,
    summary: { pagesChecked: pages.length, pass, warn, fail },
    uniqueness,
    results: collector.results,
  };

  if (options.writeReport !== false) {
    writeFileSync(join(distDir, "publish-gate.json"), `${JSON.stringify(report, null, 2)}\n`, "utf8");
  }

  if (!options.silent) {
    printReport(report);
  }

  return report;
}

type CliArgs = {
  distDir: string;
  baseline?: string;
  reportOnly: boolean;
  allowlistPath?: string;
  pacePath?: string;
  today?: string;
};

const VALUE_FLAGS: Record<string, "baseline" | "allowlistPath" | "pacePath" | "today"> = {
  "--baseline": "baseline",
  "--allowlist": "allowlistPath",
  "--pace": "pacePath",
  "--today": "today",
};

function parseCliArgs(argv: string[]): CliArgs {
  const args: CliArgs = {
    distDir: fileURLToPath(new URL("../site/dist", import.meta.url)),
    reportOnly: false,
  };
  let positionalSet = false;
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i] ?? "";
    if (arg === "--report-only") {
      args.reportOnly = true;
      continue;
    }
    const key = VALUE_FLAGS[arg];
    if (key && argv[i + 1]) {
      args[key] = argv[i + 1];
      i += 1;
      continue;
    }
    if (!arg.startsWith("--") && !positionalSet) {
      args.distDir = resolve(arg);
      positionalSet = true;
    }
  }
  return args;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const cli = parseCliArgs(process.argv.slice(2));
  const report = await runPublishGate(cli.distDir, {
    baseline: cli.baseline,
    reportOnly: cli.reportOnly,
    allowlistPath: cli.allowlistPath,
    pacePath: cli.pacePath,
    today: cli.today,
  });
  if (report.summary.fail > 0 && !cli.reportOnly) {
    process.exit(1);
  }
}
