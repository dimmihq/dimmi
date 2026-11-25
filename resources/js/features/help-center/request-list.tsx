/*
 * The help center's change requests: each article Operator asked to
 * change, or to write, oldest first. Opening one shows the change inside
 * the article, to accept or decline there.
 */
import { RiArticleLine, RiFileAddLine } from "@remixicon/react";
import { Link } from "@tanstack/react-router";

import { Chip } from "@/components/base/badges/chip";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import type { Article } from "@/lib/api/help-centers";
import { formatRelative } from "@/lib/i18n/format";

import type { ArticleRequest } from "./change-requests";

const row =
  "flex items-center gap-3 rounded-xl px-2 py-2.5 outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring";

function RequestRow({ request, article }: { request: ArticleRequest; article: Article | undefined }) {
  const Icon = request.fresh ? RiFileAddLine : RiArticleLine;
  const body = (
    <>
      <Icon aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-regular text-text-primary">
          {article?.title ?? request.op.title ?? request.changeset.title}
        </span>
        <span className="truncate text-body-2-regular text-text-secondary">
          {request.changeset.reason || request.changeset.title} · {formatRelative(request.changeset.created_at)}
        </span>
      </span>
      <Chip variant="caption" color={request.fresh ? "lime" : "blue"}>
        {request.fresh ? "New" : "Edit"}
      </Chip>
    </>
  );
  if (request.fresh) {
    return (
      <Link to="/d/help-center/requests/$changeset" params={{ changeset: request.changeset.id }} className={row}>
        {body}
      </Link>
    );
  }
  return (
    <Link to="/d/help-center/$article" params={{ article: request.op.article_id ?? "" }} className={row}>
      {body}
    </Link>
  );
}

export function RequestList({
  requests,
  articles,
  title,
  chips,
}: {
  requests: ArticleRequest[];
  articles: Article[];
  title: string;
  chips: React.ReactNode;
}) {
  return (
    <Panel label="Change requests" className="h-full w-full flex-1">
      <PageHeader title={title} />
      {chips}
      <ScrollArea>
        <div className="flex animate-page-in flex-col gap-1 px-2 pb-4">
          {requests.length === 0 ? (
            <p className="px-2 py-6 text-body-2-regular text-text-tertiary">
              No change requests. When a routine asks to change or write an article, it waits here.
            </p>
          ) : null}
          {requests.map((r) => (
            <RequestRow key={r.changeset.id} request={r} article={articles.find((a) => a.id === r.op.article_id)} />
          ))}
        </div>
      </ScrollArea>
    </Panel>
  );
}
