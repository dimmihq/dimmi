/*
 * A mention in the composer: one block in the text that names a teammate
 * or a team and reads as @Name, kept in the Markdown as <@mem_...> or
 * <@team_...>, the token the server reads. A mention written before the
 * names are known, such as in a draft opened on another device, reads as
 * @teammate until it is sent; the thread shows the real name.
 */
import { mergeAttributes, Node } from "@tiptap/react";

const token = /^<@((?:mem|team)_[0-9A-Za-z]{1,40})>/;

export const MentionToken = Node.create({
  name: "mentionToken",
  group: "inline",
  inline: true,
  atom: true,
  selectable: false,

  addAttributes() {
    return {
      id: { default: "" },
      label: { default: null },
    };
  },

  parseHTML() {
    return [
      {
        tag: "span[data-mention]",
        getAttrs: (element) => ({
          id: element.getAttribute("data-mention"),
          label: element.getAttribute("data-label"),
        }),
      },
    ];
  },

  renderHTML({ node, HTMLAttributes }) {
    const label =
      (node.attrs.label as string | null) ?? (String(node.attrs.id).startsWith("team_") ? "team" : "teammate");

    return [
      "span",
      mergeAttributes(HTMLAttributes, {
        "data-mention": node.attrs.id,
        "data-label": node.attrs.label,
        class: "rounded-md bg-background-tertiary-default px-1 text-body-medium text-text-primary",
      }),
      `@${label}`,
    ];
  },

  renderText({ node }) {
    return `<@${node.attrs.id}>`;
  },

  markdownTokenName: "mentionToken",
  markdownTokenizer: {
    name: "mentionToken",
    level: "inline",
    start: (src: string) => src.indexOf("<@"),
    tokenize: (src: string) => {
      const match = token.exec(src);
      if (!match) {
        return undefined;
      }

      return { type: "mentionToken", raw: match[0], id: match[1] };
    },
  },
  parseMarkdown: (found, helpers) => helpers.createNode("mentionToken", { id: found.id as string }),
  renderMarkdown: (node) => `<@${node.attrs?.id ?? ""}>`,
});
