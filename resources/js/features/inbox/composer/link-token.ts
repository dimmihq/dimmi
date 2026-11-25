/*
 * A link to the help center in the writer: a help article, a feature
 * request or a changelog entry, shown as one chip with its title rather
 * than a long address. The Markdown keeps it as a plain link,
 * [title](url), so the customer gets a link they can open on any
 * channel; links elsewhere stay ordinary links.
 */
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";

import { LinkView } from "./link-view";

/*
 * libraryLink matches a Markdown link to a help center page the add
 * menu brings in: an article, a feature request or a changelog entry.
 */
const libraryLink = /^\[([^\]\n]+)\]\((https?:\/\/[^)\s]+\/(?:articles|feedback|changelog)\/[^)\s]+)\)/;

export type LinkKind = "article" | "request" | "changelog";

/*
 * kindOf says what a help center link points at.
 */
export function kindOf(url: string): LinkKind {
  if (url.includes("/feedback/")) {
    return "request";
  }

  return url.includes("/changelog/") ? "changelog" : "article";
}

/*
 * linkMarkdown writes a chip the way the Markdown keeps it.
 */
export function linkMarkdown(title: string, url: string): string {
  return `[${title.replaceAll("]", ")").replaceAll("[", "(")}](${url})`;
}

export const LinkToken = Node.create({
  name: "linkToken",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      title: { default: "" },
      url: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "span[data-link-token]",
        getAttrs: (element) => ({
          title: element.getAttribute("data-title") ?? "",
          url: element.getAttribute("data-link-token") ?? "",
        }),
      },
    ];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(HTMLAttributes, { "data-link-token": node.attrs.url, "data-title": node.attrs.title }),
      String(node.attrs.title),
    ];
  },

  renderText({ node }) {
    return linkMarkdown(String(node.attrs.title), String(node.attrs.url));
  },

  addNodeView() {
    return ReactNodeViewRenderer(LinkView, { as: "span" });
  },

  markdownTokenName: "linkToken",
  markdownTokenizer: {
    name: "linkToken",
    level: "inline",
    start: (src: string) => src.indexOf("["),
    tokenize: (src: string) => {
      const match = libraryLink.exec(src);
      if (!match) {
        return undefined;
      }

      return { type: "linkToken", raw: match[0], title: match[1], url: match[2] };
    },
  },
  parseMarkdown: (found, helpers) =>
    helpers.createNode("linkToken", { title: found.title as string, url: found.url as string }),
  renderMarkdown: (node) => linkMarkdown(String(node.attrs?.title ?? ""), String(node.attrs?.url ?? "")),
});
