/*
 * A tool named in a routine's prompt: one atom in the text, drawn as a
 * chip, written into the prompt's Markdown as [[tool:name]] and read
 * back from it. The chip ends with a space so typing goes on after it.
 */
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";

import { ToolChip } from "./tool-chip";

const chip = /^\[\[tool:([a-z0-9_]+)\]\]/;

export const ToolToken = Node.create({
  name: "routineTool",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return { name: { default: "" } };
  },

  parseHTML() {
    return [{ tag: "span[data-tool]", getAttrs: (element) => ({ name: element.getAttribute("data-tool") }) }];
  },

  renderHTML({ node, HTMLAttributes }) {
    return ["span", mergeAttributes(HTMLAttributes, { "data-tool": node.attrs.name }), String(node.attrs.name)];
  },

  renderText({ node }) {
    return `[[tool:${node.attrs.name}]]`;
  },

  addNodeView() {
    return ReactNodeViewRenderer(ToolChip, { as: "span" });
  },

  markdownTokenName: "routineTool",
  markdownTokenizer: {
    name: "routineTool",
    level: "inline",
    start: (src: string) => src.indexOf("[[tool:"),
    tokenize: (src: string) => {
      const found = chip.exec(src);
      if (!found) {
        return undefined;
      }
      return { type: "routineTool", raw: found[0], name: found[1] };
    },
  },
  parseMarkdown: (found, helpers) => helpers.createNode("routineTool", { name: found.name as string }),
  renderMarkdown: (node) => `[[tool:${String(node.attrs?.name ?? "")}]]`,
});
