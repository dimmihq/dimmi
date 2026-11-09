/*
 * Where a feature request stands, drawn the same way everywhere: a dot
 * in the status's color, or the approval state while it waits.
 */
import { type FeedbackPost, statuses } from "@/lib/api/feedback";

export function StatusDot({ color }: { color: string }) {
  return <span aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ background: color }} />;
}

export function StatusChip({ post }: { post: FeedbackPost }) {
  const status = statuses.find((s) => s.id === post.status) ?? statuses[0];
  const label =
    post.moderation === "pending" ? "Waiting for approval" : post.moderation === "spam" ? "Spam" : status?.name;

  return (
    <span className="inline-flex h-6 items-center gap-1.5 rounded-full bg-background-secondary-default px-2.5 text-caption-1-medium text-text-secondary">
      <StatusDot color={post.moderation === "approved" ? (status?.color ?? "#a1a1aa") : "#eab308"} />
      {label}
    </span>
  );
}
