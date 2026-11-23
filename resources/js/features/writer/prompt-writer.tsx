/*
 * The writer for instructions a model follows: the article writer the
 * help center and broadcasts use, with headings, lists, the bar over
 * selected text, the plus beside an empty line and the slash for
 * blocks, plus an @ menu of chips the caller brings, such as the tools a
 * routine may call. Instructions are Markdown; each chip keeps its own
 * marker in it. An agent's guidance and a routine's prompt both write
 * here.
 */
import { type AnyExtension, type Editor, EditorContent, type Range, useEditor } from "@tiptap/react";
import { type ReactNode, useRef, useState } from "react";

import { articleExtensions } from "@/features/help-center/writer/article-markdown";
import { findCommands, runCommand, type SlashCommand } from "@/features/help-center/writer/slash-commands";
import { BlockPlus, SelectionBar } from "@/features/help-center/writer/writer-menus";
import { type Suggested, suggestionExtension } from "@/features/inbox/composer/suggestion-extension";
import { SuggestionList } from "@/features/inbox/composer/suggestion-list";
import { cx } from "@/utils/cx";

import { promptMarkdown } from "./prompt-markdown";

/*
 * Instructions are a prompt, not an article: they take the blocks a
 * model reads as structure, and none of the help center's callouts,
 * images or tables.
 */
const promptBlocks = new Set(["text", "heading", "subheading", "bullets", "numbers", "code"]);

const findBlocks = (query: string) => findCommands(query).filter((command) => promptBlocks.has(command.id));

/*
 * PromptChips are what the @ menu offers: the chip's node, the options
 * matching what was typed, how to put one in, and how a row reads.
 */
export interface PromptChips<T> {
  node: AnyExtension;
  label: string;
  find: (query: string) => T[];
  insert: (editor: Editor, range: Range, option: T) => void;
  keyOf: (option: T) => string;
  render: (option: T) => ReactNode;
}

function SlashOption({ command }: { command: SlashCommand }) {
  const Icon = command.icon;

  return (
    <>
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border-button-default bg-background-primary-default">
        <Icon aria-hidden className="size-4 text-foreground-icon-secondary" />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">{command.label}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{command.hint}</span>
      </span>
    </>
  );
}

export function PromptWriter<T>({
  markdown,
  label,
  chips,
  readOnly = false,
  className,
  onChange,
}: {
  markdown: string;
  label: string;
  chips: PromptChips<T>;
  readOnly?: boolean;
  className?: string;
  onChange: (markdown: string) => void;
}) {
  const [slash, setSlash] = useState<Suggested<SlashCommand> | null>(null);
  const [offered, setOffered] = useState<Suggested<T> | null>(null);
  const changed = useRef(onChange);
  changed.current = onChange;
  const known = useRef(chips);
  known.current = chips;
  /* loaded is the Markdown as the writer first read it, so reading it in is never taken for an edit. */
  const loaded = useRef<string | null>(null);
  const initial = useRef(markdown);

  const editor = useEditor({
    editable: !readOnly,
    extensions: articleExtensions([
      chips.node,
      suggestionExtension<SlashCommand>({
        name: "slashCommand",
        char: "/",
        minLength: 0,
        find: findBlocks,
        content: () => "",
        apply: (e, range, command) => runCommand(e, range, command, { pickImage: () => undefined }),
        onChange: setSlash,
      }),
      suggestionExtension<T>({
        name: "promptChips",
        char: "@",
        minLength: 0,
        find: (query) => known.current.find(query),
        content: () => "",
        apply: (e, range, option) => known.current.insert(e, range, option),
        onChange: setOffered,
      }),
    ]),
    content: markdown,
    contentType: "markdown",
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": label,
        class: cx("prose min-h-48 pb-4 outline-none", className),
      },
    },
    onCreate: ({ editor: e }) => {
      loaded.current = promptMarkdown(e.getMarkdown());
    },
    onUpdate: ({ editor: e }) => {
      const next = promptMarkdown(e.getMarkdown());
      changed.current(next === loaded.current ? initial.current : next);
    },
  });

  if (!editor) {
    return null;
  }

  return (
    <div className="article-brand relative">
      <SelectionBar editor={editor} />
      <BlockPlus editor={editor} />
      <SuggestionList
        label="Blocks"
        suggested={slash}
        keyOf={(command) => command.id}
        render={(command) => <SlashOption command={command} />}
      />
      <SuggestionList label={chips.label} suggested={offered} keyOf={chips.keyOf} render={chips.render} />
      <EditorContent editor={editor} />
    </div>
  );
}
