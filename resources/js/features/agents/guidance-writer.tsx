/*
 * The writer for an agent's guidance: the article writer the help
 * center and broadcasts use, with headings, lists, the bar over
 * selected text, the plus beside an empty line and the slash for
 * blocks, plus the @ chips procedures use, so guidance can name a
 * procedure, an article, an action, a customer detail or a team.
 * Mockup: the words live only in this page.
 */
import { EditorContent, useEditor } from "@tiptap/react";
import { useState } from "react";

import { articleExtensions } from "@/features/help-center/writer/article-markdown";
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
import { ChipContext, ProcedureToken } from "../automation/procedure-writer/procedure-token";
import { ChipOptionRow, SlashOption } from "./writer-options";

/* Guidance is no step; chips read this stand-in for settings they never show here. */
const guidanceStep = { id: "guidance", kind: "instruction", text: "" } as ProcedureStep;

export function GuidanceWriter({ html, sources }: { html: string; sources: ChipSources }) {
  const [slash, setSlash] = useState<Suggested<SlashCommand> | null>(null);
  const [chips, setChips] = useState<Suggested<ChipOption> | null>(null);

  const editor = useEditor({
    extensions: articleExtensions([
      ProcedureToken.configure({ label: (chip) => labelOf(sources, chip) }),
      suggestionExtension<SlashCommand>({
        name: "slashCommand",
        char: "/",
        minLength: 0,
        find: findCommands,
        content: () => "",
        apply: (e, range, command) => runCommand(e, range, command, { pickImage: () => undefined }),
        onChange: setSlash,
      }),
      suggestionExtension<ChipOption>({
        name: "guidanceChips",
        char: "@",
        minLength: 0,
        find: (query) => findChips(sources, query),
        content: () => "",
        apply: (e, range, option) => e.chain().focus().deleteRange(range).insertChip(option, option.label).run(),
        onChange: setChips,
      }),
    ]),
    content: html,
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": "Guidance",
        class: "prose min-h-64 pb-6 outline-none",
      },
    },
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
