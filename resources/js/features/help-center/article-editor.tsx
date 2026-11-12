/*
 * Writing one article, laid out the way readers will see it. Where it
 * is filed, who may read it, the audience it is kept to and its address
 * sit in Settings. Saving keeps a working copy readers never see;
 * Publish freezes it into the live version, and the history beside the
 * article can put any kept version back. A save made over someone
 * else's newer one is refused and says so, so nobody's words are lost.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import {
  type Article,
  type ArticleChange,
  articleQuery,
  articlesQuery,
  articleVersionsQuery,
  collectionsQuery,
  defaultBrand,
  deleteArticle,
  helpCentersQuery,
  publishArticle,
  saveArticle,
  setArticleStatus,
} from "@/lib/api/help-centers";

import { ArticleHeader } from "./article-header";
import { ArticlePage } from "./article-page";
import { ArticleSettings } from "./article-settings";
import { collectionOptions, type Draft } from "./docs-format";
import { useHistoryPane } from "./history-pane";

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
  const navigate = useNavigate();
  const { data: article } = useQuery({ ...articleQuery(workspace, id), enabled });
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled });
  const center = centers.find((c) => c.id === article?.help_center_id);
  const { data: collections = [] } = useQuery({
    ...collectionsQuery(workspace, article?.help_center_id ?? ""),
    enabled: Boolean(article),
  });
  const [draft, setDraft] = useState<Draft | null>(null);
  const [settings, setSettings] = useState(false);
  const [revision, setRevision] = useState(0);
  const [confirm, setConfirm] = useState<"publish" | "unpublish" | null>(null);

  useEffect(() => {
    if (article && draft === null) {
      setDraft(draftOf(article));
    }
  }, [article, draft]);

  const settle = (next: Article) => {
    queryClient.setQueryData(articleQuery(workspace, id).queryKey, next);
    void queryClient.invalidateQueries({ queryKey: articleVersionsQuery(workspace, id).queryKey });
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
  const history = useHistoryPane(workspace, id, (restored) => {
    settle(restored);
    setDraft(draftOf(restored));
    setRevision((r) => r + 1);
  });

  if (!article || !draft) {
    return (
      <Panel label="Article" className="h-full flex-1">
        <PageHeader title="Article" />
      </Panel>
    );
  }

  const options = collectionOptions(collections);
  const collectionName = options.find((o) => o.id === draft.collection && o.id !== "")?.label ?? "";
  const set = (change: Partial<Draft>) => setDraft({ ...draft, ...change });
  const failure = save.error ?? unpublish.error;
  const live = article.status === "published" && center ? `${center.url}/articles/${article.slug}` : null;

  return history.layout(
    <Panel label="Article" className="h-full w-full flex-1">
      <ArticleHeader
        article={article}
        live={live}
        saving={save.isPending}
        history={history}
        onSettings={() => setSettings(true)}
        onSave={() => save.mutate(false, { onSuccess: () => toast.success("Draft saved") })}
        onConfirm={setConfirm}
      />
      <ScrollArea>
        <ArticlePage
          workspace={workspace}
          draft={draft}
          brand={center?.settings.primary_color || defaultBrand}
          collection={collectionName}
          revision={revision}
          failure={failure}
          onChange={set}
          onPickCollection={() => setSettings(true)}
        />
      </ScrollArea>
      <AlertDialog
        isOpen={confirm === "publish"}
        onOpenChange={(open) => (open ? undefined : setConfirm(null))}
        variant="default"
        title={article.status === "published" ? "Publish your changes?" : "Publish this article?"}
        description={
          article.status === "published"
            ? "Readers see the new version right away. The one they see now stays in the history."
            : "Readers can find it on the help center right away, and dimmi answers from it."
        }
        confirmLabel="Publish"
        onConfirm={() => save.mutateAsync(true)}
      />
      <AlertDialog
        isOpen={confirm === "unpublish"}
        onOpenChange={(open) => (open ? undefined : setConfirm(null))}
        title="Unpublish this article?"
        description="Readers and dimmi stop seeing it right away. It goes back to a draft and every version is kept."
        confirmLabel="Unpublish"
        onConfirm={() => unpublish.mutateAsync()}
      />
      {settings ? (
        <ArticleSettings
          workspace={workspace}
          article={article}
          draft={draft}
          collections={collections}
          onChange={set}
          onSaved={settle}
          onDelete={async () => {
            await deleteArticle(workspace, article.id);
            await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "help-centers"] });
            toast.success("Article deleted");
            await navigate({ to: "/d/help-center" });
          }}
          onClose={() => setSettings(false)}
        />
      ) : null}
    </Panel>,
  );
}
