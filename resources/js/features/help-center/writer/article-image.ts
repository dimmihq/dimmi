/*
 * A picture in an article. While its upload runs it shows the file from
 * the person's own computer, marked as uploading, and is left out of the
 * Markdown, so a save in the meantime never stores an address only this
 * browser can open; once uploaded it takes the address readers load it
 * from. Its width and place in the column travel in its address, so the
 * help center lays it out the same way.
 */
import Image from "@tiptap/extension-image";
import { type Editor, ReactNodeViewRenderer } from "@tiptap/react";

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

    return this.parent?.({ ...node, attrs: { ...node.attrs, src } }, helpers, context) ?? "";
  },

  addNodeView() {
    return ReactNodeViewRenderer(ImageView);
  },
}).configure({ inline: false, allowBase64: false });

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
