/*
 * The article's header: back to the list, its title and state, and what
 * can be done with it. Publish and Unpublish ask first; history and
 * settings are icons.
 */
import { RiArrowLeftLine, RiExternalLinkLine, RiHistoryLine, RiSettings3Line } from "@remixicon/react";
import { Link } from "@tanstack/react-router";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { PageHeader } from "@/features/shell/page";
import type { Article } from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { articleState } from "./docs-format";

export function ArticleHeader({
  article,
  live,
  saving,
  history,
  onSettings,
  onSave,
  onConfirm,
}: {
  article: Article;
  live: string | null;
  saving: boolean;
  history: { shown: boolean; toggle: () => void };
  onSettings: () => void;
  onSave: () => void;
  onConfirm: (what: "publish" | "unpublish") => void;
}) {
  return (
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
          {article.title || "Untitled article"}
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
        <Button size="small" variant="secondary" onClick={() => onConfirm("unpublish")}>
          Unpublish
        </Button>
      ) : null}
      <IconButton
        size="small"
        icon={RiHistoryLine}
        aria-label="Version history"
        aria-pressed={history.shown}
        className={cx(history.shown && "border-border-button-active bg-background-primary-active")}
        onClick={history.toggle}
      />
      <IconButton size="small" icon={RiSettings3Line} aria-label="Article settings" onClick={onSettings} />
      <Button size="small" variant="secondary" onClick={onSave} disabled={saving}>
        Save draft
      </Button>
      <Button size="small" onClick={() => onConfirm("publish")} disabled={saving}>
        {saving ? "Saving…" : "Publish"}
      </Button>
    </PageHeader>
  );
}
