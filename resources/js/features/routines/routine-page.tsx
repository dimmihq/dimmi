/*
 * One routine: its runs, newest first, each opening the thread Operator
 * wrote on, and its settings, saved with the bar like any setting. From
 * the header an admin runs it now, for real or as a dry run that reads
 * real data but changes nothing, pauses or resumes it, and deletes it. A
 * routine that paused itself says why, with the way back.
 */
import { RiArrowLeftSLine, RiDeleteBin6Line, RiFlaskLine, RiPauseLine, RiPlayLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { StatCards } from "@/components/application/dashboard/stat-cards";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { usePermission } from "@/features/settings/inboxes";
import { Item, Section } from "@/features/settings/layout";
import { SaveBar } from "@/features/settings/save-bar";
import { useAccount } from "@/features/settings/use-account";
import { Panel, ScrollArea } from "@/features/shell/page";
import {
  deleteRoutine,
  type Routine,
  type RoutineRun,
  routineKeys,
  routineQuery,
  routineRunsQuery,
  runRoutine,
  updateRoutine,
} from "@/lib/api/routines";
import { formatDateTime, formatRelative } from "@/lib/i18n/format";

import { draftFromRoutine, inputOf, problems, type RoutineDraft, sameDraft } from "./draft";
import { lookOf } from "./looks";
import { RoutineFields } from "./routine-fields";
import { OutcomeChip } from "./routine-list";
import { describeRoutine } from "./timing";

type Tabbed = "runs" | "settings";

const pauses: Record<string, string> = {
  consecutive_failures: "It paused after failing three runs in a row.",
  owner_permissions_lost: "It paused because the teammate it runs as left or lost a permission it needs.",
  invalid_schedule: "It paused because its schedule could not be read.",
};

export function RoutinePage({ id }: { id: string }) {
  const workspace = useAccount().workspace.id;
  const { data: routine, isError } = useQuery(routineQuery(workspace, id));
  const [tab, setTab] = useState<Tabbed>("runs");

  if (isError) {
    return (
      <Panel label="Routine" ground className="h-full flex-1">
        <div className="m-auto flex flex-col items-center gap-2 p-8 text-center">
          <p className="text-body-medium text-text-primary">No such routine</p>
          <Link to="/d/routines" className="text-body-2-regular text-text-secondary underline">
            Back to routines
          </Link>
        </div>
      </Panel>
    );
  }
  if (!routine) {
    return (
      <Panel label="Routine" ground className="h-full flex-1">
        {null}
      </Panel>
    );
  }
  return <Loaded key={routine.updated_at} workspace={workspace} routine={routine} tab={tab} onTab={setTab} />;
}

function Loaded({
  workspace,
  routine,
  tab,
  onTab: setTab,
}: {
  workspace: string;
  routine: Routine;
  tab: Tabbed;
  onTab: (tab: Tabbed) => void;
}) {
  const canManage = usePermission(workspace, "ai_agent:manage");
  const [draft, setDraft] = useState<RoutineDraft>(() => draftFromRoutine(routine));
  const [deleting, setDeleting] = useState(false);
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const saved = draftFromRoutine(routine);
  const dirty = !sameDraft(draft, saved);
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: routineKeys.all(workspace) });
  };
  const save = useMutation({
    mutationFn: () => updateRoutine(workspace, routine.id, inputOf(draft)),
    onSuccess: (r) => {
      queryClient.setQueryData(routineKeys.one(workspace, r.id), r);
      refresh();
      toast.success("Saved.");
    },
  });
  const status = useMutation({
    mutationFn: (next: "active" | "paused") => updateRoutine(workspace, routine.id, { status: next }),
    onSuccess: (r) => {
      queryClient.setQueryData(routineKeys.one(workspace, r.id), r);
      refresh();
    },
    onError: (error) => toast.error(error.message),
  });
  const run = useMutation({
    mutationFn: (dryRun: boolean) => runRoutine(workspace, routine.id, dryRun),
    onSuccess: (r) => {
      setTab("runs");
      void queryClient.invalidateQueries({ queryKey: routineKeys.runs(workspace, routine.id) });
      toast.success(r.trigger === "dry_run" ? "Dry run started. Nothing it finds is applied." : "Run started.");
    },
    onError: (error) => toast.error(error.message),
  });

  useBlocker({
    shouldBlockFn: () => dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => dirty,
  });

  const blocking = problems(draft);

  return (
    <Panel label="Routine" ground className="relative h-full flex-1">
      <Header
        routine={routine}
        canManage={canManage}
        running={run.isPending ? run.variables : null}
        onRun={(dryRun) => run.mutate(dryRun)}
        onToggle={() => status.mutate(routine.status === "active" ? "paused" : "active")}
        onDelete={() => setDeleting(true)}
      />
      <div className="shrink-0 px-4">
        <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key) as Tabbed)}>
          <TabList
            aria-label="Routine"
            items={[
              { id: "runs", label: "Runs" },
              { id: "settings", label: "Settings" },
            ]}
          >
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
      </div>
      <ScrollArea ground>
        <div className="mx-auto flex w-full max-w-220 flex-col gap-8 px-4 pt-6 pb-28 sm:px-6">
          {routine.status === "paused" && routine.pause_reason ? (
            <Paused
              routine={routine}
              resuming={status.isPending}
              canManage={canManage}
              onResume={() => status.mutate("active")}
            />
          ) : null}
          {tab === "runs" ? <RoutineStats routine={routine} /> : null}
          {tab === "runs" ? <Runs workspace={workspace} routine={routine} /> : null}
          {tab === "settings" ? (
            <RoutineFields workspace={workspace} draft={draft} disabled={!canManage} onChange={setDraft} />
          ) : null}
        </div>
      </ScrollArea>
      <div className="pointer-events-none absolute inset-x-0 bottom-4 z-10 px-4 sm:px-6">
        <div className="pointer-events-auto mx-auto w-full max-w-[632px]">
          <SaveBar
            dirty={dirty}
            saving={save.isPending}
            canSave={dirty && blocking.length === 0}
            error={save.error ? save.error.message : blocking[0] && dirty ? blocking[0] : null}
            onSave={() => save.mutate()}
            onDiscard={() => setDraft(saved)}
          />
        </div>
      </div>
      <AlertDialog
        isOpen={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${routine.name}?`}
        description="It stops running for good. Its past runs, threads and suggestions stay."
        confirmLabel="Delete routine"
        onConfirm={async () => {
          await deleteRoutine(workspace, routine.id);
          refresh();
          void navigate({ to: "/d/routines" });
        }}
      />
    </Panel>
  );
}

function Paused({
  routine,
  resuming,
  canManage,
  onResume,
}: {
  routine: Routine;
  resuming: boolean;
  canManage: boolean;
  onResume: () => void;
}) {
  return (
    <div className="flex flex-col gap-3 rounded-2xl bg-status-yellow-background p-4 sm:flex-row sm:items-center">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-body-medium text-status-yellow-text">This routine is paused</span>
        <span className="text-body-2-regular text-text-secondary">
          {pauses[routine.pause_reason ?? ""] ?? "It paused on its own."} Resuming starts its schedule again from now.
        </span>
      </div>
      {canManage ? (
        <Button size="small" variant="secondary" loading={resuming} onClick={onResume}>
          Resume
        </Button>
      ) : null}
    </div>
  );
}

/*
 * RoutineStats says what came of the routine: its suggestions waiting,
 * applied and declined, and what it cost this month.
 */
function RoutineStats({ routine }: { routine: Routine }) {
  const c = routine.suggestions;
  const decided = c.applied + c.declined + c.reverted;
  const budget = routine.monthly_budget_micros;
  const usd = (micros: number) => `$${(micros / 1_000_000).toFixed(2)}`;

  return (
    <StatCards
      variant="compact"
      className="gap-2"
      stats={[
        {
          label: "Waiting for review",
          hint: "Suggestions from this routine nobody has applied or declined yet.",
          value: c.pending.toLocaleString(),
        },
        {
          label: "Applied",
          hint: "Suggestions a teammate applied and kept.",
          value: c.applied.toLocaleString(),
          caption: decided > 0 ? `${Math.round((c.applied / decided) * 100)}% of decided` : "None decided yet",
        },
        {
          label: "Declined",
          hint: "Suggestions declined, undone after applying, or gone stale after four weeks.",
          value: (c.declined + c.reverted).toLocaleString(),
        },
        {
          label: "Spend this month",
          hint: "What this routine's runs cost this month, in US dollars.",
          value: usd(routine.month_spend_micros),
          caption: budget === null ? "No budget set" : `of ${usd(budget)} budget`,
        },
      ]}
    />
  );
}

/*
 * runLine says what a run did, in a few words.
 */
function runLine(r: RoutineRun): string {
  switch (r.status) {
    case "queued":
      return "Waiting to start";
    case "running":
      return "Running now";
    case "skipped":
      return `Skipped: ${(r.skip_reason ?? "").replaceAll("_", " ")}`;
    case "failed":
      return `Failed: ${r.error_message ?? r.error_code ?? "unknown error"}`;
  }
  if (r.outcome === "findings") {
    return r.changesets_count === 1 ? "1 suggested change" : `${r.changesets_count} suggested changes`;
  }
  return r.noop_reason ? `Nothing to change: ${r.noop_reason}` : "Nothing to change";
}

const triggers = { scheduled: "On schedule", manual: "Run now", dry_run: "Dry run" } as const;

function Runs({ workspace, routine }: { workspace: string; routine: Routine }) {
  const { data, isPending } = useQuery(routineRunsQuery(workspace, routine.id));
  const next = routine.status === "active" && routine.next_run_at;

  return (
    <Section
      title="Runs"
      description={
        next ? `Next run ${formatRelative(routine.next_run_at ?? "")}. ${describeRoutine(routine)}.` : undefined
      }
      loading={isPending}
    >
      {data?.length === 0 ? (
        <Item
          mark={<KindMark icon={RiPlayLine} tint="neutral" />}
          title="No runs yet"
          detail="Try a dry run: it reads real data and shows what it would suggest, without changing anything."
        />
      ) : null}
      {(data ?? []).map((r) => (
        <RunRow key={r.id} routine={routine.id} run={r} />
      ))}
    </Section>
  );
}

/*
 * runTint is the colour of a run's mark: red when it failed, green when
 * it suggested something.
 */
function runTint(r: RoutineRun): "rose" | "lime" | "neutral" {
  if (r.status === "failed") {
    return "rose";
  }
  return r.outcome === "findings" ? "lime" : "neutral";
}

/*
 * RunRow is one run, opening its thread when it wrote one.
 */
function RunRow({ routine, run: r }: { routine: string; run: RoutineRun }) {
  const live = r.status === "running" || r.status === "queued";
  const cost = `$${(r.cost_micros / 1_000_000).toFixed(3)}`;
  const row = (
    <Item
      mark={<KindMark icon={r.trigger === "dry_run" ? RiFlaskLine : RiPlayLine} tint={runTint(r)} />}
      title={runLine(r)}
      detail={`${triggers[r.trigger]} · ${formatDateTime(r.started_at ?? r.created_at)} · ${cost}`}
    >
      {live ? (
        <Chip variant="caption" color="blue">
          {r.status === "running" ? "Running" : "Queued"}
        </Chip>
      ) : null}
    </Item>
  );
  if (!r.thread_id) {
    return row;
  }
  return (
    <Link
      to="/d/routines/$routine/threads/$thread"
      params={{ routine, thread: r.thread_id }}
      className="-ms-3 block ps-3 transition-colors duration-150 first:rounded-t-2xl last:rounded-b-2xl hover:bg-background-secondary-hover"
    >
      {row}
    </Link>
  );
}

/*
 * Header names the routine and how its last run went, with the ways to
 * run, pause and delete it.
 */
function Header({
  routine,
  canManage,
  running,
  onRun,
  onToggle,
  onDelete,
}: {
  routine: Routine;
  canManage: boolean;
  running: boolean | null | undefined;
  onRun: (dryRun: boolean) => void;
  onToggle: () => void;
  onDelete: () => void;
}) {
  const look = lookOf(routine.template_key);
  return (
    <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <Link
          to="/d/routines"
          aria-label="Back to routines"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
        </Link>
        <KindMark icon={look.icon} tint={look.tint} size="sm" />
        <h1 className="min-w-0 truncate text-body-medium text-text-primary">{routine.name}</h1>
        <OutcomeChip routine={routine} />
        <span className="hidden truncate text-body-2-regular text-text-tertiary md:inline">
          {describeRoutine(routine)}
        </span>
      </div>
      {canManage ? (
        <div className="flex shrink-0 items-center gap-2">
          <Button
            size="small"
            variant="secondary"
            leadingIcon={RiFlaskLine}
            loading={running === true}
            onClick={() => onRun(true)}
          >
            Dry run
          </Button>
          <Button size="small" leadingIcon={RiPlayLine} loading={running === false} onClick={() => onRun(false)}>
            Run now
          </Button>
          <IconButton
            size="small"
            icon={routine.status === "active" ? RiPauseLine : RiPlayLine}
            aria-label={routine.status === "active" ? "Pause routine" : "Resume routine"}
            onClick={onToggle}
          />
          <IconButton size="small" icon={RiDeleteBin6Line} aria-label="Delete routine" onClick={onDelete} />
        </div>
      ) : null}
    </header>
  );
}
