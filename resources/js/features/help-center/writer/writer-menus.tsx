/*
 * The writer's floating controls: the bar over selected text, the bar
 * over a selected picture, and the plus in the margin of an empty line,
 * which opens the same block menu a slash does.
 */
import {
  type RemixiconComponentType,
  RiAddLine,
  RiAlignCenter,
  RiAlignLeft,
  RiAlignRight,
  RiAspectRatioLine,
  RiBold,
  RiCodeLine,
  RiDeleteBinLine,
  RiDoubleQuotesL,
  RiH2,
  RiH3,
  RiImageEditLine,
  RiItalic,
  RiLink,
  RiStrikethrough,
} from "@remixicon/react";
import { type Editor, useEditorState } from "@tiptap/react";
import { BubbleMenu } from "@tiptap/react/menus";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import { applyLink } from "@/features/inbox/composer/editor-helpers";
import { LinkRow } from "@/features/inbox/composer/link-row";
import { cx } from "@/utils/cx";

import type { ImageAlign } from "./image-layout";

interface Tool {
  label: string;
  icon: RemixiconComponentType;
  active: (editor: Editor) => boolean;
  run: (editor: Editor) => unknown;
}

const marks: Tool[] = [
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
  {
    label: "Heading",
    icon: RiH2,
    active: (e) => e.isActive("heading", { level: 2 }),
    run: (e) => e.chain().focus().toggleHeading({ level: 2 }).run(),
  },
  {
    label: "Subheading",
    icon: RiH3,
    active: (e) => e.isActive("heading", { level: 3 }),
    run: (e) => e.chain().focus().toggleHeading({ level: 3 }).run(),
  },
  {
    label: "Quote",
    icon: RiDoubleQuotesL,
    active: (e) => e.isActive("blockquote"),
    run: (e) => e.chain().focus().toggleBlockquote().run(),
  },
];

const buttonClass = cx(
  "flex size-8 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none",
  "transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
);

const barClass = cx(
  "flex items-center gap-0.5 rounded-xl border border-border-button-default bg-background-primary-default p-1",
  "shadow-dropdown motion-safe:animate-menu-in",
);

function ToolButtons({ editor, tools }: { editor: Editor; tools: Tool[] }) {
  const active = useEditorState({ editor, selector: ({ editor: e }) => tools.map((t) => t.active(e)) });

  return (
    <>
      {tools.map((tool, index) => (
        <button
          key={tool.label}
          type="button"
          aria-label={tool.label}
          aria-pressed={active[index]}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => tool.run(editor)}
          className={cx(buttonClass, active[index] && "bg-background-tertiary-default text-foreground-icon-primary")}
        >
          <tool.icon aria-hidden className="size-4" />
        </button>
      ))}
    </>
  );
}

/*
 * SelectionBar is the bar over selected text, or the link's address
 * while one is being set.
 */
export function SelectionBar({ editor }: { editor: Editor }) {
  const [linking, setLinking] = useState<string | null>(null);

  return (
    <BubbleMenu
      editor={editor}
      pluginKey="textBar"
      shouldShow={({ editor: e, from, to }) =>
        from !== to && e.isEditable && !e.isActive("codeBlock") && !e.isActive("image")
      }
    >
      <div role="toolbar" aria-label="Formatting" className={barClass}>
        {linking === null ? (
          <>
            <ToolButtons editor={editor} tools={marks} />
            <span aria-hidden className="mx-0.5 h-5 w-px bg-separator-border" />
            <button
              type="button"
              aria-label="Link"
              aria-pressed={editor.isActive("link")}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => setLinking((editor.getAttributes("link").href as string | undefined) ?? "https://")}
              className={buttonClass}
            >
              <RiLink aria-hidden className="size-4" />
            </button>
          </>
        ) : (
          <div className="w-64">
            <LinkRow
              initial={linking}
              onApply={(href) => {
                applyLink(editor, href);
                setLinking(null);
              }}
              onCancel={() => setLinking(null)}
            />
          </div>
        )}
      </div>
    </BubbleMenu>
  );
}

const placements: { align: ImageAlign | null; label: string; icon: RemixiconComponentType }[] = [
  { align: null, label: "Align left", icon: RiAlignLeft },
  { align: "center", label: "Align center", icon: RiAlignCenter },
  { align: "end", label: "Align right", icon: RiAlignRight },
];

