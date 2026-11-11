/*
 * The series page: every series with who it starts for, whether it is
 * running, how many people are on it right now, how many ever entered
 * and how many of those reached its goal. Totals across the live ones
 * sit on top. A new series starts from a template, for contacts or for
 * whole accounts, and opens in its builder.
 */
import { RiAddLine, RiArrowRightSLine, RiBuilding2Line, RiFlowChart, RiUser3Line } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { Stat } from "@/features/broadcasts/parts";
import { useWorkspace } from "@/features/customers/names";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { createSeries, type Series, seriesListQuery } from "@/lib/api/series";
import { formatDate, formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { percent, statusLook } from "./looks";
import { summary, templateSpec, templates } from "./model";

function NameCell({ series }: { series: Series }) {
  const entry = series.spec.nodes.find((n) => n.kind === "entry");

  return (
    <span className="flex min-w-0 max-w-80 items-center gap-3">
      <KindMark icon={series.subject === "company" ? RiBuilding2Line : RiFlowChart} tint="purple" />
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">{series.name}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">
          {entry ? summary(entry, series.spec) : ""}
        </span>
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

const columns: ColumnDef<Series>[] = [
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
          <span className="text-caption-1-regular text-text-tertiary">
            Edited {formatDate(row.original.updated_at)}
          </span>
        </span>
      );
    },
  },
  {
    id: "active",
    header: "On it now",
    cell: ({ row }) => (
      <Count value={formatNumber(row.original.totals?.active ?? 0)} ran={row.original.live_version_id !== null} />
    ),
  },
  {
    id: "entered",
    header: "Entered",
    cell: ({ row }) => (
      <Count value={formatNumber(row.original.totals?.entered ?? 0)} ran={row.original.live_version_id !== null} />
    ),
  },
  {
    id: "goal",
    header: "Reached goal",
    cell: ({ row }) => (
      <Count
        value={percent(row.original.totals?.goal ?? 0, row.original.totals?.entered ?? 0)}
        ran={(row.original.totals?.entered ?? 0) > 0}
      />
    ),
  },
];

const widths = { series: "w-[40%]", status: "w-[18%]", active: "w-[14%]", entered: "w-[14%]", goal: "w-[14%]" };

function Totals({ list }: { list: Series[] }) {
  const live = list.filter((series) => series.status === "live");
  const sum = (pick: (s: Series) => number) => list.reduce((total, s) => total + pick(s), 0);
  const entered = sum((s) => s.totals?.entered ?? 0);
  const goal = sum((s) => s.totals?.goal ?? 0);

  return (
    <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
      <Stat label="Live series" value={live.length} note={`of ${list.length}`} />
      <Stat label="On a series now" value={formatNumber(sum((s) => s.totals?.active ?? 0))} note="People mid-way" />
      <Stat label="Entered" value={formatNumber(entered)} note="Since each went live" />
      <Stat label="Reached the goal" value={percent(goal, entered)} note={`${formatNumber(goal)} journeys`} />
    </div>
  );
}

/*
 * NewSeries names a new series and picks what it starts from.
 */
function NewSeries({ workspace, onClose }: { workspace: string; onClose: () => void }) {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [template, setTemplate] = useState("blank");
  const [subject, setSubject] = useState<"contact" | "company">("contact");
  const create = useMutation({
    mutationFn: () => {
      const picked = templates.find((t) => t.id === template);
      return createSeries(workspace, {
        name: name.trim() || picked?.name || "New series",
        subject: template === "blank" ? subject : (picked?.subject ?? subject),
        spec: templateSpec(template),
      });
    },
    onSuccess: (series) => void navigate({ to: "/d/series/$series", params: { series: series.id } }),
    onError: (error) => void toast.error(error instanceof Error ? error.message : "The series could not be made."),
  });

  return (
    <Modal isOpen onOpenChange={(open) => (open ? undefined : onClose())} size="md">
      <ModalHeader title="New series" description="A journey people or accounts travel on their own, step by step." />
      <ModalBody className="flex flex-col gap-4">
        <Input label="Name" placeholder="Onboarding" value={name} onChange={setName} autoFocus />
        <div className="flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1">
          {templates.map((t) => (
            <button
              key={t.id}
              type="button"
              aria-pressed={template === t.id}
              onClick={() => setTemplate(t.id)}
              className={cx(
                "flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                template === t.id && "bg-background-primary-default",
              )}
            >
              <KindMark icon={t.subject === "company" ? RiBuilding2Line : RiUser3Line} tint="purple" />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="text-body-regular text-text-primary">{t.name}</span>
                <span className="text-body-2-regular text-text-secondary">{t.description}</span>
              </span>
              <RiArrowRightSLine
                aria-hidden
                className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
              />
            </button>
          ))}
        </div>
        {template === "blank" ? (
          <div className="flex gap-2">
            <Button
              size="small"
              variant={subject === "contact" ? "primary" : "secondary"}
              onClick={() => setSubject("contact")}
            >
              For people
            </Button>
            <Button
              size="small"
              variant={subject === "company" ? "primary" : "secondary"}
              onClick={() => setSubject("company")}
            >
              For accounts
            </Button>
          </div>
        ) : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button size="small" loading={create.isPending} onClick={() => create.mutate()}>
          Create series
        </Button>
      </ModalFooter>
    </Modal>
  );
}

export function SeriesList() {
  const workspace = useWorkspace();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [making, setMaking] = useState(false);
  const list = useQuery(seriesListQuery(workspace));
  const needle = query.trim().toLowerCase();
  const all = list.data ?? [];
  const rows = all.filter((series) => needle === "" || series.name.toLowerCase().includes(needle));

  return (
    <Panel label="Series" ground className="h-full w-full flex-1">
      <PageHeader title="Series">
        <Button size="small" leadingIcon={RiAddLine} onClick={() => setMaking(true)}>
          New series
        </Button>
      </PageHeader>
      <ScrollArea ground>
        <div className="flex h-full flex-col gap-4 px-4 pb-4 animate-page-in">
          <Totals list={all} />
          <div className="min-h-[420px] flex-1">
            <DataTable
              aria-label="Series"
              data={rows}
              columns={columns}
              getRowId={(series) => series.id}
              total={`${formatNumber(rows.length)} series`}
              loading={list.isPending}
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
      {making ? <NewSeries workspace={workspace} onClose={() => setMaking(false)} /> : null}
    </Panel>
  );
}
