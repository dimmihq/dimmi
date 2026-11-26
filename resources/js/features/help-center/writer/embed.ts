/*
 * A video or demo embedded in an article, written in Markdown as
 * ::embed[title]{src="https://youtu.be/…"}. The writer plays it the way
 * the help center will, from the same allow-list of hosts; a new one
 * starts empty and asks for its link.
 */
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";

import { formatDirective, leafAt } from "./directive-syntax";
import { EmbedView } from "./embed-view";

declare module "@tiptap/react" {
  interface Commands<ReturnType> {
    embed: {
      /*
       * insertEmbed adds an embed of the link, or an empty one that asks
       * for its link.
       */
      insertEmbed: (src?: string) => ReturnType;
    };
  }
}

export const Embed = Node.create({
  name: "embed",
  group: "block",
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      src: { default: "" },
      title: { default: "" },
    };
  },

  parseHTML() {
    return [{ tag: "figure[data-embed]" }];
  },

  renderHTML({ HTMLAttributes }) {
    return ["figure", mergeAttributes(HTMLAttributes, { "data-embed": "" })];
  },

  addNodeView() {
    return ReactNodeViewRenderer(EmbedView);
  },

  addCommands() {
    return {
      insertEmbed:
        (src = "") =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs: { src, title: "" } }),
    };
  },

  markdownTokenName: "embed",

  markdownTokenizer: {
    name: "embed",
    level: "block",
    start: (src: string) => src.search(/^ {0,3}::embed\b/m),
    tokenize: (src) => {
      const found = leafAt(src, "embed");
      if (!found) {
        return undefined;
      }

      return {
        type: "embed",
        raw: found.raw,
        src: found.directive.attrs.src ?? "",
        title: found.directive.label,
      };
    },
  },

  parseMarkdown: (token, helpers) =>
    helpers.createNode("embed", { src: String(token.src ?? ""), title: String(token.title ?? "") }),

  renderMarkdown: (node) => {
    const src = String(node.attrs?.src ?? "");
    if (!src) {
      return "";
    }

    return formatDirective(2, "embed", String(node.attrs?.title ?? ""), { src });
  },
});
