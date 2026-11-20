/*
 * A variable in the message writer: one chip in the text, such as
 * "First name", that becomes the person's own value when the message
 * goes out. Picking the chip edits the fallback for people without one.
 * The Markdown keeps it in Liquid's form, which the server reads.
 */
import { Plugin } from "@tiptap/pm/state";
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

/*
 * VariableOptions lets a writer with details of its own, such as a
 * WhatsApp template's custom values, name them on their chips, and say
 * which are filled in by hand and so take no fallback.
 */
export interface VariableOptions {
  named: (key: string) => string | undefined;
  byHand: (key: string) => boolean;
}

export const VariableToken = Node.create<VariableOptions>({
  name: "variableToken",

  addOptions() {
    return {
      named: (key) => variableOf(key)?.label,
      byHand: () => false,
    };
  },

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
      this.options.named(node.attrs.key as string) ?? String(node.attrs.key),
    ];
  },

  renderText({ node }) {
    return variableMarkdown(node.attrs.key as string, node.attrs.fallback as string);
  },

  addNodeView() {
    return ReactNodeViewRenderer(VariableView, { as: "span" });
  },

  addProseMirrorPlugins() {
    return [
      new Plugin({
        props: {
          // A chip ends with a space so typing goes on after it; punctuation
          // typed straight after takes that space back, so "Hi Ada," reads
          // without a gap.
          handleTextInput(view, from, to, text) {
            const { doc } = view.state;
            if (!/^[,.!?;:)]$/.test(text) || from !== to || from < 2) {
              return false;
            }
            const before = doc.resolve(from - 1).nodeBefore;
            if (doc.textBetween(from - 1, from) !== " " || before?.type.name !== "variableToken") {
              return false;
            }
            view.dispatch(view.state.tr.insertText(text, from - 1, from));

            return true;
          },
        },
      }),
    ];
  },

  addCommands() {
    return {
      insertVariable:
        (key) =>
        ({ commands }) =>
          commands.insertContent([
            {
              type: this.name,
              attrs: { key, fallback: this.options.byHand(key) ? "" : (variableOf(key)?.fallback ?? "") },
            },
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
