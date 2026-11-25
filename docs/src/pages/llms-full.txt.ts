import { getCollection } from "astro:content";
import type { APIRoute } from "astro";

import { API_VERSION, operationMarkdown, operations } from "~docs/api/spec";
import { withBase } from "~docs/base";
import { docMarkdown, markdownPath } from "~docs/markdown";
import { GITHUB_URL, sidebar } from "~docs/nav";

// llms-full.txt: every docs page and API endpoint as Markdown in one file,
// guides in sidebar order, any additional pages, and then the API reference,
// each headed by its canonical Markdown Source URL.
export const GET: APIRoute = async ({ site }) => {
  const origin = site?.origin ?? "https://trydimmi.com";
  const collection = (await getCollection("docs")).filter((entry) => entry.id !== "404");
  const docs = new Map(collection.map((entry) => [entry.id, entry]));

  const header = [
    "# dimmi Documentation (Full)",
    "",
    "> Complete Markdown documentation and REST API reference for dimmi, the open-source (MIT) customer support platform.",
    "",
    `- Root URL: ${origin}`,
    `- Docs index: ${origin}/llms.txt`,
    `- MCP server: ${origin}/mcp (Streamable HTTP, Server Card: ${origin}${withBase("/mcp/server-card.json")})`,
    `- OpenAPI 3.1 (${API_VERSION}): ${origin}/api/v1/openapi.json`,
    `- SDK catalog: ${origin}${withBase("/sdks.json")}`,
    `- Source repository: ${GITHUB_URL}`,
  ].join("\n");

  const parts: string[] = [header];
  const included = new Set<string>(["api-reference"]);
  const page = (path: string, body: string) => `Source: ${origin}${withBase(markdownPath(path) ?? "")}\n\n${body}`;

  for (const group of sidebar) {
    for (const item of group.items) {
      const id = item.link.replace(/^\/|\/$/g, "") || "index";
      const entry = docs.get(id);
      if (entry) {
        included.add(id);
        parts.push(page(item.link, docMarkdown(entry, origin)));
      }
    }
  }

  for (const entry of collection.filter((item) => !included.has(item.id)).sort((a, b) => a.id.localeCompare(b.id))) {
    parts.push(page(`/${entry.id}`, docMarkdown(entry, origin)));
  }

  const intro = docs.get("api-reference");
  if (intro) {
    parts.push(page("/api-reference/", docMarkdown(intro, origin)));
  }
  for (const op of operations) {
    parts.push(page(op.href, operationMarkdown(op)));
  }

  return new Response(`${parts.join("\n\n---\n\n")}\n`, { headers: { "Content-Type": "text/plain; charset=utf-8" } });
};
