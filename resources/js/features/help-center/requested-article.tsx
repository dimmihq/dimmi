/*
 * A new article Operator asked to write, before anyone accepted it: laid
 * out like any article, every line green since all of it is new, and
 * still editable. Accept changes writes it as a draft, keeping whatever
 * was edited here, and opens it in the editor; Decline drops it.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { articleQuery, defaultBrand, helpCentersQuery, saveArticle } from "@/lib/api/help-centers";

import { ArticlePage } from "./article-page";
import { type ArticleRequest, useArticleRequests } from "./change-requests";
import type { Draft } from "./docs-format";
import { RequestNote } from "./request-note";

/*
 * draftOf is a requested article as the page edits it.
 */
function draftOf(request: ArticleRequest): Draft {
  return {
    title: request.op.title ?? "",
    slug: "",
    description: "",
    body: request.op.text ?? "",
    collection: "",
    visibility: request.op.visibility ?? "public",
  };
}

export function RequestedArticle({ id }: { id: string }) {
  const workspace = useAccount().workspace.id;
  const requests = useArticleRequests(workspace);
  const request = requests.requests.find((r) => r.changeset.id === id && r.fresh) ?? null;

  if (!request) {
    return (
      <Panel label="Requested article" className="h-full flex-1">
        <PageHeader title="Requested article" />
        <div className="m-auto flex flex-col items-center gap-2 p-8 text-center">
          <p className="text-body-medium text-text-primary">This request was already decided</p>
          <Link to="/d/help-center" search={{ collection: "requests" }} className="text-body-2-regular underline">
            Back to change requests
          </Link>
        </div>
      </Panel>
    );
  }
  return <Requested workspace={workspace} request={request} requests={requests} />;
}

function Requested({
  workspace,
  request,
  requests,
}: {
  workspace: string;
  request: ArticleRequest;
  requests: ReturnType<typeof useArticleRequests>;
}) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((c) => c.id === request.op.help_center_id) ?? centers[0];
  const [draft, setDraft] = useState<Draft>(() => draftOf(request));
  const edited = JSON.stringify(draft) !== JSON.stringify(draftOf(request));

  const decide = useMutation({
    mutationFn: async (accepting: boolean) => {
      if (!accepting) {
        await requests.decline(request);
        return null;
      }
      const changeset = await requests.accept(request);
      const created = changeset.operations.find((o) => o.kind === "article.create_draft")?.created_id;
      if (created && edited) {
        const article = await queryClient.fetchQuery({ ...articleQuery(workspace, created), staleTime: 0 });
        await saveArticle(workspace, created, {
          title: draft.title,
          description: draft.description,
          body_markdown: draft.body,
          lock_version: article.lock_version,
        });
      }
      return created ?? null;
    },
    onSuccess: (created, accepting) => {
      toast.success(accepting ? "Article drafted. Publish it when it reads right." : "Request declined.");
      void (created
        ? navigate({ to: "/d/help-center/$article", params: { article: created } })
        : navigate({ to: "/d/help-center", search: { collection: "requests" } }));
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Panel label="Requested article" className="h-full w-full flex-1">
      <PageHeader
        title={
          <span className="flex items-center gap-2">
            <Link
              to="/d/help-center"
              search={{ collection: "requests" }}
              aria-label="Back to change requests"
              className="rounded-md p-1 text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftLine aria-hidden className="size-4" />
            </Link>
            {draft.title || "Untitled article"}
            <Chip variant="caption" color="lime">
              New
            </Chip>
          </span>
        }
      >
        <Button size="small" variant="secondary" onClick={() => decide.mutate(false)} disabled={decide.isPending}>
          Decline
        </Button>
        <Button size="small" onClick={() => decide.mutate(true)} loading={decide.isPending}>
          Accept changes
        </Button>
      </PageHeader>
      <ScrollArea>
        <RequestNote request={request} waiting={1} />
        <ArticlePage
          workspace={workspace}
          draft={draft}
          brand={center?.settings.primary_color || defaultBrand}
          collection=""
          revision={0}
          failure={null}
          onChange={(change) => setDraft({ ...draft, ...change })}
          onPickCollection={() => undefined}
          proposed={{ title: null, body: request.op.text ?? "", whole: true }}
        />
      </ScrollArea>
    </Panel>
  );
}
