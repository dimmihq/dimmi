/*
 * Writing an article in another language the help center speaks, in the
 * same editor as the source: the language menu beside the title, then
 * Save draft and Publish for this language alone, and its own version
 * history. A language with no translation yet starts from the source's
 * text or from a blank page. When the source was published with other
 * words since the translation caught up, a note says so; Mark up to
 * date catches it up without publishing, and the source opens beside it
 * to compare.
 */
import { RiArrowLeftLine, RiExternalLinkLine, RiHistoryLine, RiTranslate2 } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import {
  type Article,
  articleQuery,
  type Collection,
  collectionsQuery,
  defaultBrand,
  helpCentersQuery,
} from "@/lib/api/help-centers";
import {
  type ArticleTranslation,
  articleTranslationQuery,
  articleTranslationsQuery,
  markTranslationCurrent,
  publishArticleTranslation,
  saveArticleTranslation,
  translationStatesQuery,
  translationVersionsQuery,
  unpublishArticleTranslation,
} from "@/lib/api/translations";
import { cx } from "@/utils/cx";

import { ArticlePage } from "./article-page";
import { collectionOptions, type Draft } from "./docs-format";
import { useHistoryPane } from "./history-pane";
import { LanguageMenu, StandingChip } from "./language-menu";
import { standingOf, useLanguageNames } from "./translation-standing";

interface Words {
  title: string;
  slug: string;
  description: string;
  body: string;
}

function wordsOf(t: Pick<ArticleTranslation, "title" | "slug" | "description" | "body_markdown">): Words {
  return { title: t.title, slug: t.slug, description: t.description ?? "", body: t.body_markdown ?? "" };
}

export function TranslationEditor({ id, locale }: { id: string; locale: string }) {
  const workspace = useAccount().workspace.id;
  const enabled = workspace !== "";
  const navigate = useNavigate();
  const name = useLanguageNames(workspace);
  const { data: article } = useQuery({ ...articleQuery(workspace, id), enabled });
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled });
  const center = centers.find((c) => c.id === article?.help_center_id);
  const { data: collections = [] } = useQuery({
    ...collectionsQuery(workspace, article?.help_center_id ?? ""),
    enabled: Boolean(article),
  });
  const translation = useQuery({ ...articleTranslationQuery(workspace, id, locale), enabled });
  const [words, setWords] = useState<Words | null>(null);
  const [revision, setRevision] = useState(0);
  const [confirm, setConfirm] = useState<"publish" | "unpublish" | null>(null);
  const missing = translation.isError;

  useEffect(() => {
    if (translation.data && words === null) {
      setWords(wordsOf(translation.data));
    }
  }, [translation.data, words]);

  const { settle, save, unpublish, review } = useTranslationWrites({
    workspace,
    id,
    locale,
    helpCenter: article?.help_center_id,
    words,
    lockVersion: translation.data?.lock_version,
  });
  const history = useHistoryPane(
    workspace,
    id,
    (restored) => {
      if (restored.object === "article_translation") {
        settle(restored);
        setWords(wordsOf(restored));
        setRevision((r) => r + 1);
      }
    },
    undefined,
    locale,
  );

  if (!article || !center || (!translation.data && !missing)) {
    return (
      <Panel label="Translation" className="h-full flex-1">
        <PageHeader title="Translation" />
      </Panel>
    );
  }

  const start = (from: "source" | "blank") => {
    setWords(startingWords(article, from));
    setRevision((r) => r + 1);
  };
  const source = name(center.locale);
  const language = name(locale);
  const current = translation.data;
  const collection = collectionLabel(collections, article.collection_id);
  const draft = draftOf(words, article);
  const live = liveAddress(center.url, locale, current);

  return history.layout(
    <Panel label="Translation" className="h-full w-full flex-1">
      <PageHeader
        title={
          <span className="flex items-center gap-2">
            <Link
              to="/d/help-center"
              search={article.collection_id ? { collection: article.collection_id } : {}}
              aria-label="Back to help center"
              className="rounded-md p-1 text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftLine aria-hidden className="size-4" />
            </Link>
            <span className="truncate">{words?.title || current?.title || article.title}</span>
            <StandingChip standing={standingOf(current)} />
          </span>
        }
      >
        <LanguageMenu workspace={workspace} article={id} center={center} current={locale} />
        <TranslationActions
          live={live}
          current={current}
          words={words}
          saving={save.isPending}
          history={history}
          onSave={() => save.mutate(false, { onSuccess: () => toast.success("Draft saved") })}
          onConfirm={setConfirm}
        />
      </PageHeader>
      <ScrollArea>
        {current?.outdated ? (
          <OutdatedNote
            source={source}
            language={language}
            onOpenSource={() => void navigate({ to: "/d/help-center/$article", params: { article: id }, search: {} })}
            onReview={() => review.mutate()}
          />
        ) : null}
        {draft ? (
          <ArticlePage
            workspace={workspace}
            draft={draft}
            brand={center.settings.primary_color || defaultBrand}
            collection={collection}
            revision={revision}
            failure={save.error ?? unpublish.error ?? review.error}
            onChange={(change) =>
              setWords({
                title: change.title ?? draft.title,
                slug: draft.slug,
                description: change.description ?? draft.description,
                body: change.body ?? draft.body,
              })
            }
            onPickCollection={() => undefined}
          />
        ) : (
          <MissingTranslation
            source={source}
            language={language}
            hidden={center.settings.untranslated_articles === "hide"}
            onStart={start}
          />
        )}
      </ScrollArea>
      <AlertDialog
        isOpen={confirm === "publish"}
        onOpenChange={(open) => (open ? undefined : setConfirm(null))}
        variant="default"
        title={`Publish the ${language} article?`}
        description={`Readers of ${language} see it right away, and the dimmi agent answers them from it.`}
        confirmLabel="Publish"
        onConfirm={() => save.mutateAsync(true)}
      />
      <AlertDialog
        isOpen={confirm === "unpublish"}
        onOpenChange={(open) => (open ? undefined : setConfirm(null))}
        title={`Unpublish the ${language} article?`}
        description={`Readers of ${language} stop seeing it. It goes back to a draft and every version is kept.`}
        confirmLabel="Unpublish"
        onConfirm={() => unpublish.mutateAsync()}
      />
    </Panel>,
  );
}

