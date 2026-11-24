/*
 * How routines look in lists: each template's mark, and the categories
 * the gallery groups templates under.
 */
import {
  RiCalendarScheduleLine,
  RiChat3Line,
  RiEmotionUnhappyLine,
  RiGitPullRequestLine,
  RiHourglassLine,
  RiLightbulbFlashLine,
  RiMegaphoneLine,
  RiQuillPenLine,
  RiRobot2Line,
  RiSearchEyeLine,
  RiStarLine,
  RiTestTubeLine,
} from "@remixicon/react";
import type { ComponentType } from "react";

import type { ChipColor } from "@/components/base/badges/chip";

/*
 * looks are each template's mark.
 */
const looks: Record<string, { icon: ComponentType<{ className?: string }>; tint: ChipColor }> = {
  tpl_knowledge_gap_finder: { icon: RiLightbulbFlashLine, tint: "yellow" },
  tpl_github_pr_docs_sync: { icon: RiGitPullRequestLine, tint: "purple" },
  tpl_knowledge_staleness_audit: { icon: RiSearchEyeLine, tint: "blue" },
  tpl_qa_procedure_digest: { icon: RiChat3Line, tint: "lime" },
  tpl_voice_of_customer: { icon: RiMegaphoneLine, tint: "cyan" },
  tpl_feature_request_roundup: { icon: RiStarLine, tint: "orange" },
  tpl_unhappy_customer_watch: { icon: RiEmotionUnhappyLine, tint: "rose" },
  tpl_waiting_conversations: { icon: RiHourglassLine, tint: "yellow" },
  tpl_ai_handover_review: { icon: RiRobot2Line, tint: "purple" },
  tpl_simulation_review: { icon: RiTestTubeLine, tint: "blue" },
  tpl_custom: { icon: RiQuillPenLine, tint: "neutral" },
};

/*
 * categories group the templates by what they are for, in the order the
 * gallery shows them. A template the app does not know lands in the
 * last one.
 */
const categories: { title: string; keys: string[] }[] = [
  {
    title: "Keep knowledge up to date",
    keys: ["tpl_knowledge_gap_finder", "tpl_knowledge_staleness_audit", "tpl_github_pr_docs_sync"],
  },
  { title: "Hear your customers", keys: ["tpl_voice_of_customer", "tpl_feature_request_roundup"] },
  { title: "Catch what needs a person", keys: ["tpl_unhappy_customer_watch", "tpl_waiting_conversations"] },
  {
    title: "Improve how the team works",
    keys: ["tpl_qa_procedure_digest", "tpl_ai_handover_review", "tpl_simulation_review"],
  },
  { title: "Your own", keys: ["tpl_custom"] },
];

const other = "More";

/*
 * grouped sorts templates into their categories, leaving out a category
 * with nothing in it.
 */
export function grouped<T extends { key: string }>(templates: T[]): { title: string; templates: T[] }[] {
  const known = new Set(categories.flatMap((c) => c.keys));
  const groups = categories.map((c) => ({
    title: c.title,
    templates: c.keys.flatMap((key) => templates.filter((t) => t.key === key)),
  }));
  const rest = templates.filter((t) => !known.has(t.key));

  return [...groups, { title: other, templates: rest }].filter((g) => g.templates.length > 0);
}

export function lookOf(key: string | null) {
  return looks[key ?? "tpl_custom"] ?? { icon: RiCalendarScheduleLine, tint: "neutral" as ChipColor };
}
