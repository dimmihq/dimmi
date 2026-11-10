/*
 * The series page: every series with who it starts for, whether it is
 * running, how many people are in it right now, how many ever entered
 * and how many of those reached its goal. Totals across the live ones
 * sit on top. Opening a series opens its builder.
 */
import { RiAddLine, RiFlowChart } from "@remixicon/react";
import { useNavigate } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { Stat } from "@/features/broadcasts/parts";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { formatNumber } from "@/lib/i18n/format";

import { statusLook } from "./looks";
import { percent, type SeriesRow, seriesList } from "./sample";

function NameCell({ series }: { series: SeriesRow }) {
  return (
    <span className="flex min-w-0 max-w-80 items-center gap-3">
      <KindMark icon={RiFlowChart} tint="purple" />
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">{series.name}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{series.entry}</span>
      </span>
    </span>
  );
}

function Count({ value, ran }: { value: string; ran: boolean }) {
  return ran ? (
    <span className="text-body-2-regular text-text-primary tabular-nums">{value}</span>
  ) : (
    <span className="text-body-2-regular text-text-tertiary">–</span>
  );
}

const columns: ColumnDef<SeriesRow>[] = [
  { id: "series", header: "Series", cell: ({ row }) => <NameCell series={row.original} /> },
  {
    id: "status",
    header: "Status",
    cell: ({ row }) => {
      const look = statusLook[row.original.status];
      return (
        <span className="flex flex-col items-start gap-0.5">
          <Chip variant="caption" color={look.tint}>
            {look.name}
          </Chip>
          <span className="text-caption-1-regular text-text-tertiary">Edited {row.original.edited}</span>
        </span>
      );
    },
  },
  {
    id: "active",
    header: "In it now",
    cell: ({ row }) => <Count value={formatNumber(row.original.active)} ran={row.original.status === "live"} />,
  },
  {
    id: "entered",
    header: "Entered",
    cell: ({ row }) => <Count value={formatNumber(row.original.entered)} ran={row.original.entered > 0} />,
  },
  {
    id: "goal",
    header: "Reached goal",
    cell: ({ row }) => (
      <Count value={percent(row.original.goal, row.original.entered)} ran={row.original.entered > 0} />
    ),
  },
];

const widths = { series: "w-[40%]", status: "w-[18%]", active: "w-[14%]", entered: "w-[14%]", goal: "w-[14%]" };

function Totals() {
  const live = seriesList.filter((series) => series.status === "live");
  const entered = seriesList.reduce((sum, series) => sum + series.entered, 0);
  const goal = seriesList.reduce((sum, series) => sum + series.goal, 0);

  return (
    <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
      <Stat label="Live series" value={live.length} note={`of ${seriesList.length}`} />
      <Stat
        label="In a series now"
        value={formatNumber(live.reduce((sum, series) => sum + series.active, 0))}
        note="People mid-way"
      />
      <Stat label="Entered" value={formatNumber(entered)} note="Since each went live" />
      <Stat label="Reached the goal" value={percent(goal, entered)} note={`${formatNumber(goal)} people`} />
    </div>
  );
}

export function SeriesList() {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const needle = query.trim().toLowerCase();
  const rows = seriesList.filter((series) => needle === "" || series.name.toLowerCase().includes(needle));

  return (
    <Panel label="Series" ground className="h-full w-full flex-1">
      <PageHeader title="Series">
        <Button
          size="small"
          leadingIcon={RiAddLine}
          onClick={() => void toast.success("This is a preview of the page. Nothing was made.")}
        >
          New series
        </Button>
      </PageHeader>
      <ScrollArea ground>
        <div className="flex h-full flex-col gap-4 px-4 pb-4 animate-page-in">
          <Totals />
          <div className="min-h-[420px] flex-1">
            <DataTable
              aria-label="Series"
              data={rows}
              columns={columns}
              getRowId={(series) => series.id}
              total={`${formatNumber(rows.length)} series`}
              toolbar={
                <Input
                  aria-label="Search series"
                  placeholder="Search series"
                  value={query}
                  onChange={setQuery}
                  className="w-64"
                />
              }
              page={1}
              pageCount={1}
              onPageChange={() => undefined}
              onRowAction={(id) => void navigate({ to: "/d/series/$series", params: { series: id } })}
              widths={widths}
              empty={needle ? "No series matches." : "No series yet. Make one to welcome new signups."}
            />
          </div>
        </div>
      </ScrollArea>
    </Panel>
  );
}
