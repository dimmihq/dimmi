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
