/*
 * The composer's formatting editor, loaded on its own so the inbox opens
 * without it. People write with the toolbar, Markdown shortcuts as they
 * type, or Cmd+B, Cmd+I, Cmd+Shift+X and Cmd+Shift+U; what they see is
 * kept as Markdown, the only thing the composer reads and saves. Pasted
 * pages keep only what the composer's document holds, so headings,
 * colours and underline fall away to plain text. A colon and two letters
 * suggest emoji by name, from the first letter. An at sign searches what
 * can go into the message: in a note, teammates and teams to mention,
 * then saved replies, help articles, feature requests, changelog entries
 * and emoji. A hash and a letter or more suggest saved replies,
 * whose text takes the place of what was typed. Where the text is filled
 * in for each customer, such as a workflow's message, an open brace
 * suggests the variables, each kept as a chip. Links to help center
 * pages show as chips with their titles.
 */
import type { RemixiconComponentType } from "@remixicon/react";
import {
  RiArticleLine,
  RiBold,
  RiChatQuoteLine,
  RiCodeLine,
  RiCodeSSlashLine,
  RiDoubleQuotesL,
  RiHistoryLine,
  RiItalic,
  RiLightbulbLine,
  RiLink,
  RiListOrdered,
  RiListUnordered,
  RiStrikethrough,
  RiTeamLine,
  RiUser3Line,
} from "@remixicon/react";
import { type Editor, EditorContent, useEditor, useEditorState } from "@tiptap/react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { VariableToken } from "@/features/broadcasts/writer/variable-token";
import { variableOf } from "@/features/broadcasts/writer/variables";
import type { SavedReply } from "@/lib/api/saved-replies";
import type { WhatsAppTemplate } from "@/lib/api/whatsapp";
import { cx } from "@/utils/cx";
import type { LibraryItem } from "../library";
import { composerExtensions } from "./composer-markdown";
import { applyLink, syncContent, whenReady } from "./editor-helpers";
import { type Emoji, findEmoji, loadEmoji } from "./emoji";
import { LinkRow } from "./link-row";
import { LinkToken } from "./link-token";
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
  /* Saved replies, articles, feature requests and changelog entries that hold what follows an @. */
  findLibrary?: (query: string) => Promise<LibraryItem[]>;
  /* Saved replies whose title or shortcut holds what follows a #. */
  findSavedReplies?: (query: string) => Promise<SavedReply[]>;
  /* The Markdown a picked saved reply puts in place of what was typed. */
  takeSavedReply?: (reply: SavedReply) => string;
  /* Templates a WhatsApp reply can be sent as, whose name holds what follows a /. */
  findTemplates?: (query: string) => WhatsAppTemplate[];
  /* Takes the picked template in place of what was typed. */
  takeTemplate?: (template: WhatsAppTemplate) => void;
  /* Where the formatting buttons go, such as the composer's own toolbar row; under the text when left out. */
  toolbarSlot?: HTMLElement | null;
  /* Details filled in for each customer, offered after an open brace and kept as chips. */
  variables?: EditorVariable[];
  /* Whether the field takes the focus when it opens; it does unless told not to. */
  autoFocus?: boolean;
}

/*
 * EditorVariable is one detail a message can name: its key, as the
 * Markdown keeps it, and the name its chip shows.
 */
export interface EditorVariable {
  key: string;
  label: string;
  detail?: string;
}

/*
 * findVariable narrows the variables to those whose name or key holds
 * what was typed after the brace.
 */
