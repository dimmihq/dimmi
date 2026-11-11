/*
 * The series page: every series with who it starts for, whether it is
 * running, how many people are on it right now, how many ever entered
 * and how many of those reached its goal. Totals across the live ones
 * sit on top. A new series starts from a template, for contacts or for
 * whole accounts, and opens in its builder.
 */
import { RiAddLine, RiBuilding2Line, RiFlowChart, RiUser3Line } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import { StatCards } from "@/components/application/dashboard/stat-cards";
import { DataTable } from "@/components/application/data-table/data-table";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { useWorkspace } from "@/features/customers/names";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { createSeries, type Series, type SeriesSubject, seriesListQuery } from "@/lib/api/series";
import { formatDate, formatNumber } from "@/lib/i18n/format";

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
    <span className="whitespace-nowrap text-body-2-regular text-text-primary tabular-nums">{value}</span>
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
          <span className="whitespace-nowrap text-caption-1-regular text-text-tertiary">
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

const widths = {
  series: "w-[320px]",
  status: "w-[180px]",
  active: "w-[130px]",
  entered: "w-[130px]",
  goal: "w-[140px]",
};

function Totals({ list }: { list: Series[] }) {
  const live = list.filter((series) => series.status === "live");
  const sum = (pick: (s: Series) => number) => list.reduce((total, s) => total + pick(s), 0);
  const entered = sum((s) => s.totals?.entered ?? 0);
  const goal = sum((s) => s.totals?.goal ?? 0);

  return (
    <StatCards
      variant="compact"
      className="gap-2"
      stats={[
        {
          label: "Live series",
          hint: "Series sending to people right now, out of all your series.",
          value: formatNumber(live.length),
          caption: `of ${list.length}`,
        },
        {
          label: "On a series now",
          hint: "People partway through a series: waiting for a step, or about to get a message.",
          value: formatNumber(sum((s) => s.totals?.active ?? 0)),
          caption: "People mid-way",
        },
        {
          label: "Entered",
          hint: "Journeys started across all series since each went live. Someone who enters twice counts twice. Test journeys are left out.",
          value: formatNumber(entered),
          caption: "Since each went live",
        },
        {
          label: "Reached the goal",
          hint: "Share of journeys that met the series goal before they ended.",
          value: percent(goal, entered),
          caption: `${formatNumber(goal)} journeys`,
        },
      ]}
    />
  );
}

/*
 * NewSeries names a new series and picks what it starts from.
 */
function NewSeries({ workspace, onClose }: { workspace: string; onClose: () => void }) {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [template, setTemplate] = useState("blank");
  const [subject, setSubject] = useState<SeriesSubject>("contact");
  const create = useMutation({
    mutationFn: () => {
      const picked = templates.find((t) => t.id === template);
      return createSeries(workspace, {
        name: name.trim() || picked?.name || "New series",
        subject,
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
        <span className="-mb-2 text-body-2-medium text-text-secondary">Who travels it</span>
        <RadioGroup
          aria-label="Who travels it"
          value={subject}
          onChange={(next) => {
            setSubject(next as SeriesSubject);
            setTemplate("blank");
          }}
          className="grid grid-cols-2 gap-2"
        >
          <RadioCard value="contact" icon={RiUser3Line} title="People" description="Each person travels it." />
          <RadioCard
            value="company"
            icon={RiBuilding2Line}
            title="Accounts"
            description="A whole company travels it once."
          />
        </RadioGroup>
        <span className="-mb-2 text-body-2-medium text-text-secondary">Start from</span>
        <RadioGroup aria-label="Start from" value={template} onChange={setTemplate} className="flex flex-col gap-2">
          {templates
            .filter((t) => t.id === "blank" || t.subject === subject)
            .map((t) => (
              <RadioCard key={t.id} value={t.id} title={t.name} description={t.description} />
            ))}
        </RadioGroup>
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
