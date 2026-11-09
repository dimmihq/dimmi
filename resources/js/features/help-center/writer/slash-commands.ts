/*
 * The blocks a slash starts in the article writer: headings, lists,
 * quotes, callouts, images, code, dividers and tables. Typing after the
 * slash narrows them by name or by a word that means the same.
 */
import {
  type RemixiconComponentType,
  RiAlertLine,
  RiCodeSSlashLine,
  RiDoubleQuotesL,
  RiErrorWarningLine,
  RiH2,
  RiH3,
  RiImageLine,
  RiInformationLine,
  RiLightbulbLine,
  RiListOrdered,
  RiListUnordered,
  RiSeparator,
  RiSpam2Line,
  RiTable2,
  RiText,
} from "@remixicon/react";
import type { ChainedCommands, Editor, Range } from "@tiptap/react";

export interface SlashCommand {
  id: string;
  label: string;
  hint: string;
  icon: RemixiconComponentType;
  words: string[];
  run: (chain: ChainedCommands) => ChainedCommands;
}

/*
 * SlashActions are what a command needs from the writer, such as asking
 * for a picture to upload.
 */
export interface SlashActions {
  pickImage: () => void;
}

export const slashCommands: SlashCommand[] = [
  {
    id: "text",
    label: "Text",
    hint: "Plain writing",
    icon: RiText,
    words: ["paragraph"],
    run: (c) => c.setParagraph(),
  },
  {
    id: "heading",
    label: "Heading",
    hint: "A section title",
    icon: RiH2,
    words: ["h2", "title"],
    run: (c) => c.setHeading({ level: 2 }),
  },
  {
    id: "subheading",
    label: "Subheading",
    hint: "A smaller title",
    icon: RiH3,
    words: ["h3"],
    run: (c) => c.setHeading({ level: 3 }),
  },
  {
    id: "bullets",
    label: "Bulleted list",
    hint: "A simple list",
    icon: RiListUnordered,
    words: ["ul", "unordered"],
    run: (c) => c.toggleBulletList(),
  },
  {
    id: "numbers",
    label: "Numbered list",
    hint: "Steps in order",
    icon: RiListOrdered,
    words: ["ol", "ordered", "steps"],
    run: (c) => c.toggleOrderedList(),
  },
  {
    id: "note",
    label: "Note",
    hint: "A callout to notice",
    icon: RiInformationLine,
    words: ["callout", "info"],
    run: (c) => c.setCallout("note"),
  },
  {
    id: "tip",
    label: "Tip",
    hint: "A callout that helps",
    icon: RiLightbulbLine,
    words: ["callout", "hint"],
    run: (c) => c.setCallout("tip"),
  },
  {
    id: "important",
    label: "Important",
    hint: "A callout not to miss",
    icon: RiErrorWarningLine,
    words: ["callout"],
    run: (c) => c.setCallout("important"),
  },
  {
    id: "warning",
    label: "Warning",
    hint: "A callout about a risk",
    icon: RiAlertLine,
    words: ["callout", "careful"],
    run: (c) => c.setCallout("warning"),
  },
  {
    id: "caution",
    label: "Caution",
    hint: "A callout about danger",
    icon: RiSpam2Line,
    words: ["callout", "danger"],
    run: (c) => c.setCallout("caution"),
  },
  {
    id: "quote",
    label: "Quote",
    hint: "Words set apart",
    icon: RiDoubleQuotesL,
    words: ["blockquote"],
    run: (c) => c.toggleBlockquote(),
  },
  {
    id: "image",
    label: "Image",
    hint: "Upload a picture",
    icon: RiImageLine,
    words: ["picture", "photo", "screenshot"],
    run: (c) => c,
  },
  {
    id: "code",
    label: "Code block",
    hint: "Commands or code",
    icon: RiCodeSSlashLine,
    words: ["pre", "snippet"],
    run: (c) => c.setCodeBlock(),
  },
  {
    id: "divider",
    label: "Divider",
    hint: "A line between parts",
    icon: RiSeparator,
    words: ["hr", "rule", "line"],
    run: (c) => c.setHorizontalRule(),
  },
  {
    id: "table",
    label: "Table",
    hint: "Rows and columns",
    icon: RiTable2,
    words: ["grid"],
    run: (c) => c.insertTable({ rows: 3, cols: 2, withHeaderRow: true }),
  },
];

/*
 * findCommands lists the commands matching what was typed after the
 * slash: those whose name starts with it, then those whose name or
 * other words contain it.
 */
export function findCommands(query: string): SlashCommand[] {
  const q = query.trim().toLowerCase();
  const named = (c: SlashCommand) => c.label.toLowerCase();
  const starts = slashCommands.filter((c) => named(c).startsWith(q));
  const contains = slashCommands.filter(
    (c) => !named(c).startsWith(q) && (named(c).includes(q) || c.words.some((w) => w.startsWith(q))),
  );

  return [...starts, ...contains];
}

/*
 * runCommand swaps the slash and what followed it for the block.
 */
export function runCommand(editor: Editor, range: Range, command: SlashCommand, actions: SlashActions) {
  const chain = editor.chain().focus().deleteRange(range);
  if (command.id === "image") {
    chain.run();
    actions.pickImage();
    return;
  }
  command.run(chain).run();
}
