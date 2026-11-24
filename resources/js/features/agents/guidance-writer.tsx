/*
 * The writer for an agent's guidance: the article writer the help
 * center and broadcasts use, with headings, lists, the bar over
 * selected text, the plus beside an empty line and the slash for
 * blocks, plus the @ chips procedures use, so guidance can name a
 * procedure, an article, an action, a customer detail or a team. The
 * guidance is Markdown with each chip kept as [[kind:ref]].
 */
import { EditorContent, useEditor } from "@tiptap/react";
import { useRef, useState } from "react";

import { articleExtensions, articleMarkdown } from "@/features/help-center/writer/article-markdown";
import { findCommands, runCommand, type SlashCommand } from "@/features/help-center/writer/slash-commands";
import { BlockPlus, SelectionBar } from "@/features/help-center/writer/writer-menus";
import { type Suggested, suggestionExtension } from "@/features/inbox/composer/suggestion-extension";
import { SuggestionList } from "@/features/inbox/composer/suggestion-list";
import type { ProcedureStep } from "@/lib/api/procedures";

import {
  type ChipOption,
  type ChipSources,
  findChips,
  labelOf,
} from "../automation/procedure-writer/procedure-suggestions";
import { ChipContext } from "../automation/procedure-writer/procedure-token";
import { GuidanceToken } from "./guidance-token";
import { ChipOptionRow, SlashOption } from "./writer-options";

/*
 * Guidance is a prompt, not an article: it takes the blocks a model
 * reads as structure, and none of the help center's callouts, images or
 * tables.
 */
const promptBlocks = new Set(["text", "heading", "subheading", "bullets", "numbers", "code"]);

const findBlocks = (query: string) => findCommands(query).filter((command) => promptBlocks.has(command.id));

/* Guidance is no step; chips read this stand-in for settings they never show here. */
const guidanceStep = { id: "guidance", kind: "instruction", text: "" } as ProcedureStep;

export function GuidanceWriter({
  markdown,
  sources,
  readOnly = false,
  onChange,
}: {
  markdown: string;
  sources: ChipSources;
  readOnly?: boolean;
  onChange: (markdown: string) => void;
}) {
  const [slash, setSlash] = useState<Suggested<SlashCommand> | null>(null);
  const [chips, setChips] = useState<Suggested<ChipOption> | null>(null);
  const changed = useRef(onChange);
  changed.current = onChange;
  const known = useRef(sources);
  known.current = sources;

  const editor = useEditor({
    editable: !readOnly,
    extensions: articleExtensions([
      GuidanceToken.configure({ label: (chip) => labelOf(known.current, chip) }),
      suggestionExtension<SlashCommand>({
        name: "slashCommand",
        char: "/",
        minLength: 0,
        find: findBlocks,
        content: () => "",
        apply: (e, range, command) => runCommand(e, range, command, { pickImage: () => undefined }),
        onChange: setSlash,
      }),
      suggestionExtension<ChipOption>({
        name: "guidanceChips",
        char: "@",
        minLength: 0,
        find: (query) => findChips(known.current, query),
        content: () => "",
        apply: (e, range, option) => e.chain().focus().deleteRange(range).insertChip(option, option.label).run(),
        onChange: setChips,
      }),
    ]),
    content: markdown,
    contentType: "markdown",
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": "Guidance",
        class: "prose prose-compact min-h-48 pb-4 outline-none",
      },
    },
    onUpdate: ({ editor: e }) => changed.current(articleMarkdown(e.getMarkdown())),
  });

  if (!editor) {
    return null;
  }

  return (
    <ChipContext.Provider value={{ sources, step: guidanceStep, earlier: [] }}>
      <div className="article-brand relative">
        <SelectionBar editor={editor} />
        <BlockPlus editor={editor} />
        <SuggestionList
          label="Blocks"
          suggested={slash}
          keyOf={(command) => command.id}
          render={(command) => <SlashOption command={command} />}
        />
        <SuggestionList
          label="Insert"
          suggested={chips}
          keyOf={(option) => `${option.kind}:${option.ref}`}
          render={(option) => <ChipOptionRow option={option} />}
        />
        <EditorContent editor={editor} />
      </div>
    </ChipContext.Provider>
  );
}
