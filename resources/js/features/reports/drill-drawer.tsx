/*
 * The conversations behind a number: a drawer listing them a page at a
 * time, each linking to the conversation in the inbox, with the value
 * it added to the number. The list stops at ten thousand.
 */
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Drawer, DrawerBody, DrawerHeader } from "@/components/base/drawer/drawer";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import {
  drillPageSize,
  type Report,
  type ReportConversation,
  type ReportUnit,
  reportConversationsQuery,
} from "@/lib/api/reports";
import { formatMonthDay, formatNumber, formatTime } from "@/lib/i18n/format";
import { formatRange, formatValue } from "./format";
import { channelOptions } from "./options";
import { drillFilters, parseDrillBy, type ReportSearch } from "./search";

/*
 * drillTitle finds what the drill counts, its unit, and the row it was
 * narrowed to, from the report it was opened on.
 */
function drillTitle(
  report: Report | undefined,
  search: ReportSearch,
): { label: string; unit: ReportUnit; row?: string } {
  const drill = search.drill ?? "";
  const by = parseDrillBy(search.drill_by);
  const byKeys = Object.keys(by);
  const row = byKeys.length
    ? report?.charts
        .flatMap((c) => c.rows)
        .find((r) => byKeys.every((k) => r.filter[k] === by[k]) && Object.keys(r.filter).length === byKeys.length)
        ?.label
    : undefined;
  const metric = report?.metrics.find((m) => m.drill === drill);
  if (metric) {
    return { label: metric.label, unit: metric.unit, row };
  }
  for (const chart of report?.charts ?? []) {
    const series = chart.series.find((s) => s.drill === drill);
    if (series) {
      return {
        label: chart.series.length > 1 ? `${chart.title}: ${series.label}` : chart.title,
        unit: chart.unit,
        row,
      };
    }
    if (chart.drill === drill) {
      return { label: chart.title, unit: chart.unit, row };
    }
  }

  return { label: "Conversations", unit: "count", row };
}

const statusNames: Record<string, string> = { open: "Open", snoozed: "Snoozed", closed: "Closed" };

function columns(unit: ReportUnit): ColumnDef<ReportConversation>[] {
  const valued = unit !== "count";

  return [
    {
      id: "conversation",
      header: "Conversation",
      cell: ({ row }) => (
        <span className="flex min-w-44 flex-col sm:min-w-56">
          <Link
            to="/d/inbox/$view/$conversation"
            params={{ view: "all", conversation: row.original.id }}
            className="truncate rounded-md text-body-2-medium text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            #{row.original.number} {row.original.subject ?? ""}
          </Link>
          <span className="truncate text-caption-1-regular text-text-tertiary">
            {channelOptions.find((c) => c.id === row.original.channel)?.label ?? row.original.channel}
            <span className="sm:hidden"> · {statusNames[row.original.status] ?? row.original.status}</span>
          </span>
        </span>
      ),
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => (
        <span className="text-body-2-regular text-text-secondary">
          {statusNames[row.original.status] ?? row.original.status}
        </span>
      ),
    },
    {
      id: "started",
      header: "Started",
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-body-2-regular text-text-secondary">
          {formatMonthDay(row.original.started_at)}, {formatTime(row.original.started_at)}
        </span>
      ),
    },
    {
      id: "closed",
      header: "Closed",
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-body-2-regular text-text-secondary">
          {row.original.closed_at
            ? `${formatMonthDay(row.original.closed_at)}, ${formatTime(row.original.closed_at)}`
            : "—"}
        </span>
      ),
    },
    ...(valued
      ? [
          {
            id: "value",
            header: () => <span className="block text-end">Value</span>,
            cell: ({ row }: { row: { original: ReportConversation } }) => (
              <span className="block text-end text-body-2-medium whitespace-nowrap text-text-primary tabular-nums">
                {formatValue(unit, row.original.value)}
              </span>
            ),
          } satisfies ColumnDef<ReportConversation>,
        ]
      : []),
  ];
}

const widths = {
  conversation: "w-full max-w-0",
  status: "hidden sm:table-cell",
  started: "hidden md:table-cell",
  closed: "hidden lg:table-cell",
  value: "w-24",
};

function DrillList({
  workspace,
  reportKey,
  report,
  search,
}: {
  workspace: string;
  reportKey: string;
  report: Report | undefined;
  search: ReportSearch;
}) {
  const [page, setPage] = useState(1);
  const drill = search.drill ?? "";
  const live = report?.live ?? false;
  const { label, unit, row } = drillTitle(report, search);
  const list = useQuery({
    ...reportConversationsQuery(workspace, reportKey, drill, drillFilters(search, live), (page - 1) * drillPageSize),
    enabled: workspace !== "" && drill !== "",
  });
  const total = list.data?.total ?? 0;
  const from = search.drill_from ?? report?.from;
  const to = search.drill_to ?? report?.to;
  const range = live ? "Now" : from && to ? formatRange(from, to) : "";
  const shown = list.data?.capped ? `Showing first ${formatNumber(10_000)}` : `${formatNumber(total)} conversations`;

  return (
    <>
      <DrawerHeader title={label} description={[row, range].filter(Boolean).join(" · ") || undefined} />
      <DrawerBody className="flex flex-col pb-4">
        <div className="min-h-[24rem] flex-1">
          <DataTable
            aria-label={label}
            data={list.data?.data ?? []}
            columns={columns(unit)}
            getRowId={(c) => c.id}
            totalLabel="Conversations"
            total={list.isError ? list.error.message : shown}
            page={page}
            pageCount={Math.ceil(Math.min(total, 10_000) / drillPageSize)}
            onPageChange={setPage}
            widths={widths}
            loading={list.isPending}
            skeleton={<TableSkeleton />}
            empty="No conversations."
          />
        </div>
      </DrawerBody>
    </>
  );
}

export function DrillDrawer({
  workspace,
  reportKey,
  report,
  search,
  onClose,
}: {
  workspace: string;
  reportKey: string;
  report: Report | undefined;
  search: ReportSearch;
  onClose: () => void;
}) {
  const open = search.drill !== undefined;

  return (
    <Drawer
      size="lg"
      aria-label="Conversations"
      isOpen={open}
      onOpenChange={(next) => {
        if (!next) {
          onClose();
        }
      }}
    >
      {open ? (
        <DrillList
          key={`${search.drill}|${search.drill_from}|${search.drill_to}|${search.drill_by}`}
          workspace={workspace}
          reportKey={reportKey}
          report={report}
          search={search}
        />
      ) : null}
    </Drawer>
  );
}
