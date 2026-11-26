/*
 * A button in an article: a link drawn in the help center's brand,
 * written in Markdown as ::button[Label]{href="…" align="center"}. It
 * sits at the start of the column or in its center.
 */
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";

import { ButtonView } from "./button-view";
import { formatDirective, leafAt } from "./directive-syntax";

export type ButtonAlign = "start" | "center" | "end";

declare module "@tiptap/react" {
  interface Commands<ReturnType> {
    articleButton: {
      /*
       * insertButton adds an empty button that asks for its label and link.
       */
      insertButton: () => ReturnType;
    };
  }
}

function alignOf(value: unknown): ButtonAlign {
  return value === "center" || value === "end" ? value : "start";
}

export const ArticleButton = Node.create({
  name: "articleButton",
  group: "block",
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      label: { default: "" },
      href: { default: "" },
      align: { default: "start" },
    };
  },

  parseHTML() {
    return [{ tag: "p[data-article-button]" }];
  },

  renderHTML({ HTMLAttributes }) {
    return ["p", mergeAttributes(HTMLAttributes, { "data-article-button": "" })];
  },

  addNodeView() {
    return ReactNodeViewRenderer(ButtonView);
  },

  addCommands() {
    return {
      insertButton:
        () =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs: { label: "", href: "", align: "start" } }),
    };
  },

  markdownTokenName: "articleButton",

  markdownTokenizer: {
    name: "articleButton",
    level: "block",
    start: (src: string) => src.search(/^ {0,3}::button\b/m),
    tokenize: (src) => {
      const found = leafAt(src, "button");
      if (!found) {
        return undefined;
      }

      return {
        type: "articleButton",
        raw: found.raw,
        label: found.directive.label,
        href: found.directive.attrs.href ?? "",
        align: alignOf(found.directive.attrs.align),
      };
    },
  },

  parseMarkdown: (token, helpers) =>
    helpers.createNode("articleButton", {
      label: String(token.label ?? ""),
      href: String(token.href ?? ""),
      align: alignOf(token.align),
    }),

  renderMarkdown: (node) => {
    const href = String(node.attrs?.href ?? "");
    const label = String(node.attrs?.label ?? "");
    if (!href && !label) {
      return "";
    }
    const align = alignOf(node.attrs?.align);

    return formatDirective(2, "button", label, { href, align: align === "start" ? "" : align });
  },
});
