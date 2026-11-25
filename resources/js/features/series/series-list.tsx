/*
 * The series page, laid out like the agents page: every series with
 * who it starts for, whether it is running, how many people are on it
 * right now, how many ever entered and how many of those reached its
 * goal, published ones first. Totals across them sit on top. A new series starts from a template, for contacts or for
 * whole accounts, and opens in its builder.
 */
import { RiAddLine, RiBuilding2Line, RiFlowChart, RiUser3Line } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { StatCards } from "@/components/application/dashboard/stat-cards";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { KindMark, ListPage, OpenRow } from "@/features/automation/parts";
import { useWorkspace } from "@/features/customers/names";
import { Item, Section } from "@/features/settings/layout";
import { createSeries, type Series, type SeriesSubject, seriesListQuery } from "@/lib/api/series";
import { formatNumber } from "@/lib/i18n/format";

import { percent, statusLook } from "./looks";
import { summary, templateSpec, templates } from "./model";

/*
 * detailOf says who a series starts for until it has run, then how it
 * is doing: who is on it now, how many entered, and how many of those
 * reached its goal.
 */
function detailOf(series: Series): string {
  const entry = series.spec.nodes.find((n) => n.kind === "entry");
  const who = entry ? summary(entry, series.spec) : "";
  if (series.live_version_id === null) {
    return who || "Not live yet";
  }
  const totals = series.totals;
  const entered = totals?.entered ?? 0;
  const parts = [`${formatNumber(totals?.active ?? 0)} on it now`, `${formatNumber(entered)} entered`];
  if (entered > 0) {
    parts.push(`${percent(totals?.goal ?? 0, entered)} reached the goal`);
  }

  return parts.join(" · ");
}

function SeriesRow({ series }: { series: Series }) {
  const look = statusLook[series.status];

  return (
    <OpenRow
      target={{ to: "/d/series/$series", params: { series: series.id } }}
      mark={<KindMark icon={series.subject === "company" ? RiBuilding2Line : RiFlowChart} tint="purple" />}
      title={series.name}
      detail={detailOf(series)}
    >
      {series.status !== "draft" && series.status !== "archived" && series.has_unpublished_changes ? (
        <Chip variant="caption" color="yellow" className="py-0.5">
          Unpublished changes
        </Chip>
      ) : null}
      <Chip variant="caption" color={look.tint}>
        {look.name}
      </Chip>
    </OpenRow>
  );
}

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
  const [making, setMaking] = useState(false);
  const list = useQuery(seriesListQuery(workspace));
  const all = list.data ?? [];
  const published = all.filter((series) => series.status === "live" || series.status === "paused");
  const drafts = all.filter((series) => series.status === "draft");
  const archived = all.filter((series) => series.status === "archived");

  return (
    <ListPage
      title="Series"
      description="Messages that reach people on their own, step by step, over days or weeks."
      action={
        <Button size="small" leadingIcon={RiAddLine} onClick={() => setMaking(true)}>
          New series
        </Button>
      }
    >
      {all.length > 0 ? <Totals list={all} /> : null}
      <Section
        title="Published"
        description="Live series send as people match. Paused ones hold everyone in place."
        loading={list.isPending}
        rows={2}
      >
        {published.length === 0 ? (
          <Item
            mark={<KindMark icon={RiFlowChart} tint="neutral" />}
            title={all.length === 0 ? "No series yet" : "None published yet"}
            detail="Make one to welcome new signups."
          />
        ) : (
          published.map((series) => <SeriesRow key={series.id} series={series} />)
        )}
      </Section>
      {drafts.length > 0 ? (
        <Section title="Drafts" description="Nobody gets these until you set them live.">
          {drafts.map((series) => (
            <SeriesRow key={series.id} series={series} />
          ))}
        </Section>
      ) : null}
      {archived.length > 0 ? (
        <Section title="Archived" description="Finished for good. Their reports stay.">
          {archived.map((series) => (
            <SeriesRow key={series.id} series={series} />
          ))}
        </Section>
      ) : null}
      {making ? <NewSeries workspace={workspace} onClose={() => setMaking(false)} /> : null}
    </ListPage>
  );
}