/*
 * ImageBar is the bar over a selected picture: what it shows, for people
 * who cannot see it, where it sits in the column, going back to its own
 * size after a resize, swapping it for another file, and removing it.
 */
export function ImageBar({ editor, onReplace }: { editor: Editor; onReplace: () => void }) {
  const image = useEditorState({
    editor,
    selector: ({ editor: e }) => {
      const attrs = e.getAttributes("image");

      return {
        alt: (attrs.alt as string) ?? "",
        align: (attrs.align as ImageAlign | null) ?? null,
        sized: attrs.width != null,
      };
    },
  });
  const set = (attrs: Record<string, unknown>) => editor.chain().updateAttributes("image", attrs).run();

  return (
    <BubbleMenu
      editor={editor}
      pluginKey="imageBar"
      shouldShow={({ editor: e }) => e.isEditable && e.isActive("image")}
    >
      <div role="toolbar" aria-label="Image" className={cx(barClass, "gap-1 ps-2")}>
        <div className="w-52">
          <Input
            size="small"
            aria-label="Alt text"
            placeholder="Describe the image"
            value={image.alt}
            onChange={(value) => set({ alt: value })}
          />
        </div>
        <span aria-hidden className="mx-0.5 h-5 w-px bg-separator-border" />
        {placements.map((p) => (
          <button
            key={p.label}
            type="button"
            aria-label={p.label}
            aria-pressed={image.align === p.align}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => set({ align: p.align })}
            className={cx(
              buttonClass,
              image.align === p.align && "bg-background-tertiary-default text-foreground-icon-primary",
            )}
          >
            <p.icon aria-hidden className="size-4" />
          </button>
        ))}
        {image.sized ? (
          <button
            type="button"
            aria-label="Original size"
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => set({ width: null })}
            className={buttonClass}
          >
            <RiAspectRatioLine aria-hidden className="size-4" />
          </button>
        ) : null}
        <span aria-hidden className="mx-0.5 h-5 w-px bg-separator-border" />
        <button
          type="button"
          aria-label="Replace image"
          onMouseDown={(event) => event.preventDefault()}
          onClick={onReplace}
          className={buttonClass}
        >
          <RiImageEditLine aria-hidden className="size-4" />
        </button>
        <button
          type="button"
          aria-label="Remove image"
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => editor.chain().focus().deleteSelection().run()}
          className={buttonClass}
        >
          <RiDeleteBinLine aria-hidden className="size-4" />
        </button>
      </div>
    </BubbleMenu>
  );
}

/*
 * emptyLineAt is where the empty top-level line holding the caret
 * starts, or null when the caret is anywhere else.
 */
function emptyLineAt(editor: Editor): number | null {
  const { $anchor, empty } = editor.state.selection;
  const blank = $anchor.parent.isTextblock && $anchor.parent.content.size === 0;
  if (!empty || $anchor.depth !== 1 || !blank || $anchor.parent.type.name !== "paragraph") {
    return null;
  }

  return $anchor.before();
}

/*
 * BlockPlus is the plus in the margin beside an empty line; it opens the
 * block menu by typing the slash for the writer.
 */
export function BlockPlus({ editor }: { editor: Editor }) {
  const at = useEditorState({ editor, selector: ({ editor: e }) => emptyLineAt(e) });
  const line = at === null ? null : (editor.view.nodeDOM(at) as HTMLElement | null);
  if (!line) {
    return null;
  }

  return (
    <button
      type="button"
      aria-label="Add a block"
      onMouseDown={(event) => event.preventDefault()}
      onClick={() => editor.chain().focus().insertContent("/").run()}
      style={{ top: line.offsetTop + line.offsetHeight / 2 - 14 }}
      className={cx(
        "absolute -start-11 flex size-7 cursor-pointer items-center justify-center rounded-full border border-border-button-default",
        "bg-background-primary-default text-foreground-icon-tertiary outline-none transition-colors duration-150 motion-safe:animate-menu-in",
        "hover:border-border-button-hover hover:text-foreground-icon-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring",
      )}
    >
      <RiAddLine aria-hidden className="size-4" />
    </button>
  );
}
