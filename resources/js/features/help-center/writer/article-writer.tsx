/*
 * The article writer, loaded on its own so the docs list opens without
 * it. It draws the article with the help center's own stylesheet and
 * brand, so what a teammate writes looks the way readers will see it.
 * Selecting text raises a small bar for marks, headings, quotes and
 * links; a slash, or the plus beside an empty line, opens every block by
 * name, callouts and pictures too. Pictures can also be pasted or
 * dropped in, resized from their sides, placed in the column and
 * swapped for another file. Markdown shortcuts work as they type, yet nobody has to
 * know Markdown: what they see is kept as Markdown behind the scenes.
 * The writer owns its document once open: value is only where it starts,
 * so a slow render never puts older words back under the caret. To load
 * other words, such as a restored version, the page opens a new writer.
 */
import { type Editor, EditorContent, useEditor } from "@tiptap/react";
import { type CSSProperties, useRef, useState } from "react";

import { type Suggested, suggestionExtension } from "@/features/inbox/composer/suggestion-extension";
import { SuggestionList } from "@/features/inbox/composer/suggestion-list";

import { articleExtensions, articleMarkdown } from "./article-markdown";
import { findCommands, runCommand, type SlashCommand } from "./slash-commands";
import { imagesOf, useImageUpload } from "./use-image-upload";
import { BlockPlus, ImageBar, SelectionBar } from "./writer-menus";

export interface ArticleWriterProps {
  workspace: string;
  value: string;
  label: string;
  brand?: string;
  onChange: (markdown: string) => void;
}

/*
 * dropPosition is where in the document a drop landed, if anywhere.
 */
function dropPosition(editor: Editor, event: DragEvent): number | undefined {
  return editor.view.posAtCoords({ left: event.clientX, top: event.clientY })?.pos;
}

export default function ArticleWriter({ workspace, value, label, brand, onChange }: ArticleWriterProps) {
  const [slash, setSlash] = useState<Suggested<SlashCommand> | null>(null);
  const changed = useRef(onChange);
  changed.current = onChange;
  const images = useImageUpload(workspace);
  const editorRef = useRef<Editor | null>(null);
  const pickImage = () => {
    if (editorRef.current) {
      images.pick(editorRef.current);
    }
  };

  const editor = useEditor({
    extensions: articleExtensions([
      suggestionExtension<SlashCommand>({
        name: "slashCommand",
        char: "/",
        minLength: 0,
        find: (query) => findCommands(query),
        content: () => "",
        apply: (e, range, command) => runCommand(e, range, command, { pickImage }),
        onChange: setSlash,
      }),
    ]),
    content: value,
    contentType: "markdown",
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": label,
        class: "prose min-h-[40vh] pb-24 outline-none",
      },
      handlePaste: (_, event) => {
        const files = imagesOf(event.clipboardData?.files);
        if (files.length === 0 || !editorRef.current) {
          return false;
        }
        images.upload(editorRef.current, files);
        return true;
      },
      handleDrop: (_, event, _slice, moved) => {
        const files = imagesOf(event.dataTransfer?.files);
        if (moved || files.length === 0 || !editorRef.current) {
          return false;
        }
        event.preventDefault();
        images.upload(editorRef.current, files, dropPosition(editorRef.current, event));
        return true;
      },
    },
    onUpdate: ({ editor: e }) => changed.current(articleMarkdown(e.getMarkdown())),
  });
  editorRef.current = editor;

  if (!editor) {
    return null;
  }

  const style = brand ? ({ "--brand": brand } as CSSProperties) : undefined;

  return (
    <div className="article-brand relative" style={style}>
      <SelectionBar editor={editor} />
      <ImageBar editor={editor} onReplace={() => images.pickReplacement(editor)} />
      <BlockPlus editor={editor} />
      <SuggestionList
        label="Blocks"
        suggested={slash}
        keyOf={(command) => command.id}
        render={(command) => (
          <>
            <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border-button-default bg-background-primary-default">
              <command.icon aria-hidden className="size-4 text-foreground-icon-secondary" />
            </span>
            <span className="flex min-w-0 flex-col">
              <span className="truncate text-body-medium text-text-primary">{command.label}</span>
              <span className="truncate text-body-2-regular text-text-tertiary">{command.hint}</span>
            </span>
          </>
        )}
      />
      <input
        ref={images.input}
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        multiple
        hidden
        aria-label="Upload an image"
        onChange={(event) => {
          images.chosen(editor, imagesOf(event.target.files));
          event.target.value = "";
        }}
      />
      {images.uploading > 0 ? (
        <p role="status" className="mb-3 text-body-2-regular text-text-tertiary">
          Uploading {images.uploading === 1 ? "an image" : `${images.uploading} images`}…
        </p>
      ) : null}
      <EditorContent editor={editor} />
    </div>
  );
}
