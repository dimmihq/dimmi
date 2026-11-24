/*
 * The writer for a routine's instructions: plain words, as anyone would
 * ask a teammate, with @ to name something of the workspace. Typing
 * nothing but words works exactly as before; a mention is only a way to
 * point at a team, a teammate, a tag and the like so that renaming it
 * never breaks the routine.
 */
import { EditorContent, useEditor } from "@tiptap/react";
import { useEffect, useRef, useState } from "react";

import { articleExtensions, articleMarkdown } from "@/features/help-center/writer/article-markdown";
import { type Suggested, suggestionExtension } from "@/features/inbox/composer/suggestion-extension";
import { SuggestionList } from "@/features/inbox/composer/suggestion-list";

import { MentionChip } from "./mention-chip";
import { MentionContext } from "./mention-context";
import { MentionMark } from "./mention-mark";
import { findMentions, type Mentionables, type MentionOption, nameOf } from "./mentions";

function MentionRow({ option }: { option: MentionOption }) {
  return (
    <span className="flex min-w-0 flex-1 flex-col">
      {option.first ? (
        <span className="pb-1.5 text-caption-1-semibold text-text-tertiary uppercase">{option.group}</span>
      ) : null}
      <span className="flex min-w-0 items-center gap-2.5">
        <MentionMark kind={option.kind} target={option.target} />
        <span className="truncate text-body-medium text-text-primary">{option.label}</span>
      </span>
    </span>
  );
}

export function RoutineWriter({
  markdown,
  known,
  label,
  placeholder,
  readOnly = false,
  onChange,
}: {
  markdown: string;
  known: Mentionables;
  label: string;
  placeholder: string;
  readOnly?: boolean;
  onChange: (markdown: string) => void;
}) {
  const [mentions, setMentions] = useState<Suggested<MentionOption> | null>(null);
  const changed = useRef(onChange);
  changed.current = onChange;
  const latest = useRef(known);
  latest.current = known;

  const editor = useEditor({
    editable: !readOnly,
    extensions: articleExtensions([
      MentionChip.configure({ name: (kind, ref) => nameOf(latest.current, kind, ref) }),
      suggestionExtension<MentionOption>({
        name: "routineMentions",
        char: "@",
        minLength: 0,
        find: (query) => findMentions(latest.current, query),
        content: () => "",
        apply: (e, range, option) =>
          e
            .chain()
            .focus()
            .deleteRange(range)
            .insertContent([
              { type: "routineMention", attrs: { kind: option.kind, ref: option.ref } },
              { type: "text", text: " " },
            ])
            .run(),
        onChange: setMentions,
      }),
    ]),
    content: markdown,
    contentType: "markdown",
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": label,
        "data-placeholder": placeholder,
        class: "prose prose-compact min-h-20 outline-none",
      },
    },
    onUpdate: ({ editor: e }) => changed.current(articleMarkdown(e.getMarkdown()).trim()),
  });

  // Someone else may set the instructions, such as Discard: show what they are now.
  useEffect(() => {
    if (editor && articleMarkdown(editor.getMarkdown()).trim() !== markdown.trim()) {
      editor.commands.setContent(markdown, { contentType: "markdown", emitUpdate: false });
    }
  }, [editor, markdown]);

  useEffect(() => {
    editor?.setEditable(!readOnly);
  }, [editor, readOnly]);

  // Names arrive after the editor is drawn: draw the chips again with them.
  const loaded = Object.values(known).reduce((sum, list) => sum + list.length, 0);
  useEffect(() => {
    if (editor && loaded > 0) {
      editor.view.dispatch(editor.state.tr.setMeta("routineMentionNames", loaded));
    }
  }, [editor, loaded]);

  if (!editor) {
    return null;
  }

  return (
    <div className="relative rounded-xl bg-background-tertiary-default px-3 py-2.5">
      <SuggestionList
        label="Mention"
        suggested={mentions}
        keyOf={(option) => `${option.kind}:${option.ref}`}
        render={(option) => <MentionRow option={option} />}
      />
      <MentionContext.Provider value={known}>
        <EditorContent editor={editor} />
      </MentionContext.Provider>
    </div>
  );
}
