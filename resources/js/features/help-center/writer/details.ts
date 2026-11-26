/*
 * Two blocks that hold other blocks. A collapsible section shows its
 * title and opens to what it holds, written in Markdown as
 * :::details[Title] … :::; sections can hold sections, written with one
 * more colon outside. A centered block centers the text it holds, as
 * :::center … :::.
 */
import type { MarkdownTokenizer } from "@tiptap/react";
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";
import { DetailsView } from "./details-view";
import { containerAt, fenceFor, formatDirective } from "./directive-syntax";

declare module "@tiptap/react" {
  interface Commands<ReturnType> {
    details: {
      /*
       * insertDetails adds an empty section with its title to fill in.
       */
      insertDetails: () => ReturnType;
    };
    center: {
      /*
       * toggleCenter centers the selected blocks, or puts centered ones
       * back at the start of the column.
       */
      toggleCenter: () => ReturnType;
    };
  }
}

/*
 * containerTokenizer reads a container directive of the name.
 */
function containerTokenizer(name: string, type: string): MarkdownTokenizer {
  return {
    name: type,
    level: "block" as const,
    start: (src: string) => src.search(new RegExp(`^ {0,3}:{3,}${name}\\b`, "m")),
    tokenize: (src, _tokens, lexer) => {
      const found = containerAt(src, name);
      if (!found) {
        return undefined;
      }

      return { type, raw: found.raw, label: found.directive.label, tokens: lexer.blockTokens(found.inner) };
    },
  };
}

/*
 * wrapped writes a container around its children's Markdown.
 */
function wrapped(name: string, label: string, inner: string): string {
  const fence = fenceFor(inner);

  return `${formatDirective(fence, name, label)}\n${inner.replace(/\n+$/, "")}\n${":".repeat(fence)}`;
}

export const Details = Node.create({
  name: "details",
  group: "block",
  content: "block+",
  defining: true,

  addAttributes() {
    return { summary: { default: "" } };
  },

  parseHTML() {
    return [{ tag: "div[data-details]" }];
  },

  renderHTML({ HTMLAttributes }) {
    return ["div", mergeAttributes(HTMLAttributes, { "data-details": "", class: "details" }), 0];
  },

  addNodeView() {
    return ReactNodeViewRenderer(DetailsView);
  },

  addCommands() {
    return {
      insertDetails:
        () =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs: { summary: "" }, content: [{ type: "paragraph" }] }),
    };
  },

  markdownTokenName: "details",
  markdownTokenizer: containerTokenizer("details", "details"),

  parseMarkdown: (token, helpers) => {
    const children = helpers.parseChildren(token.tokens ?? []);

    return {
      type: "details",
      attrs: { summary: String(token.label ?? "") },
      content: children.length > 0 ? children : [{ type: "paragraph" }],
    };
  },

  renderMarkdown: (node, helpers) =>
    wrapped("details", String(node.attrs?.summary ?? ""), helpers.renderChildren(node.content ?? [], "\n\n")),
});

export const Center = Node.create({
  name: "center",
  group: "block",
  content: "block+",
  defining: true,

  parseHTML() {
    return [{ tag: "div[data-center]" }];
  },

  renderHTML({ HTMLAttributes }) {
    return ["div", mergeAttributes(HTMLAttributes, { "data-center": "", class: "align-center" }), 0];
  },

  addCommands() {
    return {
      toggleCenter:
        () =>
        ({ editor, commands }) =>
          editor.isActive(this.name) ? commands.lift(this.name) : commands.wrapIn(this.name),
    };
  },

  markdownTokenName: "center",
  markdownTokenizer: containerTokenizer("center", "center"),

  parseMarkdown: (token, helpers) => {
    const children = helpers.parseChildren(token.tokens ?? []);

    return { type: "center", content: children.length > 0 ? children : [{ type: "paragraph" }] };
  },

  renderMarkdown: (node, helpers) => wrapped("center", "", helpers.renderChildren(node.content ?? [], "\n\n")),
});