function findVariable(variables: EditorVariable[], query: string): EditorVariable[] {
  const needle = query.replace(/[{}\s]/g, "").toLowerCase();

  return variables.filter((v) => needle === "" || `${v.label} ${v.key}`.toLowerCase().includes(needle)).slice(0, 12);
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
  findLibrary,
  findSavedReplies,
  takeSavedReply,
  findTemplates,
  takeTemplate,
  toolbarSlot,
  variables,
  autoFocus = true,
}: RichEditorProps) {
  const handlers = useRef({
    onChange,
    onSubmit,
    onBlur,
    onFiles,
    findMentions,
    findLibrary,
    findSavedReplies,
    takeSavedReply,
    findTemplates,
    takeTemplate,
  });
  handlers.current = {
    onChange,
    onSubmit,
    onBlur,
    onFiles,
    findMentions,
    findLibrary,
    findSavedReplies,
    takeSavedReply,
    findTemplates,
    takeTemplate,
  };
  const linkRef = useRef<() => void>(() => undefined);
  const [linking, setLinking] = useState<string | null>(null);
  const [emoji, setEmoji] = useState<Suggested<Emoji> | null>(null);
  const [mention, setMention] = useState<Suggested<AtItem> | null>(null);
  const [saved, setSaved] = useState<Suggested<SavedReply> | null>(null);
  const [template, setTemplate] = useState<Suggested<WhatsAppTemplate> | null>(null);
  const [variable, setVariable] = useState<Suggested<EditorVariable> | null>(null);
  const named = useRef(variables);
  named.current = variables;
  const variableHelpers = variables
    ? [
        VariableToken.configure({
          named: (key) => named.current?.find((v) => v.key === key)?.label ?? variableOf(key)?.label,
        }),
        suggestionExtension<EditorVariable>({
          name: "variableSuggestion",
          char: "{",
          minLength: 0,
          find: (query) => findVariable(named.current ?? [], query),
          content: (item) => [
            { type: "variableToken", attrs: { key: item.key, fallback: variableOf(item.key)?.fallback ?? "" } },
            { type: "text", text: " " },
          ],
          onChange: setVariable,
        }),
      ]
    : [];

  const editor = useEditor({
    extensions: composerExtensions(
      () => linkRef.current(),
      [
        suggestionExtension<Emoji>({
          name: "emojiSuggestion",
          char: ":",
          minLength: 1,
          find: async (query) => findEmoji(await loadEmoji(), query, 8),
          content: (item) => item.unicode,
          onChange: setEmoji,
        }),
        suggestionExtension<AtItem>({
          name: "mentionSuggestion",
          char: "@",
          minLength: 0,
          find: (query) => findAt(handlers.current, query),
          content: () => "",
          apply: (editor, range, item) => takeAt(editor, range, item, handlers.current.takeSavedReply),
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
        suggestionExtension<WhatsAppTemplate>({
          name: "templateSuggestion",
          char: "/",
          minLength: 0,
          find: (query) => handlers.current.findTemplates?.(query) ?? [],
          content: () => "",
          apply: (editor, range, picked) => {
            editor.chain().focus().deleteRange(range).run();
            handlers.current.takeTemplate?.(picked);
          },
          onChange: setTemplate,
        }),
        LinkToken,
        ...variableHelpers,
      ],
    ),
    content: value,
    contentType: "markdown",
    autofocus: autoFocus ? "end" : false,
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

  // The editor's own autofocus waits a frame. When it takes the place of
  // a plain field, keys pressed in that frame would land nowhere, so it
  // takes the focus as it mounts, with the caret already at the end.
  useLayoutEffect(() => {
    if (autoFocus && editor && !editor.isDestroyed && editor.view.dom.isConnected) {
      editor.view.focus();
    }
  }, [editor, autoFocus]);
  useEffect(() => whenReady(editor, (ready) => syncContent(ready, value)), [editor, value]);
  useEffect(
    () => whenReady(editor, (ready) => autoFocus && focusKey && ready.commands.focus("end")),
    [editor, focusKey, autoFocus],
  );

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
        label="Mention or add"
        suggested={mention}
        keyOf={atKey}
        render={(item) => <AtRow item={item} />}
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
        label="WhatsApp templates"
        suggested={template}
        keyOf={(item) => item.id}
        render={(item) => (
          <span className="flex min-w-0 flex-col">
            <span className="truncate">
              /{item.name}
              <span className="ms-1.5 text-text-tertiary">
                {item.language} · {item.category}
              </span>
            </span>
            <span className="truncate text-body-2-regular text-text-tertiary">{item.preview.replace(/[*_]/g, "")}</span>
          </span>
        )}
      />
      <SuggestionList
        label="Variables"
        suggested={variable}
        keyOf={(item) => item.key}
        render={(item) => (
          <span className="flex min-w-0 flex-col">
            <span className="truncate">{item.label}</span>
            <span className="truncate text-body-2-regular text-text-tertiary">{item.detail ?? item.key}</span>
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
      {editor ? <Formatting editor={editor} onLink={() => linkRef.current()} slot={toolbarSlot} /> : null}
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

/*
 * Formatting puts the toolbar where the composer asked for it, or under
 * the text.
 */
function Formatting({ editor, onLink, slot }: { editor: Editor; onLink: () => void; slot?: HTMLElement | null }) {
  const toolbar = <Toolbar editor={editor} onLink={onLink} />;

  return slot ? createPortal(toolbar, slot) : toolbar;
}

/*
 * Toolbar is the formatting buttons. An editor whose text takes fewer
 * formats, such as a WhatsApp template, names the ones it keeps.
 */
export function Toolbar({ editor, onLink, only }: { editor: Editor; onLink?: () => void; only?: string[] }) {
  const shown = only ? tools.filter((t) => only.includes(t.label)) : tools;
  const link = onLink ?? (() => undefined);
  const active = useEditorState({ editor, selector: ({ editor: e }) => shown.map((t) => t.active(e)) });

  return (
    <div role="toolbar" aria-label="Formatting" className="flex shrink-0 items-center gap-0.5">
      {shown.map((tool, index) => (
        <button
          key={tool.label}
          type="button"
          aria-label={tool.label}
          aria-pressed={active[index]}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => tool.run(editor, link)}
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

/*
 * AtItem is one suggestion after an at sign: a teammate or team to
 * mention, something from the library, or an emoji.
 */
type AtItem =
  | { kind: "mention"; mention: Mentionable }
  | { kind: "library"; item: LibraryItem }
  | { kind: "emoji"; emoji: Emoji };

/*
 * mentionsShown and emojiShown are how many teammates and emoji an at
 * sign lists beside the library's matches.
 */
const mentionsShown = 5;
const emojiShown = 3;

function atKey(item: AtItem): string {
  if (item.kind === "mention") {
    return `mention:${item.mention.id}`;
  }

  return item.kind === "library" ? `${item.item.kind}:${item.item.id}` : `emoji:${item.emoji.unicode}`;
}

/*
 * findAt searches every source an at sign offers at once: teammates
 * first where mentions are allowed, then the library, then emoji once
 * two letters name one.
 */
async function findAt(
  sources: Pick<RichEditorProps, "findMentions" | "findLibrary">,
  query: string,
): Promise<AtItem[]> {
  const [mentions, library, emoji] = await Promise.all([
    sources.findMentions?.(query).catch(() => []) ?? [],
    sources.findLibrary?.(query).catch(() => []) ?? [],
    query.trim().length >= 2 ? loadEmoji().then((all) => findEmoji(all, query, emojiShown)) : [],
  ]);

  return [
    ...mentions.slice(0, mentionsShown).map((mention) => ({ kind: "mention" as const, mention })),
    ...library.map((item) => ({ kind: "library" as const, item })),
    ...emoji.map((found) => ({ kind: "emoji" as const, emoji: found })),
  ];
}

/*
 * takeAt puts the picked suggestion in place of what was typed: a
 * mention or a link as a chip, a saved reply's text, or an emoji.
 */
function takeAt(
  editor: Editor,
  range: { from: number; to: number },
  picked: AtItem,
  takeSavedReply: RichEditorProps["takeSavedReply"],
) {
  const chain = editor.chain().focus();
  if (picked.kind === "mention") {
    chain
      .insertContentAt(range, [
        { type: "mentionToken", attrs: { id: picked.mention.id, label: picked.mention.name } },
        { type: "text", text: " " },
      ])
      .run();
    return;
  }
  if (picked.kind === "emoji") {
    chain.insertContentAt(range, picked.emoji.unicode).run();
    return;
  }
  const { item } = picked;
  if (item.reply) {
    chain.insertContentAt(range, takeSavedReply?.(item.reply) ?? item.reply.body, { contentType: "markdown" }).run();
    return;
  }
  chain
    .insertContentAt(
      range,
      item.url
        ? [
            { type: "linkToken", attrs: { title: item.title, url: item.url } },
            { type: "text", text: " " },
          ]
        : `${item.title} `,
    )
    .run();
}

const kindLooks = {
  saved: { icon: RiChatQuoteLine, name: "Saved reply" },
  article: { icon: RiArticleLine, name: "Help article" },
  request: { icon: RiLightbulbLine, name: "Feature request" },
  changelog: { icon: RiHistoryLine, name: "Changelog" },
} as const;

/*
 * AtRow is one suggestion as the list shows it, with what kind it is.
 */
function AtRow({ item }: { item: AtItem }) {
  if (item.kind === "emoji") {
    return (
      <>
        <span aria-hidden>{item.emoji.unicode}</span>
        <span className="truncate">{item.emoji.label}</span>
        <span className="ms-auto shrink-0 text-body-2-regular text-text-tertiary">Emoji</span>
      </>
    );
  }
  if (item.kind === "mention") {
    const { mention } = item;

    return (
      <span className="flex min-w-0 flex-1 items-center gap-2">
        {mention.kind === "team" ? (
          <RiTeamLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        ) : (
          <RiUser3Line aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        )}
        <span className="flex min-w-0 flex-col">
          <span className="truncate">
            @{mention.name}
            {mention.detail ? <span className="ms-1.5 text-text-tertiary">{mention.detail}</span> : null}
          </span>
          {mention.warning ? (
            <span className="text-body-2-regular text-text-error-primary">{mention.warning}</span>
          ) : null}
        </span>
      </span>
    );
  }
  const look = kindLooks[item.item.kind];
  const Icon = look.icon;

  return (
    <span className="flex min-w-0 flex-1 items-center gap-2">
      <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="flex min-w-0 flex-col">
        <span className="truncate">{item.item.title}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{look.name}</span>
      </span>
    </span>
  );
}
