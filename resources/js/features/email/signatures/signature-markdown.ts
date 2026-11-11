/*
 * A signature's document: only what a reply's signature can carry,
 * which is lines of text with bold, italic and links, plus the details
 * filled in for each reply. Anything more would be stripped by the
 * server, so the writer offers nothing more. Bold, italic and links
 * cover a detail as well as text, so "**{{member_name}}**" stays bold
 * however often the signature is opened and saved.
 */
import Bold from "@tiptap/extension-bold";
import Italic from "@tiptap/extension-italic";
import Link from "@tiptap/extension-link";
import { Placeholder } from "@tiptap/extensions";
import { Markdown } from "@tiptap/markdown";
import type { AnyExtension, Editor, JSONContent, MarkdownParseHelpers, MarkdownToken } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";

import { SignatureToken } from "./signature-token";
import { fromWriter } from "./signature-variables";

/*
 * marked puts the mark on every text and detail inside it, and on the
 * text and details nested deeper.
 */
function marked(type: string, content: JSONContent[], attrs?: Record<string, unknown>): JSONContent[] {
  const mark = attrs ? { type, attrs } : { type };

  return content.map((node) => {
    if (node.type === "text" || node.type === SignatureToken.name) {
      return { ...node, marks: [...(node.marks ?? []), mark] };
    }

    return node.content ? { ...node, content: marked(type, node.content, attrs) } : node;
  });
}

/*
 * parseMarked reads one marked span of Markdown.
 */
function parseMarked(
  type: string,
  token: MarkdownToken,
  helpers: MarkdownParseHelpers,
  attrs?: Record<string, unknown>,
) {
  return marked(type, helpers.parseInline(token.tokens ?? []), attrs);
}

/*
 * signatureExtensions is the signature schema with Markdown in and out,
 * plus any helpers, such as the slash menu.
 */
export function signatureExtensions(helpers: AnyExtension[] = []): AnyExtension[] {
  return [
    StarterKit.configure({
      heading: false,
      blockquote: false,
      bulletList: false,
      orderedList: false,
      listItem: false,
      listKeymap: false,
      codeBlock: false,
      code: false,
      strike: false,
      underline: false,
      horizontalRule: false,
      bold: false,
      italic: false,
      link: false,
    }),
    Bold.extend({ parseMarkdown: (token, h) => parseMarked("bold", token, h) }),
    Italic.extend({ parseMarkdown: (token, h) => parseMarked("italic", token, h) }),
    Link.configure({ openOnClick: false, autolink: true, protocols: ["http", "https", "mailto", "tel"] }).extend({
      parseMarkdown: (token, h) => parseMarked("link", token, h, { href: token.href, title: token.title ?? null }),
    }),
    Placeholder.configure({ placeholder: "Write your signature, or type / for a detail" }),
    Markdown,
    SignatureToken,
    ...helpers,
  ];
}

/*
 * chipsAsText writes every detail as its placeholder in plain text,
 * keeping its marks, since Markdown only marks text.
 */
function chipsAsText(node: JSONContent): JSONContent {
  if (node.type === SignatureToken.name) {
    return { type: "text", text: `{{${String(node.attrs?.key ?? "")}}}`, marks: node.marks };
  }

  return node.content ? { ...node, content: node.content.map(chipsAsText) } : node;
}

/*
 * signatureMarkdown is what the writer holds, in the form the signature
 * is stored in: placeholders as the server reads them and a lone line
 * break for each break.
 */
export function signatureMarkdown(editor: Editor): string {
  const markdown = editor.markdown?.serialize(chipsAsText(editor.getJSON())) ?? "";

  return fromWriter(markdown.replace(/\{\{([a-z\\_]+)\}\}/g, (_, key: string) => `{{${key.replaceAll("\\_", "_")}}}`));
}
