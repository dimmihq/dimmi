/*
 * What the inbox shows while its data is on the way: rows the shape of
 * the conversation list, a thread with bubbles on both sides, and the
 * customer's details, each as tall as what replaces it, so nothing moves
 * when the data lands.
 */
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { placeholders } from "@/features/settings/placeholders";
import { cx } from "@/utils/cx";

/*
 * ConversationRowsSkeleton stands in for the conversation list.
 */
export function ConversationRowsSkeleton({ rows = 6 }: { rows?: number }) {
  return (
    <ul aria-busy aria-label="Loading conversations" className="flex flex-col gap-1 px-[7px] pb-[7px]">
      {placeholders(rows).map((id) => (
        <li key={id} aria-hidden className="flex items-center gap-3 rounded-xl px-2.5 py-2.5">
          <Skeleton shape="circle" className="size-9 shrink-0" />
          <span className="flex min-w-0 flex-1 flex-col gap-2">
            <span className="flex items-center gap-2">
              <Skeleton shape="text" className="h-3.5 w-28" />
              <Skeleton shape="text" className="ms-auto h-3 w-10" />
            </span>
            <Skeleton shape="text" className="h-3 w-44 max-w-full" />
          </span>
        </li>
      ))}
    </ul>
  );
}

const bubbles = [
  { ours: false, width: "w-64" },
  { ours: true, width: "w-72" },
  { ours: false, width: "w-48" },
  { ours: true, width: "w-56" },
];

/*
 * ThreadSkeleton stands in for a conversation's messages: the
 * customer's on one side, the team's on the other.
 */
export function ThreadSkeleton() {
  return (
    <ol aria-busy aria-label="Loading messages" className="mx-auto flex w-full max-w-3xl flex-col gap-3 px-3 pt-4">
      {bubbles.map((b, i) => (
        <li key={`${b.width}-${b.ours}`} aria-hidden className={cx("flex flex-col gap-1.5", b.ours && "items-end")}>
          <Skeleton className={cx("h-14 max-w-[80%] rounded-[var(--radius-2-5xl)]", b.width, i === 0 && "h-20")} />
          <Skeleton shape="text" className="h-3 w-24" />
        </li>
      ))}
    </ol>
  );
}

/*
 * ConversationSkeleton stands in for a whole open conversation: its
 * header and its thread.
 */
export function ConversationSkeleton() {
  return (
    <div aria-busy className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 ps-4 pe-2.5 pt-[7px]">
        <Skeleton shape="text" className="h-4 w-56 max-w-[50%]" />
        <Skeleton className="h-5 w-16 rounded-full" />
        <span className="ms-auto flex gap-2">
          {placeholders(4).map((id) => (
            <Skeleton key={id} className="size-8 rounded-lg" />
          ))}
        </span>
      </div>
      <ThreadSkeleton />
    </div>
  );
}

/*
 * CustomerSkeleton stands in for the customer's details.
 */
export function CustomerSkeleton() {
  return (
    <div aria-busy className="flex flex-col gap-4 px-1 pt-1">
      <div className="flex items-center gap-3">
        <Skeleton shape="circle" className="size-9 shrink-0" />
        <span className="flex flex-1 flex-col gap-1.5">
          <Skeleton shape="text" className="h-3.5 w-32" />
          <Skeleton shape="text" className="h-3 w-20" />
        </span>
      </div>
      {placeholders(5).map((id) => (
        <div key={id} aria-hidden className="flex items-center justify-between gap-4">
          <Skeleton shape="text" className="h-3 w-20" />
          <Skeleton shape="text" className="h-3 w-32" />
        </div>
      ))}
    </div>
  );
}
