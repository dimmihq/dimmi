/*
 * Every page also exists as Markdown, at its path plus ".md" (the home
 * page at /index.md), for the page actions menu and for LLMs; /llms.txt
 * lists them all. See src/pages/[...slug].md.ts. Paths here are inside
 * the docs; add the /docs prefix with withBase (src/base.ts).
 */
import { withBase } from "./base";

export function markdownPath(path: string): string | undefined {
  const clean = path.replace(/\/+$/, "");
  if (clean === "/404") {
    return undefined;
  }
  return clean ? `${clean}.md` : "/index.md";
}

type DocEntry = { data: { title: string; description?: string }; body?: string };

// A docs page's Markdown: its title and description, then its source
// without MDX imports and comments (drafts' notes, screenshot markers),
// with each <Screenshot> as a plain Markdown image of its light version.
export function docMarkdown(entry: DocEntry, origin = ""): string {
  const body = (entry.body ?? "")
    .replace(/^import .*$\n?/gm, "")
    .replace(/\{\/\*[\s\S]*?\*\/\}\n?/g, "")
    .replace(/<Screenshot\s+([^>]*?)\/>/g, (_, attributes: string) => {
      const name = /name="([^"]*)"/.exec(attributes)?.[1] ?? "";
      const alt = /alt="([^"]*)"/.exec(attributes)?.[1] ?? "";
      return `![${alt}](${origin}${withBase(`/shots/${name}-light.webp`)})`;
    })
    .replace(/\n{3,}/g, "\n\n")
    .trim();
  return `${[`# ${entry.data.title}`, entry.data.description, body].filter(Boolean).join("\n\n")}\n`;
}
