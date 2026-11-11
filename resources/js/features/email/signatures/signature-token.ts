/*
 * A detail in the signature writer: one chip in the text, such as
 * "Teammate name", that becomes the teammate's own value in each reply.
 * The Markdown keeps it as the server reads it, {{member_name}}, so a
 * signature written here is stored exactly as before.
 */
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";

import { SignatureChip } from "./signature-chip";
import { signatureVariableOf, signatureVariablePattern } from "./signature-variables";

declare module "@tiptap/react" {
  interface Commands<ReturnType> {
    signatureVariable: {
      insertSignatureVariable: (key: string) => ReturnType;
    };
  }
}

/*
 * placeholderOf writes a detail the way the stored signature keeps it.
 */
function placeholderOf(key: string): string {
  return `{{${key}}}`;
}

export const SignatureToken = Node.create({
  name: "signatureVariable",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return { key: { default: "" } };
  },

  parseHTML() {
    return [
      { tag: "span[data-signature-variable]", getAttrs: (element) => ({ key: element.dataset.signatureVariable }) },
    ];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(HTMLAttributes, { "data-signature-variable": node.attrs.key }),
      signatureVariableOf(node.attrs.key as string)?.label ?? String(node.attrs.key),
    ];
  },

  renderText({ node }) {
    return placeholderOf(node.attrs.key as string);
  },

  addNodeView() {
    return ReactNodeViewRenderer(SignatureChip, { as: "span" });
  },

  addCommands() {
    return {
      insertSignatureVariable:
        (key) =>
        ({ commands }) =>
          commands.insertContent([
            { type: this.name, attrs: { key } },
            { type: "text", text: " " },
          ]),
    };
  },

  markdownTokenName: "signatureVariable",
  markdownTokenizer: {
    name: "signatureVariable",
    level: "inline",
    start: (src: string) => src.indexOf("{{"),
    tokenize: (src: string) => {
      const match = signatureVariablePattern.exec(src);
      if (!match) {
        return undefined;
      }

      return { type: "signatureVariable", raw: match[0], key: match[1] };
    },
  },
  parseMarkdown: (found, helpers) => helpers.createNode("signatureVariable", { key: found.key as string }),
  renderMarkdown: (node) => placeholderOf(String(node.attrs?.key ?? "")),
});
