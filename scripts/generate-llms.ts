/*
 * Builds site/dist/llms.txt and site/dist/llms-full.txt at the domain root
 * after `vite build` (marketing site) and `astro build` (docs).
 *
 * Everything is derived from the built outputs:
 * - Marketing & channel pages from site/dist/**\/index.html (title, description, canonical)
 * - SDK catalog & API metadata from site/dist/docs/sdks.json
 * - Full Markdown docs & REST API reference index from site/dist/docs/llms.txt
 * - Complete Markdown corpus from site/dist/docs/llms-full.txt
 */
import { existsSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const SITE_URL = "https://trydimmi.com";
const GITHUB_URL = "https://github.com/dimmihq/dimmi";
const LEGAL_PATHS = new Set(["/terms", "/privacy"]);

type MarketingPage = {
  path: string;
  url: string;
  title: string;
  description: string;
};

type SdksManifest = {
  api_version: string;
  openapi: string;
  mcp: {
    url: string;
    transport: string;
    oauth_metadata: string;
    server_card: string;
  };
  install_skill: {
    name: string;
    docs: string;
    skill_md: string;
    agents_snippet: string;
  };
  sdks: {
    id: string;
    package: string;
    kind: "messenger" | "backend";
    ecosystem: string;
    install: string;
    runtime: string;
    description: string;
    docs: string;
    source: string;
  }[];
};

const HTML_ENTITIES: Record<string, string> = {
  "&amp;": "&",
  "&lt;": "<",
  "&gt;": ">",
  "&quot;": '"',
  "&#39;": "'",
  "&apos;": "'",
  "&#x27;": "'",
};

function decodeEntities(text: string): string {
  return text.replace(
    /&(?:amp|lt|gt|quot|apos|#39|#x27);/gi,
    (entity) => HTML_ENTITIES[entity.toLowerCase()] ?? entity,
  );
}

function walkMarketingHtml(dir: string, root: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (dir === root && (name === "docs" || name === "assets" || name === "shots" || name === "logos")) {
        continue;
      }
      walkMarketingHtml(full, root, out);
    } else if (name === "index.html") {
      out.push(full);
    }
  }
  return out;
}

