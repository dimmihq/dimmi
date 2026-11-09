/*
 * An article's version history, in a pane beside the writer: every kept
 * version, newest first, with what made it and when. Right-clicking a
 * version, or its restore button, puts it back into the working copy
 * after asking; readers see it once it is published again.
 */
import { RiArrowGoBackLine, RiCloseLine, RiHistoryLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useId, useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { ContextMenu, ContextMenuItem, ContextMenuTrigger } from "@/components/base/context-menu/context-menu";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Panel, ScrollArea } from "@/features/shell/page";
import { ListSkeleton } from "@/features/shell/table-skeleton";
import { type Article, type ArticleVersion, articleVersionsQuery, restoreArticleVersion } from "@/lib/api/help-centers";
import { formatDateTime } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

const kinds: Record<ArticleVersion["kind"], { text: string; color: ChipColor }> = {
  published: { text: "Published", color: "lime" },
  snapshot: { text: "Snapshot", color: "soft" },
  restored: { text: "Restored", color: "blue" },
  suggestion: { text: "Suggestion", color: "yellow" },
};

function VersionCard({ version, onRestore }: { version: ArticleVersion; onRestore: () => void }) {
  const restoreId = useId();

  return (
    <ContextMenuTrigger>
      {({ isOpen }) => (
        <>
          <li
            className={cx(
              "group flex flex-col gap-1 rounded-2xl border bg-background-secondary-default p-3 transition-colors",
              isOpen ? "border-border-button-active" : "border-border-button-default",
            )}
          >
            <span className="flex items-center justify-between gap-2">
              <span className="text-body-medium text-text-primary">Version {version.number}</span>
              <Chip variant="caption" color={kinds[version.kind].color}>
                {kinds[version.kind].text}
              </Chip>
            </span>
            <span className="truncate text-body-2-regular text-text-secondary">{version.title}</span>
            <span className="flex items-center justify-between gap-2">
              <span className="text-body-2-regular text-text-tertiary">{formatDateTime(version.created_at)}</span>
              <IconButton
                size="small"
                icon={RiArrowGoBackLine}
                aria-label={`Restore version ${version.number}`}
                className="opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
                onClick={onRestore}
              />
            </span>
          </li>
          <ContextMenu aria-label={`Version ${version.number}`} onAction={() => onRestore()}>
            <ContextMenuItem id={restoreId} icon={RiArrowGoBackLine}>
              Restore this version
            </ContextMenuItem>
          </ContextMenu>
        </>
      )}
    </ContextMenuTrigger>
  );
}

export function VersionHistory({
  workspace,
  article,
  onRestored,
  onClose,
  className,
}: {
  workspace: string;
  article: string;
  onRestored: (article: Article) => void;
  onClose: () => void;
  className?: string;
}) {
  const { data, isPending } = useQuery({ ...articleVersionsQuery(workspace, article), enabled: workspace !== "" });
  const [restoring, setRestoring] = useState<ArticleVersion | null>(null);
  const restore = useMutation({
    mutationFn: (version: ArticleVersion) => restoreArticleVersion(workspace, article, version.id),
    onSuccess: onRestored,
    onError: (error) => toast.error(error.message),
  });

  return (
    <Panel label="Version history" className={className}>
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5 pt-[7px]">
        <span className="flex min-w-0 items-center gap-2">
          <RiHistoryLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          <h2 className="truncate text-body-medium text-text-primary">Version history</h2>
        </span>
        <IconButton size="small" icon={RiCloseLine} aria-label="Close version history" onClick={onClose} />
      </header>
      <ScrollArea>
        <div className="flex flex-col gap-2 px-3 pt-2 pb-6">
          {isPending ? <ListSkeleton rows={4} /> : null}
          {data?.length === 0 ? (
            <p className="px-1 text-body-2-regular text-text-secondary">
              No versions yet. Each publish keeps one here.
            </p>
          ) : null}
          <ol aria-label="Versions" className="flex flex-col gap-2">
            {(data ?? []).map((v) => (
              <VersionCard key={v.id} version={v} onRestore={() => setRestoring(v)} />
            ))}
          </ol>
        </div>
      </ScrollArea>
      <AlertDialog
        isOpen={restoring !== null}
        onOpenChange={(open) => (open ? undefined : setRestoring(null))}
        variant="default"
        title={restoring ? `Restore version ${restoring.number}?` : "Restore this version?"}
        description="Its words, address and placing replace what you are editing. Readers see it once you publish, and every version is kept."
        confirmLabel="Restore"
        onConfirm={() => (restoring ? restore.mutateAsync(restoring) : undefined)}
      />
    </Panel>
  );
}
