/*
 * What is left to translate, one language at a time: the articles
 * missing a translation, those whose source changed since, or every
 * article with where it stands, each opening the editor in that
 * language. The collections' names in that language sit below, typed
 * straight into their rows.
 */
import { RiArticleLine, RiFolder3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import type { Collection, HelpCenter } from "@/lib/api/help-centers";
import {
  type CollectionTranslation,
  collectionTranslationsQuery,
  saveCollectionTranslation,
  translationStatesQuery,
} from "@/lib/api/translations";
import { cx } from "@/utils/cx";

import { InlineName } from "./inline-name";
import { StandingChip } from "./language-menu";
import { standingOfState, useLanguageNames } from "./translation-standing";

type Show = "missing" | "outdated" | "all";

const shows: { id: Show; label: string }[] = [
  { id: "missing", label: "Missing" },
  { id: "outdated", label: "Outdated" },
  { id: "all", label: "All" },
];

export function TranslationList({
  workspace,
  center,
  collections,
  chips,
}: {
  workspace: string;
  center: HelpCenter | undefined;
  collections: Collection[];
  chips: ReactNode;
}) {
  if (!center) {
    return (
      <Panel label="Translations" className="h-full w-full flex-1">
        <PageHeader title="Translations" />
      </Panel>
    );
  }
  return <Translations workspace={workspace} center={center} collections={collections} chips={chips} />;
}

function Translations({
  workspace,
  center,
  collections,
  chips,
}: {
  workspace: string;
  center: HelpCenter;
  collections: Collection[];
  chips: ReactNode;
}) {
  const name = useLanguageNames(workspace);
  const others = (center.locales ?? [center.locale]).filter((l) => l !== center.locale);
  const [locale, setLocale] = useState(others[0] ?? "");
  const [show, setShow] = useState<Show>("missing");
  const { data: states = [], isPending } = useQuery({
    ...translationStatesQuery(workspace, center.id),
    enabled: workspace !== "",
  });
  const inLanguage = states.filter((s) => s.locale === locale);
  const shown = inLanguage.filter((s) => show === "all" || (show === "missing" ? s.state === "missing" : s.outdated));
  const count = (which: Show) =>
    which === "all"
      ? inLanguage.length
      : inLanguage.filter((s) => (which === "missing" ? s.state === "missing" : s.outdated)).length;

  if (others.length === 0) {
    return (
      <Panel label="Translations" className="h-full w-full flex-1">
        <PageHeader title="Translations" />
        <p className="px-4 py-6 text-body-2-regular text-text-tertiary">
          This help center speaks one language. Add another in Settings → Help centers to translate articles.
        </p>
      </Panel>
    );
  }

  return (
    <Panel label="Translations" className="h-full w-full flex-1">
      <PageHeader title="Translations">
        <Select
          aria-label="Language"
          size="sm"
          selectedKey={locale}
          onSelectionChange={(key) => (key === null ? undefined : setLocale(String(key)))}
          popoverClassName="w-56"
          popoverPlacement="bottom end"
        >
          {others.map((code) => (
            <SelectItem key={code} id={code} textValue={name(code)}>
              {name(code)}
            </SelectItem>
          ))}
        </Select>
      </PageHeader>
      {chips}
      <div className="px-4 pb-3">
        <SegmentedControl
          aria-label="Articles to show"
          selectedKeys={[show]}
          onSelectionChange={(keys) => {
            const [next] = [...keys];
            if (next) {
              setShow(next as Show);
            }
          }}
        >
          {shows.map((which) => (
            <SegmentedControlItem key={which.id} id={which.id}>
              {which.label} · {count(which.id)}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      </div>
      <ScrollArea>
        <div className="flex flex-col gap-1 px-2 pb-4">
          {!isPending && shown.length === 0 ? (
            <p className="px-2 py-6 text-body-2-regular text-text-tertiary">
              {show === "missing"
                ? `Every article has a ${name(locale)} translation.`
                : show === "outdated"
                  ? `No ${name(locale)} translation is behind its source.`
                  : "No articles yet."}
            </p>
          ) : null}
          {shown.map((s) => (
            <Link
              key={s.article_id}
              to="/d/help-center/$article"
              params={{ article: s.article_id }}
              search={{ lang: locale }}
              className="flex items-center gap-3 rounded-xl px-2 py-2.5 outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArticleLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary" />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate text-body-regular text-text-primary">{s.title}</span>
                <span className="truncate text-body-2-regular text-text-secondary">
                  {collections.find((c) => c.id === s.collection_id)?.name ?? "No collection"}
                </span>
              </span>
              <StandingChip standing={standingOfState(s)} />
            </Link>
          ))}
          <CollectionNames workspace={workspace} center={center} locale={locale} collections={collections} />
        </div>
      </ScrollArea>
    </Panel>
  );
}

/*
 * CollectionNames lists every collection with its name in the language,
 * one click away from typing it.
 */
function CollectionNames({
  workspace,
  center,
  locale,
  collections,
}: {
  workspace: string;
  center: HelpCenter;
  locale: string;
  collections: Collection[];
}) {
  const name = useLanguageNames(workspace);
  const queryClient = useQueryClient();
  const query = collectionTranslationsQuery(workspace, center.id);
  const { data: translations = [] } = useQuery({ ...query, enabled: workspace !== "" });
  const [editing, setEditing] = useState<string | null>(null);
  const save = useMutation({
    mutationFn: ({ collection, value }: { collection: string; value: string }) =>
      saveCollectionTranslation(workspace, collection, locale, { name: value }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: query.queryKey });
      setEditing(null);
      toast.success("Collection name saved");
    },
    onError: (error) => toast.error(error.message),
  });
  if (collections.length === 0) {
    return null;
  }
  const translated = (id: string): CollectionTranslation | undefined =>
    translations.find((t) => t.collection_id === id && t.locale === locale);

  return (
    <section aria-label={`Collection names in ${name(locale)}`} className="mt-6">
      <h2 className="px-2 pb-2 text-caption-1-medium text-text-tertiary">Collection names in {name(locale)}</h2>
      {collections.map((c) => {
        const t = translated(c.id);
        return (
          <div key={c.id} className="flex min-h-11 items-center gap-3 rounded-xl px-2 py-1.5">
            <RiFolder3Line aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary" />
            <span className="flex min-w-0 flex-1 flex-col sm:flex-row-reverse sm:items-center sm:gap-3">
              <span className="min-w-0 sm:flex-1">
                {editing === c.id ? (
                  <InlineName
                    initial={t?.name ?? ""}
                    label={`${c.name} in ${name(locale)}`}
                    onCommit={(value) => save.mutateAsync({ collection: c.id, value })}
                    onCancel={() => setEditing(null)}
                  />
                ) : (
                  <button
                    type="button"
                    onClick={() => setEditing(c.id)}
                    className={cx(
                      "w-full truncate rounded-lg px-2 py-1 text-start text-body-regular outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                      t ? "text-text-primary" : "text-text-placeholder",
                    )}
                  >
                    {t?.name ?? `Name it in ${name(locale)}`}
                  </button>
                )}
              </span>
              <span className="truncate px-2 text-body-2-regular text-text-secondary sm:w-2/5 sm:shrink-0 sm:px-0 sm:text-body-regular">
                {c.name}
              </span>
            </span>
            <StandingChip standing={t ? (t.outdated ? "outdated" : "translated") : "missing"} />
          </div>
        );
      })}
    </section>
  );
}
