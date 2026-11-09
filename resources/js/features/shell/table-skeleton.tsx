/*
 * Stand-ins for tables and record pages while their data loads: bars the
 * height of a row, so a table never says it is empty before it knows,
 * and a record keeps its shape until its details arrive.
 */
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { placeholders } from "@/features/settings/placeholders";

/*
 * TableSkeleton fills a table's body while its rows load.
 */
export function TableSkeleton({ rows = 6 }: { rows?: number }) {
  return (
    <div aria-busy aria-label="Loading" role="status" className="flex flex-col">
      {placeholders(rows).map((id) => (
        <div key={id} aria-hidden className="flex h-11 items-center gap-6 border-b border-border-table last:border-b-0">
          <Skeleton shape="text" className="h-3.5 w-40" />
          <Skeleton shape="text" className="h-3 w-28" />
          <Skeleton shape="text" className="ms-auto h-3 w-20" />
        </div>
      ))}
    </div>
  );
}

/*
 * ProfileSkeleton stands in for a record's page: its name and a column
 * of details beside a list.
 */
export function ProfileSkeleton() {
  return (
    <div aria-busy className="flex flex-col gap-6 px-4 pt-2 pb-4">
      <div className="flex items-center gap-3">
        <Skeleton shape="circle" className="size-12 shrink-0" />
        <span className="flex flex-col gap-2">
          <Skeleton shape="text" className="h-5 w-44" />
          <Skeleton shape="text" className="h-3 w-28" />
        </span>
      </div>
      <TableSkeleton rows={5} />
    </div>
  );
}

/*
 * ListSkeleton fills a card's list, such as a record's conversations,
 * while it loads.
 */
export function ListSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div aria-busy aria-label="Loading" role="status" className="flex flex-col">
      {placeholders(rows).map((id) => (
        <div key={id} aria-hidden className="flex h-9 items-center gap-3">
          <Skeleton shape="text" className="h-3 w-10" />
          <Skeleton shape="text" className="h-3.5 flex-1" />
          <Skeleton shape="text" className="h-3 w-14" />
        </div>
      ))}
    </div>
  );
}

/*
 * TitleSkeleton stands in for a record's avatar and name in a drawer's
 * header.
 */
export function TitleSkeleton() {
  return (
    <span aria-busy className="flex items-center gap-2.5">
      <Skeleton shape="circle" className="size-8 shrink-0" />
      <span className="flex flex-col gap-1.5">
        <Skeleton shape="text" className="h-4 w-36" />
        <Skeleton shape="text" className="h-3 w-24" />
      </span>
    </span>
  );
}

/*
 * EditorSkeleton stands in for a full-page editor: its header and two
 * columns of cards.
 */
export function EditorSkeleton() {
  return (
    <div aria-busy aria-label="Loading" role="status" className="flex h-full flex-col gap-4 p-4">
      <div className="flex items-center justify-between">
        <Skeleton shape="text" className="h-5 w-48" />
        <Skeleton shape="text" className="h-8 w-24" />
      </div>
      <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,22rem)]">
        <div className="flex flex-col gap-3">
          {placeholders(4).map((id) => (
            <Skeleton key={id} shape="rect" className="h-16 w-full rounded-2xl" />
          ))}
        </div>
        <Skeleton shape="rect" className="h-64 w-full rounded-2xl" />
      </div>
    </div>
  );
}
