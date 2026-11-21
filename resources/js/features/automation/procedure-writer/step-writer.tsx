/*
 * The writer of one procedure step: plain words, with @ for chips. The
 * list groups what a chip can name and moves with the arrow keys, Enter
 * or Tab takes one and Escape lets it go. Every edit is written back as
 * text with chip markers, and the step's bindings follow the chips, so
 * a chip's settings survive any rewording around it.
 */
import {
  RiArticleLine,
  RiFlashlightLine,
  RiListOrdered2,
  RiPriceTag3Line,
  RiTeamLine,
  RiTicketLine,
  RiUserLine,
} from "@remixicon/react";
import { Placeholder } from "@tiptap/extensions";
import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { useEffect, useRef, useState } from "react";

import { type Suggested, suggestionExtension } from "@/features/inbox/composer/suggestion-extension";
import { SuggestionList } from "@/features/inbox/composer/suggestion-list";
import type { BindingKind, ProcedureBinding, ProcedureStep } from "@/lib/api/procedures";
import { type ChipOption, type ChipSources, findChips, labelOf } from "./procedure-suggestions";
import { ProcedureToken } from "./procedure-token";
import { type Chip, type DocNode, docToText, syncBindings, textToDoc } from "./tokens";

const icons: Record<BindingKind, typeof RiFlashlightLine> = {
  action: RiFlashlightLine,
  attribute: RiUserLine,
  article: RiArticleLine,
  handoff: RiTeamLine,
  tag: RiPriceTag3Line,
  ticket_type: RiTicketLine,
  sub_procedure: RiListOrdered2,
};

export interface StepWriterProps {
  step: ProcedureStep;
  label: string;
  placeholder: string;
  sources: ChipSources;
  earlier: (readonly [string, string])[];
  readOnly?: boolean;
  onChange: (change: Partial<ProcedureStep>) => void;
}

function Option({ option }: { option: ChipOption }) {
  const Icon = icons[option.kind];

  return (
    <span className="flex min-w-0 flex-1 flex-col">
      {option.first ? (
        <span className="pb-1 text-caption-1-semibold text-text-tertiary uppercase">{option.group}</span>
      ) : null}
      <span className="flex min-w-0 items-center gap-2">
        <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-body-medium text-text-primary">{option.label}</span>
          <span className="truncate text-body-2-regular text-text-tertiary">{option.hint}</span>
        </span>
      </span>
    </span>
  );
}

export function StepWriter(props: StepWriterProps) {
  const [chips, setChips] = useState<Suggested<ChipOption> | null>(null);
  const now = useRef(props);
  now.current = props;
  const label = (chip: Chip) => labelOf(now.current.sources, chip);
  const binding = (chip: Chip) => now.current.step.bindings?.find((b) => b.kind === chip.kind && b.ref === chip.ref);

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: false,
        horizontalRule: false,
        underline: false,
        link: false,
        bulletList: false,
        orderedList: false,
        listItem: false,
        listKeymap: false,
        blockquote: false,
        codeBlock: false,
        bold: false,
        italic: false,
        strike: false,
        code: false,
      }),
      Placeholder.configure({ placeholder: props.placeholder }),
      ProcedureToken.configure({
        label,
        binding,
        onBinding: (chip, change) => {
          const step = now.current.step;
          const bindings = (step.bindings ?? []).map((b: ProcedureBinding) =>
            b.kind === chip.kind && b.ref === chip.ref ? { ...b, ...change } : b,
          );
          now.current.onChange({ bindings });
        },
        requires: () => now.current.step.enforcement?.requires ?? [],
        onRequires: (ids) =>
          now.current.onChange({
            enforcement: { ...now.current.step.enforcement, requires: ids.length > 0 ? ids : undefined },
          }),
        earlier: () => now.current.earlier,
      }),
      suggestionExtension<ChipOption>({
        name: "procedureChips",
        char: "@",
        minLength: 0,
        find: (query) => findChips(now.current.sources, query),
        content: () => "",
        apply: (e, range, option) => e.chain().focus().deleteRange(range).insertChip(option, option.label).run(),
        onChange: setChips,
      }),
    ],
    content: textToDoc(props.step.text, label),
    editable: !props.readOnly,
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": props.label,
        class: "min-h-8 py-1.5 text-body-regular text-text-primary outline-none",
      },
    },
    onUpdate: ({ editor: e }) => {
      const text = docToText(e.getJSON() as DocNode);
      now.current.onChange({ text, bindings: syncBindings(now.current.step, text, label) });
    },
  });

  useEffect(() => {
    editor?.setEditable(!props.readOnly);
  }, [editor, props.readOnly]);

  if (!editor) {
    return null;
  }

  return (
    <div className="relative rounded-2lg bg-background-tertiary-default px-3 ring-1 ring-transparent focus-within:ring-border-button-active">
      <EditorContent editor={editor} />
      <SuggestionList
        label="Insert"
        suggested={chips}
        keyOf={(option) => `${option.kind}:${option.ref}`}
        render={(option) => <Option option={option} />}
      />
    </div>
  );
}
