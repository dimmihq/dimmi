/*
 * The writer for a WhatsApp template's body: the broadcast writer's
 * variables, each a chip picked from the Variable menu or by typing two
 * braces, in a document that holds only what a template takes:
 * paragraphs, bold, italic, strikethrough and monospace. Lists, quotes,
 * headings, links and pictures are not in it, so pasting cannot bring
 * them in. A detail that is not the customer's, such as an order number,
 * is a custom value added with a name and an example.
 */
import { RiAddLine, RiBracesLine } from "@remixicon/react";
import { Markdown } from "@tiptap/markdown";
import { EditorContent, Extension, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { useEffect, useId, useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { VariableToken } from "@/features/broadcasts/writer/variable-token";
import { variableOf, variablePattern, variables } from "@/features/broadcasts/writer/variables";
import { syncContent, whenReady } from "@/features/inbox/composer/editor-helpers";
import { Toolbar } from "@/features/inbox/composer/rich-editor";
import { type Suggested, suggestionExtension } from "@/features/inbox/composer/suggestion-extension";
import { SuggestionList } from "@/features/inbox/composer/suggestion-list";
import { contextValues } from "@/features/saved-replies/render";
import { VariableMenu } from "@/features/saved-replies/variable-menu";

import { type CustomValue, customKey } from "./whatsapp-template-draft";

export interface TemplateWriterProps {
  value: string;
  label: string;
  placeholder: string;
  customs: CustomValue[];
  onChange: (markdown: string) => void;
  onCustoms: (customs: CustomValue[]) => void;
}

const known = Object.keys(contextValues({}));
const offered = variables.filter((variable) => known.includes(variable.key));

const formats = ["Bold", "Italic", "Strikethrough", "Inline code"];

const writerClass = [
  "flex max-h-[280px] min-h-24 flex-col gap-2 overflow-y-auto text-body-regular text-text-primary outline-none",
  "[&_code]:rounded-md [&_code]:bg-background-primary-default [&_code]:px-1 [&_code]:font-mono",
].join(" ");

/*
 * Choice is one line of the list two braces open: a detail to put in,
 * or the way to add a custom value.
 */
interface Choice {
  key: string;
  label: string;
  hint: string;
}

const addChoice: Choice = { key: "", label: "Custom value…", hint: "Filled in by hand, such as an order number" };

function choices(query: string, customs: CustomValue[]): Choice[] {
  const needle = query.trim().toLowerCase();
  const all: Choice[] = [
    ...offered.map((v) => ({ key: v.key, label: v.label, hint: `e.g. ${v.sample}` })),
    ...customs.map((c) => ({ key: c.key, label: c.label, hint: `e.g. ${c.sample}` })),
    addChoice,
  ];

  return all.filter((c) => needle === "" || c.label.toLowerCase().includes(needle));
}

const strikeKey = Extension.create({
  name: "templateKeys",
  addKeyboardShortcuts() {
    return { "Mod-Shift-x": () => this.editor.commands.toggleStrike() };
  },
});

function useTemplateEditor(props: TemplateWriterProps, onAdd: () => void) {
  const [braces, setBraces] = useState<Suggested<Choice> | null>(null);
  const now = useRef({ ...props, onAdd });
  now.current = { ...props, onAdd };

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
      }),
      Markdown,
      strikeKey,
      VariableToken.configure({
        named: (key) => variableOf(key)?.label ?? now.current.customs.find((c) => c.key === key)?.label,
        byHand: (key) => !variableOf(key),
      }),
      suggestionExtension<Choice>({
        name: "variableMenu",
        char: "{{",
        minLength: 0,
        find: (query) => choices(query, now.current.customs),
        content: () => "",
        apply: (e, range, choice) => {
          if (choice.key === "") {
            e.chain().focus().deleteRange(range).run();
            now.current.onAdd();
            return;
          }
          e.chain().focus().deleteRange(range).insertVariable(choice.key).run();
        },
        onChange: setBraces,
      }),
    ],
    content: props.value,
    contentType: "markdown",
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": props.label,
        class: writerClass,
      },
    },
    onUpdate: ({ editor: e }) => now.current.onChange(e.getMarkdown()),
  });

  useEffect(() => whenReady(editor, (ready) => syncContent(ready, props.value)), [editor, props.value]);

  return { editor, braces, now };
}

export default function TemplateWriter(props: TemplateWriterProps) {
  const [adding, setAdding] = useState(false);
  const { editor, braces, now } = useTemplateEditor(props, () => setAdding(true));

  if (!editor) {
    return null;
  }

  const insert = (key: string) => editor.chain().focus().insertVariable(key).run();

  return (
    <div className="flex flex-col gap-2">
      <div className="relative">
        {editor.isEmpty ? (
          <span
            aria-hidden
            className="pointer-events-none absolute inset-x-0 top-0 text-body-regular text-text-tertiary"
          >
            {props.placeholder}
          </span>
        ) : null}
        <EditorContent editor={editor} />
      </div>
      <SuggestionList
        label="Variables"
        suggested={braces}
        keyOf={(choice) => choice.key || "add"}
        render={(choice) => (
          <>
            {choice.key === "" ? (
              <RiAddLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
            ) : (
              <RiBracesLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
            )}
            <span className="flex min-w-0 flex-col">
              <span className="truncate text-body-medium text-text-primary">{choice.label}</span>
              <span className="truncate text-body-2-regular text-text-tertiary">{choice.hint}</span>
            </span>
          </>
        )}
      />
      <div className="flex flex-wrap items-center gap-1 border-separator-border border-t pt-2">
        <Toolbar editor={editor} only={formats} />
        <span className="ms-auto">
          <VariableMenu
            extra={props.customs}
            onPick={(markdown) => insert(variablePattern.exec(markdown)?.[1] ?? "")}
            onAdd={() => setAdding(true)}
          />
        </span>
      </div>
      {adding ? (
        <CustomValueDialog
          taken={props.customs.map((c) => c.key)}
          onClose={() => setAdding(false)}
          onAdd={(custom) => {
            const customs = [...props.customs, custom];
            now.current = { ...now.current, customs };
            props.onCustoms(customs);
            setAdding(false);
            insert(custom.key);
          }}
        />
      ) : null}
    </div>
  );
}

/*
 * CustomValueDialog adds a detail teammates fill in by hand when they
 * send the template.
 */
function CustomValueDialog({
  taken,
  onAdd,
  onClose,
}: {
  taken: string[];
  onAdd: (custom: CustomValue) => void;
  onClose: () => void;
}) {
  const formId = useId();
  const [label, setLabel] = useState("");
  const [sample, setSample] = useState("");
  const ready = label.trim() !== "" && sample.trim() !== "";

  return (
    <Modal size="sm" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title="Custom value"
        description="Teammates fill it in each time they send the template, such as an order number."
      />
      <ModalBody>
        <form
          id={formId}
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (ready) {
              onAdd({ key: customKey(label, taken), label: label.trim(), sample: sample.trim() });
            }
          }}
        >
          <Input label="Name" placeholder="Order number" autoFocus value={label} onChange={setLabel} />
          <Input
            label="Example"
            hint="Meta reviews the template with it. Customers never see it."
            placeholder="#4521"
            value={sample}
            onChange={setSample}
          />
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" form={formId} disabled={!ready}>
          Add to message
        </Button>
      </ModalFooter>
    </Modal>
  );
}
