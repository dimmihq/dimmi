/*
 * The article's languages beside its title: the default one it is
 * written in, then every other language the help center speaks, each
 * with where its translation stands, Draft, Published, Outdated or
 * Missing. Picking one opens the article in that language in the same
 * editor.
 */
import { RiTranslate2 } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { Chip } from "@/components/base/badges/chip";
import { Select, SelectItem } from "@/components/base/select/select";
import type { HelpCenter } from "@/lib/api/help-centers";
import { articleTranslationsQuery } from "@/lib/api/translations";

import { standingOf, standings, type TranslationStanding, useLanguageNames } from "./translation-standing";

export function StandingChip({ standing }: { standing: TranslationStanding }) {
  return (
    <Chip variant="caption" color={standings[standing].color}>
      {standings[standing].text}
    </Chip>
  );
}

export function LanguageMenu({
  workspace,
  article,
  center,
  current,
}: {
  workspace: string;
  article: string;
  center: HelpCenter;
  current: string;
}) {
  const navigate = useNavigate();
  const name = useLanguageNames(workspace);
  const { data: translations = [] } = useQuery({
    ...articleTranslationsQuery(workspace, article),
    enabled: workspace !== "",
  });
  const locales = center.locales ?? [center.locale];
  if (locales.length < 2) {
    return null;
  }
  const standing = (code: string): TranslationStanding =>
    code === center.locale ? "default" : standingOf(translations.find((t) => t.locale === code));

  return (
    <Select
      aria-label="Article language"
      size="sm"
      variant="quiet"
      selectedKey={current}
      onSelectionChange={(key) => {
        if (key === null || key === current) {
          return;
        }
        const lang = String(key);
        void navigate({
          to: "/d/help-center/$article",
          params: { article },
          search: lang === center.locale ? {} : { lang },
        });
      }}
      className="shrink-0"
      triggerClassName="sm:min-w-36"
      popoverClassName="w-72"
      popoverPlacement="bottom end"
      renderValue={
        <span className="flex items-center gap-1.5">
          <RiTranslate2 aria-hidden className="size-4 text-foreground-icon-secondary" />
          <span className="max-sm:sr-only">{name(current)}</span>
        </span>
      }
    >
      {locales.map((code) => (
        <SelectItem key={code} id={code} textValue={name(code)}>
          <span className="flex w-full items-center justify-between gap-3">
            <span>{name(code)}</span>
            <StandingChip standing={standing(code)} />
          </span>
        </SelectItem>
      ))}
    </Select>
  );
}
