/*
 * A callout: an aside the help center draws tinted and titled, written in
 * Markdown as a quote whose first line is [!NOTE], [!TIP], [!IMPORTANT],
 * [!WARNING] or [!CAUTION]. In the writer it is its own block, drawn the
 * way readers see it, so nobody sees or types the marker.
 */
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";

import { type CalloutKind, calloutKinds } from "./callout-kinds";
import { CalloutView } from "./callout-view";

declare module "@tiptap/react" {
  interface Commands<ReturnType> {
    callout: {
      /*
       * setCallout wraps the current block in a callout of the kind.
       */
      setCallout: (kind: CalloutKind) => ReturnType;
    };
  }
}

/*
 * calloutPattern finds a callout at the start of the text: the marker
 * line and every quoted line after it.
 */
const calloutPattern = /^ {0,3}> ?\[!(note|tip|important|warning|caution)\][ \t]*(?:\n|$)((?: {0,3}>[^\n]*(?:\n|$))*)/i;

function isKind(value: unknown): value is CalloutKind {
  return typeof value === "string" && (calloutKinds as readonly string[]).includes(value);
}

/*
 * quoted renders one block child as quoted lines.
 */
function quoted(text: string): string {
  return text
    .replace(/\n+$/, "")
    .split("\n")
    .map((line) => (line.trim() === "" ? ">" : `> ${line}`))
    .join("\n");
}

export const Callout = Node.create({
  name: "callout",
  group: "block",
  content: "block+",
  defining: true,

  addAttributes() {
    return {
      kind: {
        default: "note",
        parseHTML: (element) => element.getAttribute("data-kind") ?? "note",
        renderHTML: (attributes) => ({ "data-kind": attributes.kind }),
      },
    };
  },

  parseHTML() {
    return [{ tag: "aside[data-callout]" }];
  },

  renderHTML({ HTMLAttributes, node }) {
    return [
      "aside",
      mergeAttributes(HTMLAttributes, { "data-callout": "", class: `alert alert-${node.attrs.kind}` }),
      0,
    ];
  },

  addNodeView() {
    return ReactNodeViewRenderer(CalloutView);
  },

  addCommands() {
    return {
      setCallout:
        (kind) =>
        ({ commands }) =>
          commands.wrapIn(this.name, { kind }),
    };
  },

  markdownTokenName: "callout",

  markdownTokenizer: {
    name: "callout",
    level: "block",
    start: (src: string) => {
      const at = src.search(/^ {0,3}> ?\[!/im);
      return at < 0 ? -1 : at;
    },
    tokenize: (src, _tokens, lexer) => {
      const match = calloutPattern.exec(src);
      if (!match) {
        return undefined;
      }
      const inner = (match[2] ?? "")
        .split("\n")
        .map((line) => line.replace(/^ {0,3}> ?/, ""))
        .join("\n");

      return {
        type: "callout",
        raw: match[0],
        kind: (match[1] ?? "note").toLowerCase(),
        tokens: lexer.blockTokens(inner),
      };
    },
  },

  parseMarkdown: (token, helpers) => {
    const children = helpers.parseChildren(token.tokens ?? []);

    return {
      type: "callout",
      attrs: { kind: isKind(token.kind) ? token.kind : "note" },
      content: children.length > 0 ? children : [{ type: "paragraph" }],
    };
  },

  renderMarkdown: (node, helpers) => {
    const kind = isKind(node.attrs?.kind) ? node.attrs.kind : "note";
    const body = (node.content ?? []).map((child) => quoted(helpers.renderChildren([child]))).join("\n>\n");

    return `> [!${kind.toUpperCase()}]\n${body}`;
  },
});
