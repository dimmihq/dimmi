/*
 * A picture in an article. Its title is the caption readers see under
 * it, and it can link somewhere, written the usual Markdown way as
 * [![alt](src "caption")](href). While its upload runs it shows the file from
 * the person's own computer, marked as uploading, and is left out of the
 * Markdown, so a save in the meantime never stores an address only this
 * browser can open; once uploaded it takes the address readers load it
 * from. Its width and place in the column travel in its address, so the
 * help center lays it out the same way.
 */
import Image from "@tiptap/extension-image";
import { type Editor, Extension, ReactNodeViewRenderer } from "@tiptap/react";

import { joinLayout, splitLayout } from "./image-layout";
import { ImageView } from "./image-view";

export const ArticleImage = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      upload: {
        default: null,
        parseHTML: () => null,
        renderHTML: (attributes) => (attributes.upload ? { "data-uploading": "" } : {}),
      },
      width: { default: null, parseHTML: () => null, renderHTML: () => ({}) },
      align: { default: null, parseHTML: () => null, renderHTML: () => ({}) },
      href: { default: null, parseHTML: () => null, renderHTML: () => ({}) },
    };
  },

  parseMarkdown(token, helpers) {
    const { src, width, align } = splitLayout(token.href ?? "");

    return helpers.createNode("image", { src, width, align, title: token.title, alt: token.text });
  },

  renderMarkdown(node, helpers, context) {
    if (node.attrs?.upload) {
      return "";
    }
    const src = joinLayout(node.attrs?.src ?? "", {
      width: node.attrs?.width ?? null,
      align: node.attrs?.align ?? null,
    });

    const caption = typeof node.attrs?.title === "string" ? node.attrs.title.trim() : "";
    const title = caption ? caption.replace(/[\\"]/g, (c) => `\\${c}`) : null;
    const image = this.parent?.({ ...node, attrs: { ...node.attrs, src, title } }, helpers, context) ?? "";
    const href = typeof node.attrs?.href === "string" ? node.attrs.href.trim() : "";

    return href && image ? `[${image}](${href.replace(/[()\s]/g, encodeURIComponent)})` : image;
  },

  addNodeView() {
    return ReactNodeViewRenderer(ImageView);
  },
}).configure({ inline: false, allowBase64: false });

/*
 * linkedImagePattern is a picture alone on its line inside a link.
 */
const linkedImagePattern =
  /^ {0,3}\[!\[((?:[^\]\\]|\\.)*)\]\(\s*(\S+?)(?:\s+"((?:[^"\\]|\\.)*)")?\s*\)\]\(\s*(\S+?)\s*\)[ \t]*(?:\n|$)/;

/*
 * LinkedImage reads a picture wrapped in a link as a picture that links.
 */
export const LinkedImage = Extension.create({
  name: "linkedImage",

  markdownTokenName: "linkedImage",

  markdownTokenizer: {
    name: "linkedImage",
    level: "block",
    start: (src: string) => src.search(/^ {0,3}\[!\[/m),
    tokenize: (src) => {
      const match = linkedImagePattern.exec(src);
      if (!match) {
        return undefined;
      }
      const unescaped = (text = "") => text.replace(/\\(.)/g, "$1");

      return {
        type: "linkedImage",
        raw: match[0],
        alt: unescaped(match[1]),
        href: match[2] ?? "",
        title: match[3] === undefined ? null : unescaped(match[3]),
        link: match[4] ?? "",
      };
    },
  },

  parseMarkdown: (token, helpers) => {
    const { src, width, align } = splitLayout(String(token.href ?? ""));

    return helpers.createNode("image", {
      src,
      width,
      align,
      title: token.title ?? null,
      alt: String(token.alt ?? ""),
      href: String(token.link ?? "") || null,
    });
  },
});

/*
 * findUpload returns where the picture still uploading under the id is,
 * if it is still in the document.
 */
function findUpload(editor: Editor, id: string): number | null {
  let found: number | null = null;
  editor.state.doc.descendants((node, pos) => {
    if (found === null && node.type.name === "image" && node.attrs.upload === id) {
      found = pos;
    }
    return found === null;
  });

  return found;
}

/*
 * settleUpload gives the uploading picture its address. When the upload
 * failed it goes back to the picture it replaced, if any, or is taken
 * out.
 */
export function settleUpload(editor: Editor, id: string, src: string | null, previous?: string) {
  const at = findUpload(editor, id);
  if (at === null) {
    return;
  }
  const { tr } = editor.state;
  if (src === null && previous) {
    const node = tr.doc.nodeAt(at);
    editor.view.dispatch(tr.setNodeMarkup(at, undefined, { ...node?.attrs, src: previous, upload: null }));
    return;
  }
  if (src === null) {
    const node = tr.doc.nodeAt(at);
    editor.view.dispatch(tr.delete(at, at + (node?.nodeSize ?? 1)));
    return;
  }
  const node = tr.doc.nodeAt(at);
  editor.view.dispatch(tr.setNodeMarkup(at, undefined, { ...node?.attrs, src, upload: null }));
}
