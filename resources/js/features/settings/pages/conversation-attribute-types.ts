/*
 * Each conversation attribute type's mark and one-line hint, the types
 * that take options, and the descriptions kept for a list's options.
 */
import {
  RiAlignLeft,
  RiAttachment2,
  RiBuilding2Line,
  RiCalendarLine,
  RiCheckboxMultipleLine,
  RiHashtag,
  RiListCheck,
  RiNumbersLine,
  RiText,
  RiTimeLine,
  RiToggleLine,
  RiUser3Line,
} from "@remixicon/react";
import type { ComponentType } from "react";

import type { ConversationAttributeType } from "@/lib/api/conversation-attributes";

export const typeIcons: Record<ConversationAttributeType, ComponentType<{ className?: string }>> = {
  text: RiText,
  long_text: RiAlignLeft,
  number: RiHashtag,
  decimal: RiNumbersLine,
  choice: RiListCheck,
  multiple_choice: RiCheckboxMultipleLine,
  boolean: RiToggleLine,
  date: RiCalendarLine,
  datetime: RiTimeLine,
  file: RiAttachment2,
  contact: RiUser3Line,
  company: RiBuilding2Line,
};

export const typeHints: Record<ConversationAttributeType, string> = {
  text: "One line, such as an order number.",
  long_text: "Several lines, such as steps to reproduce.",
  number: "A whole number, such as seats.",
  decimal: "A number with decimals, such as a refund amount.",
  choice: "One of a list, such as the issue type.",
  multiple_choice: "Any of a list, such as the products involved.",
  boolean: "Yes or no, such as whether it's a bug.",
  date: "A day, such as a renewal date.",
  datetime: "A day and a time, such as a callback.",
  file: "Files a teammate attaches, such as a screenshot.",
  contact: "A link to one of your contacts.",
  company: "A link to one of your companies.",
};

export const choosable: ConversationAttributeType[] = ["choice", "multiple_choice"];

/*
 * describedOf keeps the descriptions of the options the list has, and
 * drops empty ones.
 */
export function describedOf(options: string[], descriptions: Record<string, string>): Record<string, string> {
  return Object.fromEntries(
    options.map((o) => [o, (descriptions[o] ?? "").trim()] as const).filter(([, text]) => text !== ""),
  );
}
