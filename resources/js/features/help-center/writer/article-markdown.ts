/*
 * An article's document: everything a help center page shows. Headings,
 * paragraphs, bold, italic, strike, code, links, lists, quotes,
 * callouts, rules, images and tables. Articles are stored as Markdown,
 * so the writer reads and writes Markdown only. An empty line always
 * follows the last block, so there is somewhere to write after a
 * callout, a table or a picture.
 */
import { TableKit } from "@tiptap/extension-table";
import { Placeholder, TrailingNode } from "@tiptap/extensions";
import { Markdown } from "@tiptap/markdown";
import type { AnyExtension } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";

import { ArticleImage } from "./article-image";
import { Callout } from "./callout";

/*
 * placeholderFor is the hint on an empty line: where to start in an
 * empty article, the level in an empty heading, and the slash elsewhere.
 */
function placeholderFor({
  node,
  editor,
}: {
  node: { type: { name: string }; attrs: Record<string, unknown> };
  editor: { isEmpty: boolean };
}): string {
  if (node.type.name === "heading") {
    return node.attrs.level === 3 ? "Subheading" : "Heading";
  }

  return editor.isEmpty ? "Start writing, or type / for blocks" : "Type / for blocks";
}

/*
 * articleExtensions is the article schema with Markdown in and out, plus
 * any helpers, such as the slash menu.
 */
export function articleExtensions(helpers: AnyExtension[] = []): AnyExtension[] {
  return [
    StarterKit.configure({
      heading: { levels: [2, 3] },
      underline: false,
      link: { openOnClick: false, autolink: true, protocols: ["http", "https", "mailto"] },
    }),
    ArticleImage,
    TableKit.configure({ table: { resizable: false } }),
    Callout,
    TrailingNode,
    Placeholder.configure({ placeholder: placeholderFor, showOnlyCurrent: true, includeChildren: true }),
    Markdown,
    ...helpers,
  ];
}

/*
 * articleMarkdown is the editor's Markdown with callout markers as
 * written, for quotes that still carry one as text.
 */
export function articleMarkdown(markdown: string): string {
  return markdown.replace(/^(>\s*)\\\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\\\]/gm, "$1[!$2]");
}
