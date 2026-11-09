/*
 * What every formatting editor shares: waiting for the editor to be
 * ready, taking Markdown that changed outside it, and setting a link.
 */
import type { Editor } from "@tiptap/react";

/*
 * applyLink puts a link over the selection, or removes it when the
 * address is empty.
 */
export function applyLink(editor: Editor, href: string) {
  const chain = editor.chain().focus().extendMarkRange("link");
  if (href.trim() === "") {
    chain.unsetLink().run();
    return;
  }
  chain.setLink({ href: href.trim() }).run();
}

/*
 * whenReady runs the step on the editor once it is set up, at once when
 * it already is, and returns what stops waiting.
 */
export function whenReady(editor: Editor | null, step: (ready: Editor) => unknown): () => void {
  if (!editor || editor.isDestroyed) {
    return () => undefined;
  }
  if (editor.isInitialized) {
    step(editor);
    return () => undefined;
  }
  const run = () => step(editor);
  editor.on("create", run);

  return () => editor.off("create", run);
}

/*
 * syncContent puts Markdown that changed outside the editor, such as a
 * draft from another device, into it without counting as typing.
 */
export function syncContent(editor: Editor, value: string) {
  if (editor.getMarkdown() !== value) {
    editor.commands.setContent(value, { contentType: "markdown", emitUpdate: false });
  }
}
