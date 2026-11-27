/*
 * Submits trydimmi.com URLs from sitemap.xml to IndexNow in batches of up to
 * 10,000 URLs. Reads local site/dist/sitemap.xml when present, or fetches
 * https://trydimmi.com/sitemap.xml in production releases.
 *
 * Usage:
 *   node scripts/indexnow.ts [--sitemap <path-or-url>] [--previous-sitemap <path-or-url>] [--since YYYY-MM-DD] [--dry-run]
 */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

export const DEFAULT_HOST = "trydimmi.com";
export const DEFAULT_ENDPOINT = "https://api.indexnow.org/indexnow";
export const MAX_BATCH_SIZE = 10_000;

const KEY_FILE_RE = /^([0-9a-f]{32})\.txt$/;
const DEFAULT_PUBLIC_DIR = fileURLToPath(new URL("../site/public", import.meta.url));
const DEFAULT_LOCAL_SITEMAP = fileURLToPath(new URL("../site/dist/sitemap.xml", import.meta.url));

export type SitemapEntry = {
  loc: string;
  lastmod: string | null;
};

export type IndexNowPayload = {
  host: string;
  key: string;
  keyLocation: string;
  urlList: string[];
};

export type IndexNowOptions = {
  sitemap?: string;
  previousSitemap?: string;
  since?: string;
  key?: string;
  publicDir?: string;
  host?: string;
  endpoint?: string;
  batchSize?: number;
  dryRun?: boolean;
  fetchImpl?: typeof fetch;
  log?: (message: string) => void;
};

export type IndexNowResult = {
  submitted: number;
  batches: number;
  statuses: number[];
  payloads: IndexNowPayload[];
};

const XML_ENTITIES: Record<string, string> = {
  "&amp;": "&",
  "&lt;": "<",
  "&gt;": ">",
  "&quot;": '"',
  "&apos;": "'",
};

function decodeXmlEntities(value: string): string {
  return value.replace(/&(?:amp|lt|gt|quot|apos);/g, (entity) => XML_ENTITIES[entity] ?? entity);
}

function isHttpUrl(source: string): boolean {
  return source.startsWith("http://") || source.startsWith("https://");
}

export function parseSitemapXml(xml: string): SitemapEntry[] {
  const entries: SitemapEntry[] = [];
  const seen = new Set<string>();
  const urlBlockRe = /<url\b[^>]*>([\s\S]*?)<\/url>/gi;

  for (const match of xml.matchAll(urlBlockRe)) {
    const block = match[1] ?? "";
    const locMatch = /<loc>\s*([^<]+?)\s*<\/loc>/i.exec(block);
    if (!locMatch) {
      continue;
    }
    const loc = decodeXmlEntities(locMatch[1].trim());
    if (!loc || seen.has(loc)) {
      continue;
    }
    seen.add(loc);
    const lastmodMatch = /<lastmod>\s*([^<]+?)\s*<\/lastmod>/i.exec(block);
    const lastmod = lastmodMatch ? decodeXmlEntities(lastmodMatch[1].trim()) : null;
    entries.push({ loc, lastmod });
  }

  return entries;
}

export function selectUrls(
  current: SitemapEntry[],
  options: { previous?: SitemapEntry[]; since?: string; host?: string } = {},
): string[] {
  const previousMap = options.previous ? new Map(options.previous.map((entry) => [entry.loc, entry.lastmod])) : null;
  const host = options.host;
  const since = options.since;
  const selected: string[] = [];

  for (const entry of current) {
    if (host && !matchesHost(entry.loc, host)) {
      continue;
    }
    if (previousMap?.has(entry.loc) && previousMap.get(entry.loc) === entry.lastmod) {
      continue;
    }
    if (since && (!entry.lastmod || entry.lastmod < since)) {
      continue;
    }
    selected.push(entry.loc);
  }

  return selected;
}

function matchesHost(urlString: string, expectedHost: string): boolean {
  try {
    return new URL(urlString).host === expectedHost;
  } catch {
    return false;
  }
}

export function chunkUrls(urls: string[], maxBatchSize = MAX_BATCH_SIZE): string[][] {
  if (maxBatchSize <= 0) {
    throw new Error(`indexnow: batchSize must be positive, got ${maxBatchSize}`);
  }
  const chunks: string[][] = [];
  for (let i = 0; i < urls.length; i += maxBatchSize) {
    chunks.push(urls.slice(i, i + maxBatchSize));
  }
  return chunks;
}

export function readIndexNowKey(publicDir = DEFAULT_PUBLIC_DIR): string {
  const matches = readdirSync(publicDir).filter((name) => KEY_FILE_RE.test(name));
  if (matches.length !== 1) {
    throw new Error(`indexnow: expected 1 32-hex key file in ${publicDir}, found ${matches.length}`);
  }
  const filename = matches[0];
  const expectedKey = filename.slice(0, -".txt".length);
  const content = readFileSync(join(publicDir, filename), "utf8").trim();
  if (content !== expectedKey) {
    throw new Error(`indexnow: ${filename} contents do not match its filename`);
  }
  return expectedKey;
}

