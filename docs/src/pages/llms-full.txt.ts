import { getCollection } from "astro:content";
import type { APIRoute } from "astro";

import { operationMarkdown, operations } from "~docs/api/spec";
import { withBase } from "~docs/base";
import { docMarkdown, markdownPath } from "~docs/markdown";
import { sidebar } from "~docs/nav";

// llms-full.txt: every page's Markdown in one file, guides in sidebar order
// and then the API reference, each headed by its URL.
export const GET: APIRoute = async ({ site }) => {
  const origin = site?.origin ?? "";
  const docs = new Map((await getCollection("docs")).map((entry) => [entry.id, entry]));
  const parts = ["# dimmi docs", "> Set up, run and extend dimmi, the open-source customer support platform."];
  const page = (path: string, body: string) => `Source: ${origin}${withBase(markdownPath(path) ?? "")}\n\n${body}`;
  for (const group of sidebar) {
    for (const item of group.items) {
      const entry = docs.get(item.link.replace(/^\/|\/$/g, "") || "index");
      if (entry) {
        parts.push(page(item.link, docMarkdown(entry, origin)));
      }
    }
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
