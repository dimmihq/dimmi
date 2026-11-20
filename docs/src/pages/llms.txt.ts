import { getCollection } from "astro:content";
import type { APIRoute } from "astro";

import { operations } from "~docs/api/spec";
import { withBase } from "~docs/base";
import { markdownPath } from "~docs/markdown";
import { sidebar } from "~docs/nav";

// The llms.txt index (llmstxt.org): every page's Markdown, by group.
export const GET: APIRoute = async ({ site }) => {
  const origin = site?.origin ?? "";
  const docs = new Map((await getCollection("docs")).map((entry) => [entry.id, entry.data]));
  const lines = ["# dimmi docs", "", "> Set up, run and extend dimmi, the open-source customer support platform.", ""];
  for (const group of sidebar) {
    lines.push(`## ${group.label}`, "");
    for (const item of group.items) {
      const id = item.link.replace(/^\/|\/$/g, "") || "index";
      const description = docs.get(id)?.description;
      lines.push(
        `- [${item.label}](${origin}${withBase(markdownPath(item.link) ?? "")})${description ? `: ${description}` : ""}`,
      );
    }
    lines.push("");
  }
  lines.push("## API reference", "", `- [Introduction](${origin}${withBase("/api-reference.md")})`);
  let category = "";
  for (const op of operations) {
    if (op.category !== category) {
      category = op.category;
      lines.push("", `### ${category}`, "");
    }
    lines.push(`- [${op.title}](${origin}${withBase(markdownPath(op.href) ?? "")}): \`${op.method} ${op.path}\``);
  }
  return new Response(`${lines.join("\n")}\n`, { headers: { "Content-Type": "text/plain; charset=utf-8" } });
};
