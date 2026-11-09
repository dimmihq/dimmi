/*
 * Putting pictures in an article: picked from a file, pasted or dropped,
 * or swapped in for one already there.
 * Each shows at once where the caret was, or where it was dropped, and
 * takes its real address when the upload finishes; anything that is not
 * a picture is left alone, and a refused upload is taken out and says
 * why.
 */
import type { Editor } from "@tiptap/react";
import { useRef, useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { uploadArticleImage } from "@/lib/api/help-centers";

import { settleUpload } from "./article-image";

/*
 * imagesOf keeps only the pictures among the files.
 */
export function imagesOf(files: FileList | File[] | null | undefined): File[] {
  return Array.from(files ?? []).filter((f) => f.type.startsWith("image/"));
}

export function useImageUpload(workspace: string) {
  const [uploading, setUploading] = useState(0);
  const input = useRef<HTMLInputElement | null>(null);
  const at = useRef<number | null>(null);
  const swapping = useRef<number | null>(null);

  const send = async (editor: Editor, file: File, upload: { id: string; preview: string; previous?: string }) => {
    const { id, preview, previous } = upload;
    try {
      settleUpload(editor, id, await uploadArticleImage(workspace, file), previous);
    } catch (error) {
      settleUpload(editor, id, null, previous);
      toast.error(error instanceof Error ? error.message : "The image was not uploaded.");
    } finally {
      URL.revokeObjectURL(preview);
      setUploading((n) => n - 1);
    }
  };

  /*
   * upload places a preview of every picture at the position, or at the
   * caret, then uploads each.
   */
  const upload = (editor: Editor, files: File[], position?: number) => {
    if (files.length === 0) {
      return;
    }
    const placed = files.map((file) => ({ file, id: crypto.randomUUID(), preview: URL.createObjectURL(file) }));
    const nodes = placed.map((p) => ({ type: "image", attrs: { src: p.preview, alt: "", upload: p.id } }));
    const where = position ?? at.current ?? editor.state.selection.from;
    at.current = null;
    editor.chain().focus().insertContentAt(where, nodes).run();
    setUploading((n) => n + placed.length);
    for (const p of placed) {
      void send(editor, p.file, p);
    }
  };

  /*
   * replace swaps the picture at the position for the file, keeping its
   * size, place and alt text; a refused upload puts the old one back.
   */
  const replace = (editor: Editor, position: number, file: File) => {
    const node = editor.state.doc.nodeAt(position);
    if (node?.type.name !== "image") {
      return;
    }
    const id = crypto.randomUUID();
    const preview = URL.createObjectURL(file);
    const previous = node.attrs.src as string;
    editor.view.dispatch(
      editor.state.tr.setNodeMarkup(position, undefined, { ...node.attrs, src: preview, upload: id }),
    );
    setUploading((n) => n + 1);
    void send(editor, file, { id, preview, previous });
  };

  /*
   * chosen takes the files picked in the file dialog: one swaps the
   * picture being replaced, if any; otherwise all are placed.
   */
  const chosen = (editor: Editor, files: File[]) => {
    const target = swapping.current;
    swapping.current = null;
    const [first] = files;
    if (target !== null && first) {
      replace(editor, target, first);
      return;
    }
    upload(editor, files);
  };

  /*
   * pickReplacement asks for the file to replace the selected picture
   * with.
   */
  const pickReplacement = (editor: Editor) => {
    swapping.current = editor.state.selection.from;
    input.current?.click();
  };

  /*
   * pick asks for files, remembering where the caret is so the pictures
   * land there whatever happens while the file dialog is open.
   */
  const pick = (editor: Editor) => {
    at.current = editor.state.selection.from;
    input.current?.click();
  };

  return { uploading, input, upload, pick, chosen, pickReplacement };
}