function TranslationActions({
  live,
  current,
  words,
  saving,
  history,
  onSave,
  onConfirm,
}: {
  live: string | null;
  current: ArticleTranslation | undefined;
  words: Words | null;
  saving: boolean;
  history: { shown: boolean; toggle: (view?: "history") => void };
  onSave: () => void;
  onConfirm: (what: "publish" | "unpublish") => void;
}) {
  const blank = !words?.title.trim();

  return (
    <>
      {live ? (
        <a
          href={live}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 rounded-lg px-2 py-1.5 text-body-2-medium text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiExternalLinkLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
          View
        </a>
      ) : null}
      {current?.status === "published" ? (
        <Button size="small" variant="secondary" onClick={() => onConfirm("unpublish")}>
          Unpublish
        </Button>
      ) : null}
      {current ? (
        <IconButton
          size="small"
          icon={RiHistoryLine}
          aria-label="Version history"
          aria-pressed={history.shown}
          className={cx(history.shown && "border-border-button-active bg-background-primary-active")}
          onClick={() => history.toggle("history")}
        />
      ) : null}
      {words ? (
        <>
          <Button size="small" variant="secondary" onClick={onSave} disabled={saving || blank}>
            Save draft
          </Button>
          <Button size="small" onClick={() => onConfirm("publish")} disabled={saving || blank}>
            {saving ? "Saving…" : "Publish"}
          </Button>
        </>
      ) : null}
    </>
  );
}

/*
 * OutdatedNote says the source moved on since the translation caught
 * up, with the way to the source and the way to mark it up to date.
 */
function OutdatedNote({
  source,
  language,
  onOpenSource,
  onReview,
}: {
  source: string;
  language: string;
  onOpenSource: () => void;
  onReview: () => void;
}) {
  return (
    <div className="mx-auto w-full max-w-[42.5rem] px-6 pt-6">
      <Notification
        status="information"
        dismissible={false}
        title={`${source} changed since this translation`}
        description={`The ${source} article was published with new words. Update the ${language} text, then publish it, or mark it up to date if nothing here needs to change.`}
        actions={[
          { label: `Open ${source}`, variant: "secondary", onClick: onOpenSource },
          { label: "Mark up to date", onClick: onReview },
        ]}
      />
    </div>
  );
}