export async function loadSitemapXml(source: string, fetchImpl: typeof fetch = fetch): Promise<string> {
  if (isHttpUrl(source)) {
    const response = await fetchImpl(source);
    if (!response.ok) {
      throw new Error(`indexnow: failed to fetch sitemap ${source}: HTTP ${response.status}`);
    }
    return response.text();
  }
  return readFileSync(source, "utf8");
}

function resolveSitemapSource(configured: string | undefined, host: string): string {
  if (configured) {
    return configured;
  }
  if (existsSync(DEFAULT_LOCAL_SITEMAP)) {
    return DEFAULT_LOCAL_SITEMAP;
  }
  return `https://${host}/sitemap.xml`;
}

async function postPayloads(
  payloads: IndexNowPayload[],
  endpoint: string,
  fetchImpl: typeof fetch,
  log: (message: string) => void,
): Promise<number[]> {
  const statuses: number[] = [];
  for (let i = 0; i < payloads.length; i += 1) {
    const payload = payloads[i];
    const response = await fetchImpl(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json; charset=utf-8" },
      body: JSON.stringify(payload),
    });
    statuses.push(response.status);
    log(`indexnow: batch ${i + 1}/${payloads.length} (${payload.urlList.length} URLs) -> HTTP ${response.status}`);
    if (response.status !== 200 && response.status !== 202) {
      const body = await response.text().catch(() => "");
      throw new Error(`indexnow: submission rejected with HTTP ${response.status}${body ? `: ${body}` : ""}`);
    }
  }
  return statuses;
}

export async function submitIndexNow(options: IndexNowOptions = {}): Promise<IndexNowResult> {
  const host = options.host ?? DEFAULT_HOST;
  const endpoint = options.endpoint ?? DEFAULT_ENDPOINT;
  const batchSize = options.batchSize ?? MAX_BATCH_SIZE;
  const fetchImpl = options.fetchImpl ?? fetch;
  const log = options.log ?? ((message: string) => process.stdout.write(`${message}\n`));
  const key = options.key ?? readIndexNowKey(options.publicDir);
  const keyLocation = `https://${host}/${key}.txt`;

  const sitemapSource = resolveSitemapSource(options.sitemap, host);
  const currentEntries = parseSitemapXml(await loadSitemapXml(sitemapSource, fetchImpl));
  const previousEntries = options.previousSitemap
    ? parseSitemapXml(await loadSitemapXml(options.previousSitemap, fetchImpl))
    : undefined;

  const urls = selectUrls(currentEntries, { previous: previousEntries, since: options.since, host });
  if (urls.length === 0) {
    log("indexnow: no changed URLs to submit");
    return { submitted: 0, batches: 0, statuses: [], payloads: [] };
  }

  const payloads: IndexNowPayload[] = chunkUrls(urls, batchSize).map((urlList) => ({
    host,
    key,
    keyLocation,
    urlList,
  }));

  if (options.dryRun) {
    log(`indexnow: dry run (${urls.length} URLs in ${payloads.length} batch(es))`);
    for (const payload of payloads) {
      log(JSON.stringify(payload, null, 2));
    }
    return { submitted: urls.length, batches: payloads.length, statuses: [], payloads };
  }

  const statuses = await postPayloads(payloads, endpoint, fetchImpl, log);
  return { submitted: urls.length, batches: payloads.length, statuses, payloads };
}

type StringOptionKey = "sitemap" | "previousSitemap" | "since" | "key" | "host" | "endpoint";

const VALUE_FLAGS: Record<string, StringOptionKey> = {
  "--sitemap": "sitemap",
  "--previous-sitemap": "previousSitemap",
  "--since": "since",
  "--key": "key",
  "--host": "host",
  "--endpoint": "endpoint",
};

function readFlagValue(argv: string[], index: number, flag: string): string {
  const value = argv[index + 1];
  if (!value || value.startsWith("--")) {
    throw new Error(`indexnow: missing value for ${flag}`);
  }
  return value;
}

export function parseCliArgs(argv: string[]): IndexNowOptions {
  const options: IndexNowOptions = {};
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--dry-run") {
      options.dryRun = true;
      continue;
    }
    const target = VALUE_FLAGS[arg];
    if (!target) {
      throw new Error(`indexnow: unknown argument ${arg}`);
    }
    options[target] = readFlagValue(argv, i, arg);
    i += 1;
  }
  return options;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  submitIndexNow(parseCliArgs(process.argv.slice(2))).catch((err: unknown) => {
    const message = err instanceof Error ? err.message : String(err);
    process.stderr.write(`${message}\n`);
    process.exitCode = 1;
  });
}
