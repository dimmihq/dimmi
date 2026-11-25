import { getCollection } from "astro:content";
import type { APIRoute } from "astro";

import { API_VERSION, operations } from "~docs/api/spec";
import { withBase } from "~docs/base";
import { markdownPath } from "~docs/markdown";
import { GITHUB_URL, type Section, sections, sidebar } from "~docs/nav";
import { SDK_CATALOG } from "~docs/sdks";

type DocData = { title: string; description?: string };
type DocEntry = { id: string; data: DocData };
type SidebarGroup = (typeof sidebar)[number];

function itemId(link: string): string {
  return link.replace(/^\/|\/$/g, "") || "index";
}

function summarizeSections(groupsByLabel: Map<string, SidebarGroup>) {
  const listedIds = new Set<string>(["api-reference"]);
  const counts = sections.map((section) => {
    if (section.prefix) {
      return { label: section.label, count: operations.length + 1, href: section.href };
    }
    let count = 0;
    for (const label of section.groups) {
      const group = groupsByLabel.get(label);
      if (!group) {
        continue;
      }
      for (const item of group.items) {
        listedIds.add(itemId(item.link));
        count++;
      }
    }
    return { label: section.label, count, href: section.href };
  });
  return { listedIds, counts };
}

function renderSection(
  section: Section,
  groupsByLabel: Map<string, SidebarGroup>,
  docs: Map<string, DocData>,
  origin: string,
): string[] {
  const lines = [`## ${section.label}`, ""];
  for (const label of section.groups) {
    const group = groupsByLabel.get(label);
    if (!group) {
      continue;
    }
    lines.push(`### ${group.label}`, "");
    for (const item of group.items) {
      const description = docs.get(itemId(item.link))?.description;
      const url = `${origin}${withBase(markdownPath(item.link) ?? "")}`;
      lines.push(`- [${item.label}](${url})${description ? ` - ${description}` : ""}`);
    }
    lines.push("");
  }
  return lines;
}

function renderUnlisted(unlisted: DocEntry[], origin: string): string[] {
  if (unlisted.length === 0) {
    return [];
  }
  const lines = ["## Other pages", ""];
  for (const entry of unlisted) {
    const url = `${origin}${withBase(markdownPath(`/${entry.id}`) ?? "")}`;
    const description = entry.data.description;
    lines.push(`- [${entry.data.title}](${url})${description ? ` - ${description}` : ""}`);
  }
  lines.push("");
  return lines;
}

function renderApiReference(origin: string): string[] {
  const lines = [
    "## API reference",
    "",
    `- [API reference overview](${origin}${withBase("/api-reference.md")}) - Base URL, authentication, abilities, pagination, rate limits, and errors (${operations.length} public endpoints, version ${API_VERSION}).`,
  ];
  let category = "";
  for (const op of operations) {
    if (op.category !== category) {
      category = op.category;
      lines.push("", `### ${category}`, "");
    }
    lines.push(`- [${op.title}](${origin}${withBase(markdownPath(op.href) ?? "")}) - \`${op.method} ${op.path}\``);
  }
  return lines;
}

// The docs llms.txt index (llmstxt.org): machine-readable discovery metadata
// plus every docs page and REST API endpoint as Markdown, grouped by section.
export const GET: APIRoute = async ({ site }) => {
  const origin = site?.origin ?? "https://trydimmi.com";
  const collection = (await getCollection("docs")).filter((entry) => entry.id !== "404");
  const docs = new Map(collection.map((entry) => [entry.id, entry.data]));
  const groupsByLabel = new Map(sidebar.map((group) => [group.label, group]));
  const { listedIds, counts } = summarizeSections(groupsByLabel);
  const unlisted = collection.filter((entry) => !listedIds.has(entry.id)).sort((a, b) => a.id.localeCompare(b.id));

  const lines: string[] = [
    "# dimmi Documentation",
    "",
    "> Set up, run, and extend dimmi, the open-source (MIT) customer support platform with a shared inbox, AI support agents, procedures, Operator, tickets, workflows, and help center.",
    "",
    "## Root URL",
    "",
    `dimmi Cloud & marketing site: ${origin}`,
    `Documentation: ${origin}${withBase("/")}`,
    `REST API (${API_VERSION}): ${origin}/api/v1`,
    `Messenger API: ${origin}/widget/v1`,
    `Open-source repository (MIT): ${GITHUB_URL}`,
    "",
    "## MCP server",
    "",
    `Connect an MCP client to ${origin}/mcp (Streamable HTTP transport, OAuth 2.0 or Bearer API token) to search knowledge, manage conversations and tickets, test procedures, and propose reviewable Operator edits.`,
    `OAuth discovery metadata: ${origin}/.well-known/oauth-authorization-server`,
    `Server card: ${origin}${withBase("/mcp/server-card.json")}`,
    `MCP guide: ${origin}${withBase("/developers/mcp.md")}`,
    "",
    "## OpenAPI, SDKs & coding agent skill",
    "",
    `- OpenAPI 3.1 contract (${API_VERSION}): ${origin}/api/v1/openapi.json`,
    `- SDK catalog (${SDK_CATALOG.length} browser & backend packages, JSON): ${origin}${withBase("/sdks.json")}`,
    `- Install skill for Claude Code, Cursor & Codex: ${origin}${withBase("/developers/ai-coding-agent.md")} (raw SKILL.md: https://raw.githubusercontent.com/dimmihq/dimmi/main/sdk/skills/dimmi-install/SKILL.md)`,
    "",
    "## Documentation overview",
    "",
    ...counts.map((section) => `- ${section.label} (${section.count} pages) - ${origin}${withBase(section.href)}`),
    "",
    "---",
    "",
    ...sections
      .filter((section) => !section.prefix)
      .flatMap((section) => renderSection(section, groupsByLabel, docs, origin)),
    ...renderUnlisted(unlisted, origin),
    ...renderApiReference(origin),
    "",
    "---",
    "",
    `For the full site index including product & pricing pages, see [llms.txt](${origin}/llms.txt).`,
    `For the complete documentation in one file, see [llms-full.txt](${origin}/llms-full.txt) (or [docs/llms-full.txt](${origin}${withBase("/llms-full.txt")})).`,
  ];

  return new Response(`${lines.join("\n")}\n`, { headers: { "Content-Type": "text/plain; charset=utf-8" } });
};
