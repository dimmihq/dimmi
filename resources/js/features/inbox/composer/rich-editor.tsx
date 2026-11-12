/*
 * The composer's formatting editor, loaded on its own so the inbox opens
 * without it. People write with the toolbar, Markdown shortcuts as they
 * type, or Cmd+B, Cmd+I, Cmd+Shift+X and Cmd+Shift+U; what they see is
 * kept as Markdown, the only thing the composer reads and saves. Pasted
 * pages keep only what the composer's document holds, so headings,
 * colours and underline fall away to plain text. A colon and two letters
 * suggest emoji by name; in a note, an at sign suggests teammates and
 * teams to mention. A hash and a letter or more suggest saved replies,
 * whose text takes the place of what was typed.
 */
import type { RemixiconComponentType } from "@remixicon/react";
import {
  RiBold,
  RiCodeLine,
  RiCodeSSlashLine,
  RiDoubleQuotesL,
  RiItalic,
  RiLink,
  RiListOrdered,
  RiListUnordered,
  RiStrikethrough,
} from "@remixicon/react";
import { type Editor, EditorContent, useEditor, useEditorState } from "@tiptap/react";
import { useEffect, useRef, useState } from "react";

import type { SavedReply } from "@/lib/api/saved-replies";
import { cx } from "@/utils/cx";

import { composerExtensions } from "./composer-markdown";
import { applyLink, syncContent, whenReady } from "./editor-helpers";
import { type Emoji, findEmoji, loadEmoji } from "./emoji";
import { LinkRow } from "./link-row";
import type { Mentionable } from "./mentionable";
import { type Suggested, suggestionExtension } from "./suggestion-extension";
import { SuggestionList } from "./suggestion-list";

/*
 * richClass styles the editable document like a sent message reads.
 */
const richClass = [
  "flex max-h-[280px] min-h-24 flex-col gap-2 overflow-y-auto text-body-regular text-text-primary outline-none",
  "[&_ul]:list-disc [&_ul]:ps-5 [&_ol]:list-decimal [&_ol]:ps-5 [&_a]:underline",
  "[&_blockquote]:border-s-2 [&_blockquote]:border-separator-border [&_blockquote]:ps-3",
  "[&_blockquote]:text-text-secondary [&_code]:rounded-md [&_code]:bg-background-secondary-default",
  "[&_code]:px-1 [&_code]:font-mono [&_pre]:rounded-xl [&_pre]:bg-background-secondary-default",
  "[&_pre]:p-2 [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_img]:inline [&_img]:max-h-40",
].join(" ");

export interface RichEditorProps {
  value: string;
  label: string;
  placeholder: string;
  focusKey: string;
  onChange: (markdown: string) => void;
  onSubmit: (then?: "close" | "snooze") => void;
  onBlur: () => void;
  onFiles: (files: FileList) => void;
  findMentions?: (query: string) => Promise<Mentionable[]>;
  /* Saved replies whose title or shortcut holds what follows a #. */
  findSavedReplies?: (query: string) => Promise<SavedReply[]>;
  /* The Markdown a picked saved reply puts in place of what was typed. */
  takeSavedReply?: (reply: SavedReply) => string;
}

/*
 * submitKey reads the send shortcuts: Cmd+Enter sends, with Shift it
 * also closes and with Alt it snoozes.
 */
function submitKey(event: KeyboardEvent): "send" | "close" | "snooze" | null {
  if (event.key !== "Enter" || !(event.metaKey || event.ctrlKey)) {
    return null;
  }
  if (event.shiftKey) {
    return "close";
  }

  return event.altKey ? "snooze" : "send";
}

