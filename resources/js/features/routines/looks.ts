/*
 * How routines look in lists: each template's mark, and the names of
 * what a template needs connected.
 */
import {
  RiCalendarScheduleLine,
  RiChat3Line,
  RiGitPullRequestLine,
  RiLightbulbFlashLine,
  RiQuillPenLine,
  RiSearchEyeLine,
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
  tpl_custom: { icon: RiQuillPenLine, tint: "neutral" },
};

export const needs: Record<string, string> = {
  github: "GitHub",
  help_center: "Help center",
  conversations: "Conversations",
  procedures: "Procedures",
};

export function lookOf(key: string | null) {
  return looks[key ?? "tpl_custom"] ?? { icon: RiCalendarScheduleLine, tint: "neutral" as ChipColor };
}