function readMarketingPages(distDir: string): MarketingPage[] {
  const files = walkMarketingHtml(distDir, distDir);
  const pages: MarketingPage[] = [];

  for (const file of files) {
    const rel = relative(distDir, file).replace(/\\/g, "/");
    const route = rel === "index.html" ? "/" : `/${rel.replace(/\/index\.html$/, "")}`;
    if (route === "/404") {
      continue;
    }
    const html = readFileSync(file, "utf8");
    const titleMatch = /<title>([\s\S]*?)<\/title>/i.exec(html);
    const descMatch =
      /<meta[^>]+name=["']description["'][^>]+content=["']([^"']*)["']/i.exec(html) ??
      /<meta[^>]+content=["']([^"']*)["'][^>]+name=["']description["']/i.exec(html);

    const rawTitle = decodeEntities(titleMatch?.[1]?.trim() ?? route);
    const title = rawTitle.replace(/\s*\|\s*dimmi$/i, "").trim();
    const description = decodeEntities(descMatch?.[1]?.trim() ?? "");
    const url = route === "/" ? `${SITE_URL}/` : `${SITE_URL}${route}`;

    pages.push({ path: route, url, title, description });
  }

  return pages.sort((a, b) => {
    if (a.path === "/") {
      return -1;
    }
    if (b.path === "/") {
      return 1;
    }
    return a.path.localeCompare(b.path);
  });
}

function extractDocsSections(docsLlmsTxt: string): { overviewLines: string[]; body: string } {
  const divider = "\n---\n";
  const firstDivider = docsLlmsTxt.indexOf(divider);
  if (firstDivider === -1) {
    return { overviewLines: [], body: docsLlmsTxt.trim() };
  }

  const head = docsLlmsTxt.slice(0, firstDivider);
  const overviewMatch = /## Documentation overview\n\n([\s\S]*)$/.exec(head);
  const overviewLines = overviewMatch
    ? overviewMatch[1]
        .trim()
        .split("\n")
        .filter((line) => line.startsWith("- "))
    : [];

  let body = docsLlmsTxt.slice(firstDivider + divider.length).trim();
  const lastDivider = body.lastIndexOf(divider);
  if (lastDivider !== -1) {
    body = body.slice(0, lastDivider).trim();
  }

  return { overviewLines, body };
}

function formatPageLine(page: MarketingPage): string {
  return `- [${page.title}](${page.url})${page.description ? ` - ${page.description}` : ""}`;
}

export function generateLlms(distDir = fileURLToPath(new URL("../site/dist", import.meta.url))): {
  llmsLines: number;
  fullBytes: number;
} {
  const docsLlmsPath = join(distDir, "docs", "llms.txt");
  const docsFullPath = join(distDir, "docs", "llms-full.txt");
  const sdksJsonPath = join(distDir, "docs", "sdks.json");

  if (!existsSync(docsLlmsPath) || !existsSync(docsFullPath) || !existsSync(sdksJsonPath)) {
    throw new Error(
      "Missing built docs outputs (docs/llms.txt, docs/llms-full.txt, docs/sdks.json). Run astro build first.",
    );
  }

  const docsLlmsTxt = readFileSync(docsLlmsPath, "utf8");
  const docsFullTxt = readFileSync(docsFullPath, "utf8");
  const sdks = JSON.parse(readFileSync(sdksJsonPath, "utf8")) as SdksManifest;
  const marketingPages = readMarketingPages(distDir);

  const productPages = marketingPages.filter((p) => !LEGAL_PATHS.has(p.path) && !p.path.startsWith("/channels"));
  const channelPages = marketingPages.filter((p) => p.path === "/channels" || p.path.startsWith("/channels/"));
  const legalPages = marketingPages.filter((p) => LEGAL_PATHS.has(p.path));

  const messengerSdks = sdks.sdks.filter((s) => s.kind === "messenger");
  const backendSdks = sdks.sdks.filter((s) => s.kind === "backend");
  const { overviewLines, body: docsBody } = extractDocsSections(docsLlmsTxt);

  const quickReference = [
    "## Quick reference for coding agents",
    "",
    "You need two values from the dimmi dashboard: the messenger app id (`wgt_...`, Settings > Messengers) and the dimmi host (`https://trydimmi.com` or your self-hosted URL). The loader script is `<host>/widget/v1/loader.js`.",
    "",
    "### Pick the messenger package",
    "",
    `- Plain HTML: paste \`<script async src="<host>/widget/v1/loader.js" data-app-id="wgt_..."></script>\` before \`</body>\`, or use \`@dimmi/browser\`.`,
    ...messengerSdks.map(
      (sdk) => `- \`${sdk.package}\` (${sdk.runtime}): \`${sdk.install}\` - ${sdk.description} ([docs](${sdk.docs}))`,
    ),
    "",
    "### Pick the backend SDK",
    "",
    ...backendSdks.map(
      (sdk) => `- \`${sdk.package}\` (${sdk.runtime}): \`${sdk.install}\` - ${sdk.description} ([docs](${sdk.docs}))`,
    ),
    "",
    "### Identity verification",
    "",
    "Never trust a user id from the browser. Sign a short-lived JWT on your server and pass only the token to the browser:",
    "- Server: `@dimmi/server` (`signIdentityToken`, `createIdentityHandler`, `@dimmi/server/express`, `@dimmi/server/hono`), `@dimmi/nextjs/server`, or the Go, PHP, Laravel, or Python SDKs.",
    "- Env vars: `DIMMI_IDENTITY_SECRET` (optional `DIMMI_IDENTITY_KID` for HS256; required with `DIMMI_IDENTITY_PRIVATE_KEY` for RS256/ES256/EdDSA) from Settings > Identity verification.",
    '- Browser: pass `identity: async () => tokenOrNull` or `identityUrl: "/api/dimmi/token"` returning `{"token": "<jwt>"}` or `{"token": null}`. Call `identify()` after sign-in without reload and `shutdown()` on sign-out.',
    "",
    "### Messenger JavaScript API",
    "",
    "`show()`, `hide()`, `toggle()`, `showNewMessage(text?)`, `update({ appearance: { position, primary_color, theme, launcher } })`, `identify(fn?)`, `shutdown()`, `onUnreadCountChange(fn)`, `trackEvent(name, metadata)` (for trusted rule events, record server-side via `POST /api/v1/contact-events`). Options: `appId`, `host`, `apiBase`, `nonce`, `identity`, `identityUrl`, `settings`.",
    "",
    "### REST API essentials",
    "",
    `- Base URL: \`<host>/api/v1\` (OpenAPI 3.1 contract: \`<host>/api/v1/openapi.json\`, version \`${sdks.api_version}\`).`,
    "- Headers: `Authorization: Bearer <access token>` and `Dimmi-Workspace: <workspace id>`.",
    "- Pagination & retries: `cursor` + `limit` (`meta.next_cursor`); `429` includes `Retry-After`; `POST` endpoints accept `Idempotency-Key`.",
  ];

  const llmsLines = [
    "# dimmi",
    "",
    "> dimmi is an open-source (MIT) customer support platform with a shared inbox, AI support agents, procedures, Operator, tickets, workflows, broadcasts, and a help center. Run on dimmi Cloud with unlimited free seats or self-host on your own Linux server.",
    "",
    "## Root URL",
    "",
    `dimmi Cloud, dashboard & marketing site: ${SITE_URL}`,
    `Documentation: ${SITE_URL}/docs/`,
    `REST API (${sdks.api_version}): ${SITE_URL}/api/v1`,
    `Messenger API: ${SITE_URL}/widget/v1`,
    `Open-source repository (MIT): ${GITHUB_URL}`,
    "",
    "## MCP server",
    "",
    `Connect an MCP client to ${sdks.mcp.url} (Streamable HTTP transport, OAuth 2.0 or Bearer API token) to search knowledge, manage conversations, contacts, tickets, help articles, and procedures, or propose reviewable Operator edits.`,
    `OAuth discovery metadata: ${sdks.mcp.oauth_metadata}`,
    `Server card: ${sdks.mcp.server_card}`,
    `MCP guide: ${SITE_URL}/docs/developers/mcp.md`,
    "",
    "## OpenAPI, SDKs & coding agent skill",
    "",
    `- OpenAPI 3.1 contract (${sdks.api_version}): ${sdks.openapi}`,
    `- SDK catalog (${sdks.sdks.length} browser & backend packages, JSON): ${SITE_URL}/docs/sdks.json`,
    `- Install skill (${sdks.install_skill.name}) for Claude Code, Cursor & Codex: ${sdks.install_skill.docs} (raw SKILL.md: ${sdks.install_skill.skill_md})`,
    "",
    "## Content overview",
    "",
    `- Product, channels & pricing (${productPages.length + channelPages.length} pages) - ${SITE_URL}/`,
    ...overviewLines,
    "",
    ...quickReference,
    "",
    "---",
    "",
    "## Product & pricing",
    "",
    ...productPages.map(formatPageLine),
    "",
    "## Channels",
    "",
    ...channelPages.map(formatPageLine),
    "",
    docsBody,
    "",
    "## Optional",
    "",
    ...legalPages.map(formatPageLine),
    `- [Docs-only llms.txt](${SITE_URL}/docs/llms.txt) - Documentation and REST API index without marketing pages.`,
    `- [Sitemap](${SITE_URL}/sitemap.xml) - Complete XML sitemap of every marketing and documentation URL.`,
    "",
    "---",
    "",
    `For the complete documentation in one file, see [llms-full.txt](${SITE_URL}/llms-full.txt).`,
  ];

  const llmsContent = `${llmsLines.join("\n")}\n`;
  writeFileSync(join(distDir, "llms.txt"), llmsContent, "utf8");

  const fullHeader = [
    "# dimmi (Complete Documentation)",
    "",
    "> Open-source (MIT) customer support platform: shared inbox, AI support agents, procedures, Operator, tickets, workflows, broadcasts, help center, REST API, Messenger API, MCP server, and 12 SDKs.",
    "",
    `- Root URL: ${SITE_URL}`,
    `- Index: ${SITE_URL}/llms.txt`,
    `- MCP server: ${sdks.mcp.url} (Streamable HTTP, Server Card: ${sdks.mcp.server_card})`,
    `- OpenAPI 3.1 (${sdks.api_version}): ${sdks.openapi}`,
    `- SDK catalog: ${SITE_URL}/docs/sdks.json`,
    `- Source repository: ${GITHUB_URL}`,
    "",
    ...quickReference,
    "",
    "## Product, channels & pricing",
    "",
    ...productPages.map(formatPageLine),
    ...channelPages.map(formatPageLine),
  ].join("\n");

  const docsFullBodyStart = docsFullTxt.indexOf("\n\n---\n\n");
  const docsFullBody = docsFullBodyStart !== -1 ? docsFullTxt.slice(docsFullBodyStart + 7) : docsFullTxt;
  const fullContent = `${fullHeader}\n\n---\n\n${docsFullBody}`;
  writeFileSync(join(distDir, "llms-full.txt"), fullContent, "utf8");

  return {
    llmsLines: llmsContent.trimEnd().split("\n").length,
    fullBytes: Buffer.byteLength(fullContent, "utf8"),
  };
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const { llmsLines, fullBytes } = generateLlms();
  const fullKb = Math.round(fullBytes / 1024);
  process.stdout.write(
    `llms: wrote site/dist/llms.txt (${llmsLines} lines) and site/dist/llms-full.txt (${fullKb} KB)\n`,
  );
}
