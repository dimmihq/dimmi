/*
 * A mention in a routine's instructions: one block in the text that
 * reads as @Name and is kept in the Markdown as [[kind:ref]]. The name
 * is read from the workspace each time it is drawn, so it follows a
 * rename; a mention whose target is gone, or not loaded yet, reads as
 * the kind of thing it was, such as @team.
 */
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";

import { MentionPill } from "./mention-pill";
import { isMentionKind, looks, token } from "./mentions";

const found = /^\[\[([a-z_]+):([^\]\s]+)\]\]/;

export interface MentionChipOptions {
  /* The name of what a mention points at now, or nothing. */
  name: (kind: string, ref: string) => string;
}

export const MentionChip = Node.create<MentionChipOptions>({
  name: "routineMention",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addOptions() {
    return { name: () => "" };
  },

  addAttributes() {
    return {
      kind: { default: "" },
      ref: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "span[data-routine-mention]",
        getAttrs: (element) => {
          const [kind = "", ref = ""] = (element.getAttribute("data-routine-mention") ?? "").split(":");

          return { kind, ref };
        },
      },
    ];
  },

  renderHTML({ node, HTMLAttributes }) {
    const kind = String(node.attrs.kind);
    const ref = String(node.attrs.ref);
    const word = isMentionKind(kind) ? looks[kind].word : "something";

    return [
      "span",
      mergeAttributes(HTMLAttributes, {
        "data-routine-mention": `${kind}:${ref}`,
        class: "rounded-md bg-background-tertiary-default px-1 text-body-medium text-text-primary",
      }),
      `@${this.options.name(kind, ref) || word}`,
    ];
  },

  renderText({ node }) {
    return token(String(node.attrs.kind), String(node.attrs.ref));
  },

  addNodeView() {
    return ReactNodeViewRenderer(MentionPill);
  },

  markdownTokenName: "routineMention",
  markdownTokenizer: {
    name: "routineMention",
    level: "inline",
    start: (src: string) => src.indexOf("[["),
    tokenize: (src: string) => {
      const match = found.exec(src);
      if (!match) {
        return undefined;
      }

      return { type: "routineMention", raw: match[0], kind: match[1], ref: match[2] };
    },
  },
  parseMarkdown: (match, helpers) =>
    helpers.createNode("routineMention", { kind: match.kind as string, ref: match.ref as string }),
  renderMarkdown: (node) => token(String(node.attrs?.kind ?? ""), String(node.attrs?.ref ?? "")),
});
