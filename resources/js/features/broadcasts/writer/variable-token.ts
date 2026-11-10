/*
 * A variable in the message writer: one chip in the text, such as
 * "First name", that becomes the person's own value when the message
 * goes out. Picking the chip edits the fallback for people without one.
 * The Markdown keeps it in Liquid's form, which the server reads.
 */
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";

import { VariableView } from "./variable-view";
import { variableMarkdown, variableOf, variablePattern } from "./variables";

declare module "@tiptap/react" {
  interface Commands<ReturnType> {
    variableToken: {
      insertVariable: (key: string) => ReturnType;
    };
  }
}

export const VariableToken = Node.create({
  name: "variableToken",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      key: { default: "" },
      fallback: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "span[data-variable]",
        getAttrs: (element) => ({
          key: element.getAttribute("data-variable"),
          fallback: element.getAttribute("data-fallback") ?? "",
        }),
      },
    ];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(HTMLAttributes, { "data-variable": node.attrs.key, "data-fallback": node.attrs.fallback }),
      variableOf(node.attrs.key as string)?.label ?? String(node.attrs.key),
    ];
  },

  renderText({ node }) {
    return variableMarkdown(node.attrs.key as string, node.attrs.fallback as string);
  },

  addNodeView() {
    return ReactNodeViewRenderer(VariableView, { as: "span" });
  },

  addCommands() {
    return {
      insertVariable:
        (key) =>
        ({ commands }) =>
          commands.insertContent([
            { type: this.name, attrs: { key, fallback: variableOf(key)?.fallback ?? "" } },
            { type: "text", text: " " },
          ]),
    };
  },

  markdownTokenName: "variableToken",
  markdownTokenizer: {
    name: "variableToken",
    level: "inline",
    start: (src: string) => src.indexOf("{{"),
    tokenize: (src: string) => {
      const match = variablePattern.exec(src);
      if (!match) {
        return undefined;
      }

      return { type: "variableToken", raw: match[0], key: match[1], fallback: match[2] ?? "" };
    },
  },
  parseMarkdown: (found, helpers) =>
    helpers.createNode("variableToken", { key: found.key as string, fallback: found.fallback as string }),
  renderMarkdown: (node) => variableMarkdown(String(node.attrs?.key ?? ""), String(node.attrs?.fallback ?? "")),
});
