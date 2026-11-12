/*
 * Where a feature request stands, drawn the same way everywhere: a dot
 * in its status's color, by the workspace's own statuses, or the
 * approval state while it waits.
 */
import { useAccount } from "@/features/settings/use-account";
import type { FeedbackPost } from "@/lib/api/feedback";

import { useStatuses } from "./use-statuses";

export function StatusDot({ color }: { color: string }) {
  return <span aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ background: color }} />;
}

export function StatusChip({ post }: { post: FeedbackPost }) {
  const list = useStatuses(useAccount().workspace.id);
  const status = list.find((s) => s.id === post.status);
  const label =
    post.moderation === "pending" ? "Waiting for approval" : post.moderation === "spam" ? "Spam" : status?.name;

  return (
    <span className="inline-flex h-6 items-center gap-1.5 rounded-full bg-background-secondary-default px-2.5 text-caption-1-medium text-text-secondary">
      <StatusDot color={post.moderation === "approved" ? (status?.color ?? "#a1a1aa") : "#eab308"} />
      {label ?? post.status}
    </span>
  );
}