/*
 * MissingTranslation is a language with no translation yet: what its
 * readers see meanwhile, and the two ways to start one.
 */
function MissingTranslation({
  source,
  language,
  hidden,
  onStart,
}: {
  source: string;
  language: string;
  hidden: boolean;
  onStart: (from: "source" | "blank") => void;
}) {
  return (
    <EmptyState
      icon={RiTranslate2}
      title={`Not translated into ${language} yet`}
      description={`Readers of ${language} ${hidden ? "don't see this article" : `see it in ${source}`} until you publish a translation.`}
    >
      <div className="flex items-center gap-2">
        <Button size="small" variant="secondary" onClick={() => onStart("blank")}>
          Start blank
        </Button>
        <Button size="small" onClick={() => onStart("source")}>
          Start from {source}
        </Button>
      </div>
    </EmptyState>
  );
}

/*
 * useTranslationWrites saves, publishes, unpublishes and marks up to date
 * one translation, keeping every list that shows it in step.
 */
function useTranslationWrites({
  workspace,
  id,
  locale,
  helpCenter,
  words,
  lockVersion,
}: {
  workspace: string;
  id: string;
  locale: string;
  helpCenter: string | undefined;
  words: Words | null;
  lockVersion: number | undefined;
}) {
  const queryClient = useQueryClient();
  const settle = (next: ArticleTranslation) => {
    queryClient.setQueryData(articleTranslationQuery(workspace, id, locale).queryKey, next);
    void queryClient.invalidateQueries({ queryKey: articleTranslationsQuery(workspace, id).queryKey });
    void queryClient.invalidateQueries({ queryKey: translationVersionsQuery(workspace, id, locale).queryKey });
    if (helpCenter) {
      void queryClient.invalidateQueries({
        queryKey: translationStatesQuery(workspace, helpCenter).queryKey,
      });
    }
  };
  const save = useMutation({
    mutationFn: async (publish: boolean) => {
      if (!words) {
        throw new Error("The translation is still loading.");
      }
      const saved = await saveArticleTranslation(workspace, id, locale, {
        title: words.title,
        slug: words.slug || undefined,
        description: words.description,
        body_markdown: words.body,
        lock_version: lockVersion,
      });
      return publish && (saved.status !== "published" || saved.has_unpublished_changes || saved.outdated)
        ? publishArticleTranslation(workspace, id, locale)
        : saved;
    },
    onSuccess: settle,
  });
  const unpublish = useMutation({
    mutationFn: () => unpublishArticleTranslation(workspace, id, locale),
    onSuccess: settle,
  });
  const review = useMutation({
    mutationFn: () => markTranslationCurrent(workspace, id, locale),
    onSuccess: (next) => {
      settle(next);
      toast.success("Marked up to date");
    },
  });

  return { settle, save, unpublish, review };
}

/*
 * startingWords is what a new translation starts from: the source's
 * text, or nothing.
 */
function startingWords(article: Article, from: "source" | "blank"): Words {
  if (from === "blank") {
    return { title: "", slug: "", description: "", body: "" };
  }
  return { title: article.title, slug: "", description: article.description ?? "", body: article.body_markdown ?? "" };
}

/*
 * draftOf lays the translation's words over the source's placing, as the
 * writer edits them.
 */
function draftOf(words: Words | null, article: Article): Draft | null {
  return words && { ...words, collection: article.collection_id ?? "", visibility: article.visibility };
}

function collectionLabel(collections: Collection[], id: string | null): string {
  return collectionOptions(collections).find((o) => o.id === id && o.id !== "")?.label ?? "";
}

/*
 * liveAddress is where readers of the language read the translation,
 * null while it is not published.
 */
function liveAddress(url: string, locale: string, current: ArticleTranslation | undefined): string | null {
  return current?.status === "published" ? `${url}/${locale}/articles/${current.slug}` : null;
}
