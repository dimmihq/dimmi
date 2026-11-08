/*
 * Writing one article: its title, description, collection, who may read
 * it, the audience it is kept to and its Markdown body. Saving keeps a working copy readers never
 * see; Publish freezes it into the live version. A save made over
 * someone else's newer one is refused and says so, so nobody's words are
 * lost. Unpublishing keeps every version.
 */
import { RiArrowLeftLine, RiExternalLinkLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Notification } from "@/components/base/notification/notification";
import { Textarea } from "@/components/base/textarea/textarea";
import { Choice } from "@/features/settings/controls";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import {
  type Article,
  type ArticleChange,
  articleQuery,
  articlesQuery,
  collectionsQuery,
  helpCentersQuery,
  publishArticle,
  saveArticle,
  setArticleStatus,
} from "@/lib/api/help-centers";

import { ArticleAudience } from "./article-audience";
import { articleState, collectionOptions } from "./docs-format";

const visibilities = { Public: "public", "Agent only": "agent_only" } as const;

interface Draft {
  title: string;
  slug: string;
  description: string;
  body: string;
  collection: string;
  visibility: "public" | "agent_only";
}

function draftOf(article: Article): Draft {
  return {
    title: article.title,
    slug: article.slug,
    description: article.description ?? "",
    body: article.body_markdown ?? "",
    collection: article.collection_id ?? "",
    visibility: article.visibility,
  };
}

function changeOf(draft: Draft, article: Article): ArticleChange {
  return {
    title: draft.title,
    slug: draft.slug,
    description: draft.description,
    body_markdown: draft.body,
    visibility: draft.visibility,
    collection_id: draft.collection,
    lock_version: article.lock_version,
  };
}

export function ArticleEditor({ id }: { id: string }) {
  const workspace = useAccount().workspace.id;
  const enabled = workspace !== "";
  const queryClient = useQueryClient();
  const { data: article } = useQuery({ ...articleQuery(workspace, id), enabled });
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled });
  const center = centers.find((c) => c.id === article?.help_center_id);
  const { data: collections = [] } = useQuery({
    ...collectionsQuery(workspace, article?.help_center_id ?? ""),
    enabled: Boolean(article),
  });
  const [draft, setDraft] = useState<Draft | null>(null);

  useEffect(() => {
    if (article && draft === null) {
      setDraft(draftOf(article));
    }
  }, [article, draft]);

  const settle = (next: Article) => {
    queryClient.setQueryData(articleQuery(workspace, id).queryKey, next);
    void queryClient.invalidateQueries({ queryKey: articlesQuery(workspace, next.help_center_id).queryKey });
  };
  const save = useMutation({
    mutationFn: async (publish: boolean) => {
      if (!article || !draft) {
        throw new Error("The article is still loading.");
      }
      const saved = await saveArticle(workspace, id, changeOf(draft, article));
      return publish && (saved.status !== "published" || saved.has_unpublished_changes)
        ? publishArticle(workspace, id)
        : saved;
    },
    onSuccess: settle,
  });
  const unpublish = useMutation({ mutationFn: () => setArticleStatus(workspace, id, "draft"), onSuccess: settle });

  if (!article || !draft) {
    return (
      <Panel label="Article" className="h-full flex-1">
        <PageHeader title="Article" />
      </Panel>
    );
  }

  const options = collectionOptions(collections);
  const set = (change: Partial<Draft>) => setDraft({ ...draft, ...change });
  const failure = save.error ?? unpublish.error;
  const live = article.status === "published" && center ? `${center.url}/articles/${article.slug}` : null;

  return (
    <Panel label="Article" className="h-full flex-1">
      <PageHeader
        title={
          <span className="flex items-center gap-2">
            <Link
              to="/d/docs"
              aria-label="Back to docs"
              className="rounded-md p-1 text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftLine aria-hidden className="size-4" />
            </Link>
            {article.title}
            <Chip variant="caption" color={article.status === "published" ? "lime" : "gray"}>
              {articleState(article)}
            </Chip>
          </span>
        }
      >
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
        {article.status === "published" ? (
          <Button size="small" variant="secondary" onClick={() => unpublish.mutate()} disabled={unpublish.isPending}>
            Unpublish
          </Button>
        ) : null}
        <Button size="small" variant="secondary" onClick={() => save.mutate(false)} disabled={save.isPending}>
          Save draft
        </Button>
        <Button size="small" onClick={() => save.mutate(true)} disabled={save.isPending}>
          {save.isPending ? "Saving…" : "Publish"}
        </Button>
      </PageHeader>
      <ScrollArea>
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-4 px-4 pb-8">
          {failure ? (
            <Notification status="error" title={failure.message} dismissible={false} role="alert" className="w-full" />
          ) : null}
          <Input label="Title" value={draft.title} onChange={(title) => set({ title })} maxLength={150} />
          <Input
            label="Description"
            value={draft.description}
            onChange={(description) => set({ description })}
            maxLength={300}
            hint="Shown in search results and under the title."
          />
          <div className="flex flex-wrap items-end gap-4">
            <div className="flex flex-col gap-1.5">
              <span className="text-body-medium text-text-secondary">Collection</span>
              <Choice
                label="Collection"
                options={options.map((o) => o.label)}
                value={options.find((o) => o.id === draft.collection)?.label ?? "No collection"}
                onChange={(label) => set({ collection: options.find((o) => o.label === label)?.id ?? "" })}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <span className="text-body-medium text-text-secondary">Who can read it</span>
              <Choice
                label="Who can read it"
                options={Object.keys(visibilities)}
                value={draft.visibility === "public" ? "Public" : "Agent only"}
                onChange={(label) => set({ visibility: visibilities[label as keyof typeof visibilities] })}
              />
            </div>
            {article ? <ArticleAudience workspace={workspace} article={article} onSaved={settle} /> : null}
            <Input
              label="Slug"
              value={draft.slug}
              onChange={(slug) => set({ slug })}
              maxLength={120}
              inputDir="ltr"
              className="min-w-48 flex-1"
            />
          </div>
          <Textarea
            label="Article"
            value={draft.body}
            onChange={(body) => set({ body })}
            rows={24}
            fieldClassName="font-mono"
            hint="Markdown: ## headings, lists, `code`, tables and > [!NOTE] callouts."
          />
        </div>
      </ScrollArea>
    </Panel>
  );
}
