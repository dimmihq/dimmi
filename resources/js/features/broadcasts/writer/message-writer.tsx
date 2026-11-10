/*
 * The writer for a broadcast's message: the article writer the help
 * center and the changelog use, with the same bar over selected text,
 * the plus beside an empty line, the slash for blocks and pictures
 * pasted or dropped in, plus variables. A message can name the person
 * it goes to: the slash menu lists every detail, and typing two braces
 * offers them too, each kept as a chip. Whatever is written comes out
 * as Markdown, which the server turns into an email, a chat message or
 * a post when it sends.
 */
import { RiBracesLine } from "@remixicon/react";
import { type Editor, EditorContent, useEditor } from "@tiptap/react";
import { useRef, useState } from "react";

import { articleExtensions, articleMarkdown } from "@/features/help-center/writer/article-markdown";
import { findCommands, runCommand, type SlashCommand } from "@/features/help-center/writer/slash-commands";
import { imagesOf, useImageUpload } from "@/features/help-center/writer/use-image-upload";
import { BlockPlus, ImageBar, SelectionBar } from "@/features/help-center/writer/writer-menus";
import { type Suggested, suggestionExtension } from "@/features/inbox/composer/suggestion-extension";
import { SuggestionList } from "@/features/inbox/composer/suggestion-list";

import { VariableToken } from "./variable-token";
import { findVariables, type Variable, variables } from "./variables";

export interface MessageWriterProps {
  workspace: string;
  value: string;
  label: string;
  onChange: (markdown: string) => void;
}

const variableCommands: SlashCommand[] = variables.map((variable) => ({
  id: `variable:${variable.key}`,
  label: variable.label,
  hint: `Variable, e.g. ${variable.sample}`,
  icon: RiBracesLine,
  words: ["variable", "name", ...variable.key.split(/[._]/)],
  run: (chain) => chain.insertVariable(variable.key),
}));

/*
 * findBlocks lists the article's blocks that match what was typed after
 * the slash, then the variables that do.
 */
function findBlocks(query: string): SlashCommand[] {
  const q = query.trim().toLowerCase();
  const named = variableCommands.filter(
    (command) => command.label.toLowerCase().includes(q) || command.words.some((word) => word.startsWith(q)),
  );

  return [...findCommands(query), ...named];
}

function useMessageEditor({ workspace, value, label, onChange }: MessageWriterProps) {
  const [slash, setSlash] = useState<Suggested<SlashCommand> | null>(null);
  const [braces, setBraces] = useState<Suggested<Variable> | null>(null);
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
      VariableToken,
      suggestionExtension<SlashCommand>({
        name: "slashCommand",
        char: "/",
        minLength: 0,
        find: findBlocks,
        content: () => "",
        apply: (e, range, command) => runCommand(e, range, command, { pickImage }),
        onChange: setSlash,
      }),
      suggestionExtension<Variable>({
        name: "variableMenu",
        char: "{{",
        minLength: 0,
        find: findVariables,
        content: () => "",
        apply: (e, range, variable) => e.chain().focus().deleteRange(range).insertVariable(variable.key).run(),
        onChange: setBraces,
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
    },
    onUpdate: ({ editor: e }) => changed.current(articleMarkdown(e.getMarkdown())),
  });
  editorRef.current = editor;

  return { editor, slash, braces, images };
}

function Option({ label, hint, icon: Icon }: { label: string; hint: string; icon: SlashCommand["icon"] }) {
  return (
    <>
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border-button-default bg-background-primary-default">
        <Icon aria-hidden className="size-4 text-foreground-icon-secondary" />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">{label}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{hint}</span>
      </span>
    </>
  );
}

export default function MessageWriter(props: MessageWriterProps) {
  const { editor, slash, braces, images } = useMessageEditor(props);

  if (!editor) {
    return null;
  }

  return (
    <div className="article-brand relative">
      <SelectionBar editor={editor} />
      <ImageBar editor={editor} onReplace={() => images.pickReplacement(editor)} />
      <BlockPlus editor={editor} />
      <SuggestionList
        label="Blocks"
        suggested={slash}
        keyOf={(command) => command.id}
        render={(command) => <Option label={command.label} hint={command.hint} icon={command.icon} />}
      />
      <SuggestionList
        label="Variables"
        suggested={braces}
        keyOf={(variable) => variable.key}
        render={(variable) => <Option label={variable.label} hint={`e.g. ${variable.sample}`} icon={RiBracesLine} />}
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
