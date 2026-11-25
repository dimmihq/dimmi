/*
 * What a routine's instructions can mention: a team, a teammate, an
 * inbox, a tag, an AI agent, a help article, a procedure, a ticket type
 * or an action. A mention is kept in the instructions as [[kind:ref]],
 * the token the server spells out by the name its target has when the
 * routine runs, so a rename never breaks the instructions.
 */
import {
  RiArticleLine,
  RiFlashlightLine,
  RiInbox2Line,
  RiListOrdered2,
  RiPriceTag3Line,
  RiRobot2Line,
  RiTeamLine,
  RiTicketLine,
  RiUserLine,
} from "@remixicon/react";

import type { ChipColor } from "@/components/base/badges/chip";

export const mentionKinds = [
  "handoff",
  "teammate",
  "inbox",
  "tag",
  "agent",
  "article",
  "sub_procedure",
  "ticket_type",
  "action",
] as const;
export type MentionKind = (typeof mentionKinds)[number];

export interface Named {
  id: string;
  name: string;
  /* A tag's own colour and icon. */
  color?: string | null;
  icon?: string | null;
  /* A teammate's photo. */
  avatar?: string | null;
  /* Whether an agent is the default one, which has its own colour. */
  isDefault?: boolean;
}

/*
 * Mentionables is everything of the workspace a routine can name, by
 * kind.
 */
export type Mentionables = Record<MentionKind, Named[]>;

export const nothingToMention: Mentionables = {
  handoff: [],
  teammate: [],
  inbox: [],
  tag: [],
  agent: [],
  article: [],
  sub_procedure: [],
  ticket_type: [],
  action: [],
};

/*
 * looks are each kind's heading in the list, its icon, and the word a
 * chip shows when what it points at is gone or not loaded yet.
 */
export const looks: Record<MentionKind, { group: string; icon: typeof RiTeamLine; word: string; tint: ChipColor }> = {
  handoff: { group: "Teams", icon: RiTeamLine, word: "team", tint: "orange" },
  teammate: { group: "Teammates", icon: RiUserLine, word: "teammate", tint: "blue" },
  inbox: { group: "Inboxes", icon: RiInbox2Line, word: "inbox", tint: "blue" },
  tag: { group: "Tags", icon: RiPriceTag3Line, word: "tag", tint: "neutral" },
  agent: { group: "AI agents", icon: RiRobot2Line, word: "agent", tint: "purple" },
  article: { group: "Help articles", icon: RiArticleLine, word: "article", tint: "cyan" },
  sub_procedure: { group: "Procedures", icon: RiListOrdered2, word: "procedure", tint: "purple" },
  ticket_type: { group: "Ticket types", icon: RiTicketLine, word: "ticket type", tint: "rose" },
  action: { group: "Actions", icon: RiFlashlightLine, word: "action", tint: "lime" },
};

export interface MentionOption {
  kind: MentionKind;
  ref: string;
  label: string;
  group: string;
  /* What it points at, for its colour or face. */
  target: Named;
  /* Whether it is the first of its group in the list, which shows the group's heading. */
  first: boolean;
}

const perGroup = 5;

/*
 * isMentionKind says whether a kind read from the text is one a routine
 * can mention.
 */
export function isMentionKind(kind: string): kind is MentionKind {
  return (mentionKinds as readonly string[]).includes(kind);
}

/*
 * findMentions lists what matches the words typed after the @, a few of
 * each kind, in the order the kinds are listed.
 */
export function findMentions(known: Mentionables, query: string): MentionOption[] {
  const needle = query.trim().toLowerCase();
  const out: MentionOption[] = [];

  for (const kind of mentionKinds) {
    const matches = known[kind]
      .filter((n) => needle === "" || n.name.toLowerCase().includes(needle))
      .slice(0, perGroup);

    matches.forEach((n, i) => {
      out.push({ kind, ref: n.id, label: n.name, group: looks[kind].group, target: n, first: i === 0 });
    });
  }

  return out;
}

/*
 * nameOf is what a mention points at now, or nothing when it is gone or
 * not loaded yet.
 */
export function nameOf(known: Mentionables, kind: string, ref: string): string {
  if (!isMentionKind(kind)) {
    return "";
  }

  return known[kind].find((n) => n.id === ref)?.name ?? "";
}

/*
 * targetOf is what a mention points at now, or nothing when it is gone
 * or not loaded yet.
 */
export function targetOf(known: Mentionables, kind: string, ref: string): Named | undefined {
  return isMentionKind(kind) ? known[kind].find((n) => n.id === ref) : undefined;
}

/*
 * token is a mention as the instructions keep it.
 */
export function token(kind: string, ref: string): string {
  return `[[${kind}:${ref}]]`;
}
