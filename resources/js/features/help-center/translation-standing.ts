/*
 * Where an article stands in one language, as the editor's chips name
 * it, and the languages' names in themselves. Kept apart from the
 * components so the wording can be tested without rendering them.
 */
import { useQuery } from "@tanstack/react-query";

import type { ChipColor } from "@/components/base/badges/chip";
import { type ArticleTranslation, languagesQuery, type TranslationState } from "@/lib/api/translations";

export type TranslationStanding = "default" | "missing" | "draft" | "published" | "translated" | "changes" | "outdated";

export const standings: Record<TranslationStanding, { text: string; color: ChipColor }> = {
  default: { text: "Default", color: "neutral" },
  missing: { text: "Missing", color: "soft" },
  draft: { text: "Draft", color: "gray" },
  published: { text: "Published", color: "lime" },
  translated: { text: "Translated", color: "lime" },
  changes: { text: "Unpublished changes", color: "yellow" },
  outdated: { text: "Outdated", color: "orange" },
};

/*
 * standingOf says where a translation stands, outdated first.
 */
export function standingOf(t: ArticleTranslation | undefined): TranslationStanding {
  if (!t) {
    return "missing";
  }
  if (t.outdated) {
    return "outdated";
  }
  if (t.status === "draft") {
    return "draft";
  }
  return t.has_unpublished_changes ? "changes" : "published";
}

/*
 * standingOfState says where an article stands in a language of the
 * help center's list.
 */
export function standingOfState(s: TranslationState): TranslationStanding {
  return s.outdated ? "outdated" : s.state;
}

/*
 * leftToTranslate counts the articles missing or behind in every
 * language.
 */
export function leftToTranslate(states: TranslationState[]): number {
  return states.filter((s) => s.state === "missing" || s.outdated).length;
}

/*
 * useLanguageNames names the languages in themselves, falling back to
 * the code.
 */
export function useLanguageNames(workspace: string) {
  const { data: languages = [] } = useQuery({ ...languagesQuery(workspace), enabled: workspace !== "" });
  return (code: string) => languages.find((l) => l.code === code)?.native_name ?? code;
}