export default function RichEditor({
  value,
  label,
  placeholder,
  focusKey,
  onChange,
  onSubmit,
  onBlur,
  onFiles,
  findMentions,
  findSavedReplies,
  takeSavedReply,
}: RichEditorProps) {
  const handlers = useRef({ onChange, onSubmit, onBlur, onFiles, findMentions, findSavedReplies, takeSavedReply });
  handlers.current = { onChange, onSubmit, onBlur, onFiles, findMentions, findSavedReplies, takeSavedReply };
  const linkRef = useRef<() => void>(() => undefined);
  const [linking, setLinking] = useState<string | null>(null);
  const [emoji, setEmoji] = useState<Suggested<Emoji> | null>(null);
  const [mention, setMention] = useState<Suggested<Mentionable> | null>(null);
  const [saved, setSaved] = useState<Suggested<SavedReply> | null>(null);

  const editor = useEditor({
    extensions: composerExtensions(
      () => linkRef.current(),
      [
        suggestionExtension<Emoji>({
          name: "emojiSuggestion",
          char: ":",
          minLength: 2,
          find: async (query) => findEmoji(await loadEmoji(), query, 8),
          content: (item) => item.unicode,
          onChange: setEmoji,
        }),
        suggestionExtension<Mentionable>({
          name: "mentionSuggestion",
          char: "@",
          minLength: 0,
          find: (query) => handlers.current.findMentions?.(query) ?? [],
          content: (item) => [
            { type: "mentionToken", attrs: { id: item.id, label: item.name } },
            { type: "text", text: " " },
          ],
          onChange: setMention,
        }),
        suggestionExtension<SavedReply>({
          name: "savedReplySuggestion",
          char: "#",
          minLength: 1,
          find: (query) => handlers.current.findSavedReplies?.(query) ?? [],
          content: () => "",
          apply: (editor, range, reply) => {
            const markdown = handlers.current.takeSavedReply?.(reply) ?? reply.body;
            editor.chain().focus().insertContentAt(range, markdown, { contentType: "markdown" }).run();
          },
          onChange: setSaved,
        }),
      ],
    ),
    content: value,
    contentType: "markdown",
    autofocus: "end",
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-label": label,
        class: richClass,
      },
      handleKeyDown: (_, event) => {
        const key = submitKey(event);
        if (!key) {
          return false;
        }
        event.preventDefault();
        handlers.current.onSubmit(key === "send" ? undefined : key);
        return true;
      },
      handlePaste: (_, event) => {
        const files = event.clipboardData?.files;
        if (files && files.length > 0) {
          handlers.current.onFiles(files);
          return true;
        }
        return false;
      },
    },
    onUpdate: ({ editor: e }) => handlers.current.onChange(e.getMarkdown()),
    onBlur: () => handlers.current.onBlur(),
  });

  linkRef.current = () => setLinking((editor?.getAttributes("link").href as string | undefined) ?? "https://");

  useEffect(() => whenReady(editor, (ready) => syncContent(ready, value)), [editor, value]);
  useEffect(() => whenReady(editor, (ready) => focusKey && ready.commands.focus("end")), [editor, focusKey]);

  return (
    <div className="flex flex-col gap-2">
      <div className="relative">
        {value.trim() === "" ? (
          <span
            aria-hidden
            className="pointer-events-none absolute inset-x-0 top-0 text-body-regular text-text-tertiary"
          >
            {placeholder}
          </span>
        ) : null}
        <EditorContent editor={editor} />
      </div>
      <SuggestionList
        label="Mention"
        suggested={mention}
        keyOf={(item) => item.id}
        render={(item) => (
          <span className="flex min-w-0 flex-col">
            <span className="truncate">
              @{item.name}
              {item.detail ? <span className="ms-1.5 text-text-tertiary">{item.detail}</span> : null}
            </span>
            {item.warning ? <span className="text-body-2-regular text-text-error-primary">{item.warning}</span> : null}
          </span>
        )}
      />
      <SuggestionList
        label="Saved replies"
        suggested={saved}
        keyOf={(item) => item.id}
        render={(item) => (
          <span className="flex min-w-0 flex-col">
            <span className="truncate">
              {item.title}
              {item.shortcut ? <span className="ms-1.5 text-text-tertiary">#{item.shortcut}</span> : null}
            </span>
            <span className="truncate text-body-2-regular text-text-tertiary">{item.body}</span>
          </span>
        )}
      />
      <SuggestionList
        label="Emoji"
        suggested={emoji}
        keyOf={(item) => item.unicode}
        render={(item) => (
          <>
            <span aria-hidden>{item.unicode}</span>
            <span className="truncate">{item.label}</span>
          </>
        )}
      />
      {editor ? <Toolbar editor={editor} onLink={() => linkRef.current()} /> : null}
      {editor && linking !== null ? (
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
      ) : null}
    </div>
  );
}

interface Tool {
  label: string;
  icon: RemixiconComponentType;
  active: (editor: Editor) => boolean;
  run: (editor: Editor, link: () => void) => void;
}

const tools: Tool[] = [
  { label: "Bold", icon: RiBold, active: (e) => e.isActive("bold"), run: (e) => e.chain().focus().toggleBold().run() },
  {
    label: "Italic",
    icon: RiItalic,
    active: (e) => e.isActive("italic"),
    run: (e) => e.chain().focus().toggleItalic().run(),
  },
  {
    label: "Strikethrough",
    icon: RiStrikethrough,
    active: (e) => e.isActive("strike"),
    run: (e) => e.chain().focus().toggleStrike().run(),
  },
  {
    label: "Inline code",
    icon: RiCodeLine,
    active: (e) => e.isActive("code"),
    run: (e) => e.chain().focus().toggleCode().run(),
  },
  { label: "Link", icon: RiLink, active: (e) => e.isActive("link"), run: (_, link) => link() },
  {
    label: "Bulleted list",
    icon: RiListUnordered,
    active: (e) => e.isActive("bulletList"),
    run: (e) => e.chain().focus().toggleBulletList().run(),
  },
  {
    label: "Numbered list",
    icon: RiListOrdered,
    active: (e) => e.isActive("orderedList"),
    run: (e) => e.chain().focus().toggleOrderedList().run(),
  },
  {
    label: "Quote",
    icon: RiDoubleQuotesL,
    active: (e) => e.isActive("blockquote"),
    run: (e) => e.chain().focus().toggleBlockquote().run(),
  },
  {
    label: "Code block",
    icon: RiCodeSSlashLine,
    active: (e) => e.isActive("codeBlock"),
    run: (e) => e.chain().focus().toggleCodeBlock().run(),
  },
];

function Toolbar({ editor, onLink }: { editor: Editor; onLink: () => void }) {
  const active = useEditorState({ editor, selector: ({ editor: e }) => tools.map((t) => t.active(e)) });

  return (
    <div role="toolbar" aria-label="Formatting" className="flex flex-wrap items-center gap-0.5">
      {tools.map((tool, index) => (
        <button
          key={tool.label}
          type="button"
          aria-label={tool.label}
          aria-pressed={active[index]}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => tool.run(editor, onLink)}
          className={cx(
            "flex size-7 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
            active[index] && "bg-background-tertiary-default text-foreground-icon-primary",
          )}
        >
          <tool.icon aria-hidden className="size-4" />
        </button>
      ))}
    </div>
  );
}
