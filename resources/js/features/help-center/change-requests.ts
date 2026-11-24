/*
 * Change requests on the help center: articles Operator asked to change,
 * and new articles it asked to write, each waiting for a teammate. They
 * are reviewed in the article itself, drawn inside the writer, and
 * accepted or declined from its header. The help center lists them under
 * Change requests. Dry run previews never show up here.
 */
import { useMutation } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";
import { useSuggestions } from "@/features/operator/suggestions";
import type { OperatorChangeset, OperatorOperation } from "@/lib/api/operator";

export interface ArticleRequest {
  changeset: OperatorChangeset;
  op: OperatorOperation;
  // fresh is a request to write a new article.
  fresh: boolean;
}

const articleKinds = new Set(["article.create_draft", "article.edit_draft"]);

/*
 * requestsOf is the article requests among pending changesets, oldest
 * first, one per changeset.
 */
export function requestsOf(changesets: OperatorChangeset[]): ArticleRequest[] {
  const out: ArticleRequest[] = [];
  for (const changeset of changesets) {
    const op = changeset.operations.find((o) => articleKinds.has(o.kind));
    if (op) {
      out.push({ changeset, op, fresh: op.kind === "article.create_draft" });
    }
  }
  return out.sort((a, b) => a.changeset.created_at.localeCompare(b.changeset.created_at));
}

/*
 * useArticleRequests is the workspace's article requests and how to
 * accept or decline one.
 */
export function useArticleRequests(workspace: string) {
  const { suggestions, decide } = useSuggestions(workspace, (op) => articleKinds.has(op.kind));
  const requests = requestsOf(suggestions);
  return {
    requests,
    forArticle: (article: string) => requests.find((r) => !r.fresh && r.op.article_id === article) ?? null,
    accept: (r: ArticleRequest) => decide(r.changeset.id, "applications"),
    decline: (r: ArticleRequest) => decide(r.changeset.id, "declines"),
  };
}

/*
 * useCenterRequests is the requests on one help center's articles.
 */
export function useCenterRequests(workspace: string, center: string, articles: { id: string }[]) {
  return useArticleRequests(workspace).requests.filter((r) =>
    r.fresh ? r.op.help_center_id === center : articles.some((a) => a.id === r.op.article_id),
  );
}

/*
 * useArticleDecision accepts or declines the request on an article,
 * then hands the accepted article on to be loaded again.
 */
export function useArticleDecision(workspace: string, article: string, onAccepted: () => Promise<void>) {
  const requests = useArticleRequests(workspace);
  const request = requests.forArticle(article);
  const decide = useMutation({
    mutationFn: async (accepting: boolean) => {
      if (!request) {
        return;
      }
      await (accepting ? requests.accept(request) : requests.decline(request));
      if (accepting) {
        await onAccepted();
      }
    },
    onSuccess: (_, accepting) =>
      toast.success(accepting ? "Changes accepted into the draft. Publish when it reads right." : "Request declined."),
    onError: (error) => toast.error(error.message),
  });
  const waiting = requests.requests.filter((r) => !r.fresh && r.op.article_id === article).length;
  return { request, waiting, decide };
}
