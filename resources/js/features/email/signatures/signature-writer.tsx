/*
 * The writer for a signature: the message writer cut down to what a
 * signature carries. A bar above the text sets bold, italic and links
 * and inserts a detail, and typing a slash offers the details too, each
 * kept as a chip that names it. Enter starts a new line, as a signature
 * is written line by line, and twice leaves a gap. Whatever is written
 * comes out in the form signatures are stored in.
 */
import { type RemixiconComponentType, RiBold, RiBracesLine, RiItalic, RiLink } from "@remixicon/react";
import { type Editor, EditorContent, Extension, useEditor, useEditorState } from "@tiptap/react";
import { useRef, useState } from "react";

import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { applyLink } from "@/features/inbox/composer/editor-helpers";
import { LinkRow } from "@/features/inbox/composer/link-row";
import { type Suggested, suggestionExtension } from "@/features/inbox/composer/suggestion-extension";
import { SuggestionList } from "@/features/inbox/composer/suggestion-list";
import { cx } from "@/utils/cx";

import { signatureExtensions, signatureMarkdown } from "./signature-markdown";
import { findSignatureVariables, type SignatureVariable, signatureVariables, toWriter } from "./signature-variables";

export interface SignatureWriterProps {
  value: string;
  label: string;
  onChange: (body: string) => void;
}

/*
 * SignatureLines makes Enter a line break, the way a signature's lines
 * are stored. It runs before the editor's own Enter, and after the
 * slash menu, which takes Enter first while it is open.
 */
const SignatureLines = Extension.create({
  name: "signatureLines",
  priority: 150,
  addKeyboardShortcuts() {
    return { Enter: ({ editor }) => editor.commands.setHardBreak() };
  },
});

function useSignatureEditor({ value, label, onChange }: SignatureWriterProps) {
  const [slash, setSlash] = useState<Suggested<SignatureVariable> | null>(null);
  const changed = useRef(onChange);
  changed.current = onChange;

  const editor = useEditor({
    extensions: signatureExtensions([
      suggestionExtension<SignatureVariable>({
        name: "signatureVariableMenu",
        char: "/",
        minLength: 0,
        find: findSignatureVariables,
        content: () => "",
        apply: (e, range, variable) => e.chain().focus().deleteRange(range).insertSignatureVariable(variable.key).run(),
        onChange: setSlash,
      }).extend({ priority: 200 }),
      SignatureLines,
    ]),
    content: toWriter(value),
    contentType: "markdown",
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": label,
        class: "prose min-h-40 px-4 py-3 outline-none",
      },
    },
    onUpdate: ({ editor: e }) => changed.current(signatureMarkdown(e)),
  });

  return { editor, slash };
}

const toolClass = cx(
  "flex size-8 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none",
  "transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
);

function Tool({
  label,
  icon: Icon,
  active,
  onPress,
}: {
  label: string;
  icon: RemixiconComponentType;
  active: boolean;
  onPress: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={active}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onPress}
      className={cx(toolClass, active && "bg-background-tertiary-default text-foreground-icon-primary")}
    >
      <Icon aria-hidden className="size-4" />
    </button>
  );
}

function VariableMenu({ editor }: { editor: Editor }) {
  const [open, setOpen] = useState(false);

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        className={cx(toolClass, "w-auto gap-1.5 px-2 text-body-2-medium text-text-secondary")}
        aria-label="Insert a detail"
      >
        <RiBracesLine aria-hidden className="size-4" />
        Detail
      </DropdownTrigger>
      <DropdownPopover aria-label="Details" placement="bottom start">
        {signatureVariables.map((variable) => (
          <DropdownItem
            key={variable.key}
            onSelect={() => {
              setOpen(false);
              editor.chain().focus().insertSignatureVariable(variable.key).run();
            }}
          >
            <span className="flex min-w-0 flex-col">
              <span className="truncate text-body-2-medium text-text-primary">{variable.label}</span>
              <span className="truncate text-body-2-regular text-text-tertiary">{variable.description}</span>
            </span>
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
  );
}

/*
 * Toolbar is the bar above the text, or the link's address while one is
 * being set.
 */
function Toolbar({ editor }: { editor: Editor }) {
  const [linking, setLinking] = useState<string | null>(null);
  const active = useEditorState({
    editor,
    selector: ({ editor: e }) => ({ bold: e.isActive("bold"), italic: e.isActive("italic"), link: e.isActive("link") }),
  });

  if (linking !== null) {
    return (
      <div className="border-separator-border border-b p-1.5">
        <LinkRow
          initial={linking}
          onApply={(href) => {
            applyLink(editor, href);
            setLinking(null);
          }}
          onCancel={() => {
            setLinking(null);
            editor.commands.focus();
          }}
        />
      </div>
    );
  }

  return (
    <div
      role="toolbar"
      aria-label="Formatting"
      className="flex items-center gap-0.5 border-separator-border border-b p-1"
    >
      <Tool label="Bold" icon={RiBold} active={active.bold} onPress={() => editor.chain().focus().toggleBold().run()} />
      <Tool
        label="Italic"
        icon={RiItalic}
        active={active.italic}
        onPress={() => editor.chain().focus().toggleItalic().run()}
      />
      <Tool
        label="Link"
        icon={RiLink}
        active={active.link}
        onPress={() => setLinking((editor.getAttributes("link").href as string | undefined) ?? "https://")}
      />
      <span aria-hidden className="mx-0.5 h-5 w-px bg-separator-border" />
      <VariableMenu editor={editor} />
    </div>
  );
}

function Option({ variable }: { variable: SignatureVariable }) {
  return (
    <>
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border-button-default bg-background-primary-default">
        <RiBracesLine aria-hidden className="size-4 text-foreground-icon-secondary" />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">{variable.label}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{variable.description}</span>
      </span>
    </>
  );
}

export function SignatureWriter(props: SignatureWriterProps) {
  const { editor, slash } = useSignatureEditor(props);

  if (!editor) {
    return null;
  }

  return (
    <div className="flex flex-col overflow-hidden rounded-2xl border border-border-button-default bg-background-primary-default">
      <Toolbar editor={editor} />
      <SuggestionList
        label="Details"
        suggested={slash}
        keyOf={(variable) => variable.key}
        render={(variable) => <Option variable={variable} />}
      />
      <EditorContent editor={editor} />
    </div>
  );
}
