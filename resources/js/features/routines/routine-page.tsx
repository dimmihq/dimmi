/*
 * One routine: how it is built, saved with the bar like any setting, and
 * its runs, each opening the thread Operator wrote on. From
 * the header an admin runs it now, for real or as a dry run that reads
 * real data but changes nothing, pauses or resumes it, and deletes it. A
 * routine that paused itself says why, with the way back.
 */
import { RiArrowLeftSLine, RiDeleteBin6Line, RiFlaskLine, RiPauseLine, RiPlayLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { usePermission } from "@/features/settings/inboxes";
import { SaveBar } from "@/features/settings/save-bar";
import { useAccount } from "@/features/settings/use-account";
import { Panel, ScrollArea } from "@/features/shell/page";
import { PushPane } from "@/features/shell/push-pane";
import { useWide } from "@/features/shell/use-wide";
import { deleteRoutine, type Routine, routineKeys, routineQuery, runRoutine, updateRoutine } from "@/lib/api/routines";

import { draftFromRoutine, inputOf, problems, type RoutineDraft, sameDraft } from "./draft";
import { lookOf } from "./looks";
import { RoutineFields } from "./routine-fields";
import { OutcomeChip } from "./routine-list";
import { Runs } from "./run-list";
import { RunPanel } from "./run-panel";
import { describeRoutine } from "./timing";
import { useRoutineWatch } from "./use-run-live";

type Tabbed = "build" | "runs";

const pauses: Record<string, string> = {
  consecutive_failures: "It paused after failing three runs in a row.",
  owner_permissions_lost: "It paused because the teammate it runs as left or lost a permission it needs.",
  invalid_schedule: "It paused because its schedule could not be read.",
};

export function RoutinePage({ id }: { id: string }) {
  const workspace = useAccount().workspace.id;
  const { data: routine, isError } = useQuery(routineQuery(workspace, id));
  const [tab, setTab] = useState<Tabbed>("build");

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
  const [generation, setGeneration] = useState(0);
  /* The run in the side panel stays the last one picked while the panel slides shut. */
  const [watching, setWatching] = useState<string | null>(null);
  const [panel, setPanel] = useState(false);
  const watch = (run: string) => {
    setWatching(run);
    setPanel(true);
  };
  const wide = useWide(1024);
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
      if (wide) {
        watch(r.id);
      } else {
        void navigate({ to: "/d/routines/$routine/runs/$run", params: { routine: routine.id, run: r.id } });
      }
    },
    onError: (error) => toast.error(error.message),
  });

  useBlocker({
    shouldBlockFn: () => dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => dirty,
  });

  const blocking = problems(draft);
  useRoutineWatch(workspace, routine.id);

  const main = (
    <Panel label="Routine" ground className="relative h-full min-w-0 flex-1">
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
              { id: "build", label: "Build" },
              { id: "runs", label: "Runs" },
            ]}
          >
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
      </div>
      <ScrollArea ground>
        <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-4 pt-6 pb-28 sm:px-6">
          {routine.status === "paused" && routine.pause_reason ? (
            <Paused
              routine={routine}
              resuming={status.isPending}
              canManage={canManage}
              onResume={() => status.mutate("active")}
            />
          ) : null}
          {tab === "runs" ? <Runs workspace={workspace} routine={routine} onOpen={wide ? watch : undefined} /> : null}
          {tab === "build" ? (
            <RoutineFields
              key={generation}
              workspace={workspace}
              draft={draft}
              disabled={!canManage}
              onChange={setDraft}
            />
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
            onDiscard={() => {
              setDraft(saved);
              setGeneration((g) => g + 1);
            }}
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

  return (
    <PushPane
      name="dimmi:routine"
      label="Run"
      open={panel && watching !== null}
      onOpenChange={setPanel}
      width={420}
      main={main}
      side={
        watching ? (
          <RunPanel
            workspace={workspace}
            routine={routine}
            run={watching}
            onPick={watch}
            onClose={() => setPanel(false)}
          />
        ) : null
      }
    />
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
    <header className="@container flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
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
        <span className="hidden shrink-0 @2xl:flex">
          <OutcomeChip routine={routine} />
        </span>
        <span className="hidden truncate text-body-2-regular text-text-tertiary @4xl:inline">
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
