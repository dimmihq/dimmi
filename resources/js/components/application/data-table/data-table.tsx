"use client";

import type { ReactNode } from "react";
import { flexRender, getCoreRowModel, useReactTable } from "@tanstack/react-table";
import type { ColumnDef, RowSelectionState, SortingState } from "@tanstack/react-table";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Pagination } from "@/components/base/pagination/pagination";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { ChevronSortDown } from "@/components/foundations/icons/chevrons";
import { cx } from "@/utils/cx";

/**
 * Data Table — the BoardUI data table block, taking real data. TanStack Table
 * owns the columns, selection and sort state; the rows, their order and the
 * page come from the caller, which asks the server for them (manual sorting
 * and pagination). The card holds a toolbar (the total on the start side,
 * the caller's filters, search and actions on the end side), an optional bar
 * for what to do with the selected rows, the table at normal size and the
 * numbered pagination footer. The card fills the height it is given: its
 * rows scroll inside it under a header that stays put, with the toolbar
 * above and the pagination below always in reach. The card sits on the
 * page ground surface.
 */

export interface DataTableProps<T> {
  "aria-label": string;
  data: T[];
  columns: ColumnDef<T>[];
  getRowId: (row: T) => string;
  /** The caption above the total, such as "Total results". */
  totalLabel?: ReactNode;
  /** The total itself, such as "1,204 people". */
  total: ReactNode;
  /** Filters, search and actions, at the toolbar's end. */
  toolbar?: ReactNode;
  /** A row under the toolbar, such as the chips of the filters in force. */
  filters?: ReactNode;
  /** Shown in place of the toolbar's end while rows are selected. */
  selectionBar?: (selected: string[], clear: () => void) => ReactNode;
  selection?: RowSelectionState;
  onSelectionChange?: (next: RowSelectionState) => void;
  sorting?: SortingState;
  onSortingChange?: (next: SortingState) => void;
  page: number;
  pageCount: number;
  onPageChange: (page: number) => void;
  /** Opens a row, by pointer or keyboard. */
  onRowAction?: (id: string) => void;
  /** Width classes by column id. */
  widths?: Record<string, string>;
  empty: ReactNode;
  loading?: boolean;
  skeleton?: ReactNode;
}

function SortChevron({ dir }: { dir: false | "asc" | "desc" }) {
  return (
    <ChevronSortDown
      className={cx(
        "size-6 shrink-0 transition-[transform,color] duration-150",
        dir === "asc" && "rotate-180",
        dir ? "text-text-secondary" : "text-text-tertiary",
      )}
    />
  );
}

export function DataTable<T>({
  "aria-label": label,
  data,
  columns,
  getRowId,
  totalLabel = "Total results",
  total,
  toolbar,
  filters,
  selectionBar,
  selection,
  onSelectionChange,
  sorting = [],
  onSortingChange,
  page,
  pageCount,
  onPageChange,
  onRowAction,
  widths = {},
  empty,
  loading = false,
  skeleton,
}: DataTableProps<T>) {
  const selectable = selection !== undefined && onSelectionChange !== undefined;
  const table = useReactTable({
    data,
    columns,
    getRowId,
    state: { sorting, rowSelection: selection ?? {} },
    manualSorting: true,
    manualPagination: true,
    pageCount,
    enableRowSelection: selectable,
    enableSortingRemoval: false,
    onSortingChange: (updater) => onSortingChange?.(typeof updater === "function" ? updater(sorting) : updater),
    onRowSelectionChange: (updater) =>
      onSelectionChange?.(typeof updater === "function" ? updater(selection ?? {}) : updater),
    getCoreRowModel: getCoreRowModel(),
  });

  const headers = table.getHeaderGroups()[0]?.headers ?? [];
  const rows = table.getRowModel().rows;
  const selected = Object.keys(selection ?? {}).filter((id) => selection?.[id]);
  const clear = () => onSelectionChange?.({});
  const first = headers[0]?.column.id;

  return (
    <section
      aria-label={label}
      className={cx(
        "flex h-full min-h-0 w-full flex-col overflow-hidden rounded-2xl border border-border-table pt-2",
      )}
    >
      <div className="flex min-h-12 w-full flex-col items-start gap-3 px-3 py-1 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col justify-center">
          <p className="text-body-medium whitespace-nowrap text-text-tertiary">{totalLabel}</p>
          {loading ? (
            <Skeleton shape="text" className="mt-1 h-3.5 w-24" />
          ) : (
            <p className="text-body-medium whitespace-nowrap text-text-primary">{total}</p>
          )}
        </div>
        <div className="-mx-3 flex w-[calc(100%+1.5rem)] items-center gap-2.5 overflow-x-auto px-3 sm:mx-0 sm:w-auto sm:flex-wrap sm:justify-end sm:overflow-visible sm:px-0">
          {selected.length > 0 && selectionBar ? selectionBar(selected, clear) : toolbar}
        </div>
      </div>

      {filters ? <div className="px-3 pt-2">{filters}</div> : null}

      <div className="mt-2 min-h-0 flex-1">
        <Table
          aria-label={label}
          size="md"
          selectionMode="none"
          onRowAction={(key) => onRowAction?.(String(key))}
          containerClassName="relative h-full overflow-auto"
        >
          <TableHeader>
            {headers.map((header) => {
              const id = header.column.id;
              const canSort = header.column.getCanSort();
              const title = flexRender(header.column.columnDef.header, header.getContext());
              const sortButton = canSort ? (
                <button
                  type="button"
                  onClick={header.column.getToggleSortingHandler()}
                  className="flex cursor-pointer items-center gap-0.5 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                >
                  {title}
                  <SortChevron dir={header.column.getIsSorted()} />
                </button>
              ) : (
                title
              );

              return (
                <TableColumn
                  key={header.id}
                  id={header.id}
                  isRowHeader={id === first}
                  className={cx("sticky top-0 z-10", widths[id])}
                >
                  {id === first && selectable ? (
                    <div className="flex items-center gap-2">
                      <Checkbox
                        slot={null}
                        aria-label="Select every row on this page"
                        isSelected={table.getIsAllPageRowsSelected()}
                        isIndeterminate={table.getIsSomePageRowsSelected() && !table.getIsAllPageRowsSelected()}
                        onChange={(on) => table.toggleAllPageRowsSelected(on)}
                      />
                      {sortButton}
                    </div>
                  ) : (
                    sortButton
                  )}
                </TableColumn>
              );
            })}
          </TableHeader>
          <TableBody
            renderEmptyState={() =>
              loading && skeleton ? (
                skeleton
              ) : (
                <div className="flex h-40 items-center justify-center text-body-medium text-text-tertiary">{empty}</div>
              )
            }
          >
            {rows.map((row) => (
              <TableRow
                key={row.id}
                id={row.id}
                className={cx(
                  onRowAction && "cursor-pointer hover:bg-background-secondary-hover",
                  row.getIsSelected() && "bg-background-secondary-default",
                )}
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id} className={widths[cell.column.id]}>
                    {cell.column.id === first && selectable ? (
                      <div className="flex min-w-0 items-center gap-2">
                        <Checkbox
                          slot={null}
                          aria-label="Select row"
                          isSelected={row.getIsSelected()}
                          onChange={(on) => row.toggleSelected(on)}
                        />
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </div>
                    ) : (
                      flexRender(cell.column.columnDef.cell, cell.getContext())
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {pageCount > 1 && (
        <div className="shrink-0 border-t border-separator-border px-3 py-3">
          <Pagination page={page} totalPages={pageCount} onChange={onPageChange} />
        </div>
      )}
    </section>
  );
}
