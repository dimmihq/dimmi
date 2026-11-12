/*
 * One series: the canvas of its steps and the inspector of the one
 * picked, under a header with its name, where it stands, a switch
 * between building it and reading its results, and the buttons that
 * set it live, publish changes, pause, resume, archive and try it on
 * yourself. Every change saves a moment after the last keystroke, with
 * the lock version it read, so two teammates never undo each other;
 * nothing reaches customers until it is published. Picking a card
 * pushes its settings out from the right, in a pane that drags to any
 * width; on screens too narrow for both, they open in a drawer.
 */
import {
  type RemixiconComponentType,
  RiArchiveLine,
  RiArrowLeftSLine,
  RiDeleteBinLine,
  RiPauseLine,
  RiPlayLine,
  RiTestTubeLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { Focusable } from "react-aria-components";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { toast } from "@/components/base/toast/toast";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { KindMark } from "@/features/automation/parts";
import { useWorkspace } from "@/features/customers/names";
import { Panel } from "@/features/shell/page";
import { PushPane } from "@/features/shell/push-pane";
import { useWide } from "@/features/shell/use-wide";
import {
  actOnSeries,
  deleteSeries,
  type NodeKind,
  publishSeries,
  type Series,
  type SeriesAction,
  type SeriesSpec,
  saveSeries,
  seriesKey,
  seriesQuery,
  seriesReportQuery,
  trySeries,
} from "@/lib/api/series";

import { addGroups, nodeLook, statusLook } from "./looks";
import { addStep, type Sending, sendingOf } from "./model";
import { SeriesCanvas } from "./series-canvas";
import { SeriesInspector } from "./series-inspector";
import { TryPanel } from "./try-panel";

const modes = [
  ["build", "Build"],
  ["results", "Results"],
] as const;

function Mode({ results, onChange }: { results: boolean; onChange: (results: boolean) => void }) {
  return (
    <SegmentedControl
      aria-label="Show"
      selectedKeys={new Set([results ? "results" : "build"])}
      onSelectionChange={(keys) => onChange([...keys][0] === "results")}
    >
      {modes.map(([id, name]) => (
        <SegmentedControlItem key={id} id={id}>
          {name}
        </SegmentedControlItem>
      ))}
    </SegmentedControl>
  );
}

/*
 * AddStep lists the kinds of step a path can lead to.
 */
function AddStep({
  account,
  onPick,
  onClose,
}: {
  account: boolean;
  onPick: (kind: NodeKind) => void;
  onClose: () => void;
}) {
  return (
    <Modal isOpen onOpenChange={(open) => (open ? undefined : onClose())} size="md">
      <ModalHeader title="Add a step" />
      <ModalBody className="flex flex-col gap-4">
        {addGroups.map(([group, kinds]) => (
          <div key={group} className="flex flex-col gap-1">
            <span className="text-caption-1-semibold text-text-tertiary">{group}</span>
            <div className="grid grid-cols-2 gap-1">
              {kinds
                .filter((kind) => !(kind === "topic" && account))
                .map((kind) => (
                  <button
                    key={kind}
                    type="button"
                    onClick={() => onPick(kind)}
                    className="flex cursor-pointer items-center gap-2 rounded-xl bg-background-secondary-default px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                  >
                    <KindMark icon={nodeLook[kind].icon} tint={nodeLook[kind].tint} size="sm" />
                    <span className="text-body-2-medium text-text-primary">{nodeLook[kind].name}</span>
                  </button>
                ))}
            </div>
          </div>
        ))}
      </ModalBody>
    </Modal>
  );
}

/*
 * Publish asks for a note and sets the working copy live.
 */
function Publish({
  series,
  saving,
  onPublish,
  onClose,
}: {
  series: Series;
  saving: boolean;
  onPublish: (note: string) => void;
  onClose: () => void;
}) {
  const [note, setNote] = useState("");
  const first = series.live_version_id === null;

  return (
    <Modal isOpen onOpenChange={(open) => (open ? undefined : onClose())} size="md">
      <ModalHeader
        title={first ? "Set this series live" : "Publish changes"}
        description={
          first
            ? "People who match start entering at once."
            : "People already on the series follow the new version from the step they stand on. Anyone on a removed step moves to the next step that remains."
        }
      />
      <ModalBody>
        <Input label="Note" placeholder="What changed" value={note} onChange={setNote} />
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button size="small" loading={saving} onClick={() => onPublish(note)}>
          {first ? "Set live" : "Publish"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * useAutosave keeps the working copy saved a moment after each change,
 * one save at a time, over the lock version the last save returned.
 */
function useAutosave(workspace: string, series: Series) {
  const client = useQueryClient();
  const [spec, setSpec] = useState<SeriesSpec>(series.spec);
  const [sending, setSending] = useState<Sending>(sendingOf(series));
  const [dirty, setDirty] = useState(false);
  const lock = useRef(series.lock_version);
  const pending = useRef<{ spec: SeriesSpec; sending: Sending } | null>(null);
  const save = useMutation({
    mutationFn: (next: { spec: SeriesSpec; sending: Sending }) =>
      saveSeries(workspace, series.id, {
        lock_version: lock.current,
        name: series.name,
        spec: next.spec,
        ...next.sending,
      }),
    onSuccess: (saved) => {
      lock.current = saved.lock_version;
      client.setQueryData(seriesQuery(workspace, series.id).queryKey, saved);
    },
    onError: (error) => void toast.error(error instanceof Error ? error.message : "The series was not saved."),
  });
  const { mutate } = save;

  useEffect(() => {
    if (!dirty) {
      return;
    }
    pending.current = { spec, sending };
    const timer = setTimeout(() => {
      setDirty(false);
      pending.current = null;
      mutate({ spec, sending });
    }, 700);

    return () => clearTimeout(timer);
  }, [dirty, spec, sending, mutate]);

  useEffect(
    () => () => {
      if (pending.current) {
        mutate(pending.current);
      }
    },
    [mutate],
  );

  return {
    spec,
    sending,
    saving: save.isPending || dirty,
    setSpec: (next: SeriesSpec) => {
      setSpec(next);
      setDirty(true);
    },
    setSending: (next: Sending) => {
      setSending(next);
      setDirty(true);
    },
  };
}

function saveLine(saving: boolean, problems: number): string {
  if (saving) {
    return "Saving…";
  }

  return problems > 0 ? `${problems} to fix before going live` : "Saved";
}

/*
 * Tip is an icon button that names itself in a tooltip.
 */
function Tip({
  label,
  icon,
  busy = false,
  onPress,
}: {
  label: string;
  icon: RemixiconComponentType;
  busy?: boolean;
  onPress: () => void;
}) {
  return (
    <TooltipTrigger delay={200}>
      <Focusable>
        <IconButton size="small" icon={icon} aria-label={label} disabled={busy} onClick={onPress} />
      </Focusable>
      <Tooltip size="md">{label}</Tooltip>
    </TooltipTrigger>
  );
}

type Asked = "try" | "pause" | "archive" | "delete";

/*
 * asks words each question the header puts before acting: what trying
 * the series on yourself does, and what pausing or archiving does to
 * the people on it.
 */
const asks = {
  try: {
    title: "Try this series on yourself?",
    description:
      "You enter the published version as a test journey, under the contact with your email. Its messages really " +
      "go out, to you. Rules, quiet hours and the daily cap do not hold you back, you can skip each wait, and the " +
      "report never counts you.",
    confirmLabel: "Start test",
    variant: "default",
  },
  pause: {
    title: "Pause this series?",
    description:
      "No message goes out and nobody moves on until you resume. Everyone keeps their place, and waits that end " +
      "meanwhile carry on once it resumes.",
    confirmLabel: "Pause",
    variant: "default",
  },
  archive: {
    title: "Archive this series?",
    description:
      "Every journey on it ends now and messages not sent yet are held back. It cannot go live again. Its report " +
      "stays.",
    confirmLabel: "Archive",
    variant: "danger",
  },
  delete: {
    title: "Delete this draft?",
    description: "The series never went live, so nobody is on it. It goes away for good.",
    confirmLabel: "Delete",
    variant: "danger",
  },
} as const;

/*
 * Actions are the header's buttons, as the series' status allows them.
 * Trying, pausing, archiving and deleting a draft ask first; resuming does not, since it
 * only lets the series carry on. The question keeps its words while it
 * closes, so a dialog fading out never turns into a different one.
 */
function Actions({
  series,
  saving,
  trying,
  onTry,
  onAct,
  onDelete,
  onPublish,
}: {
  series: Series;
  saving: boolean;
  trying: boolean;
  onTry: () => void;
  onAct: (action: SeriesAction) => void;
  onDelete: () => void;
  onPublish: () => void;
}) {
  const [asked, setAsked] = useState<Asked | null>(null);
  const [worded, setWorded] = useState<Asked>("pause");
  const ask = (next: Asked) => {
    setWorded(next);
    setAsked(next);
  };
  const ran = series.live_version_id !== null;
  const live = series.status === "live";
  const editable = series.status !== "archived";
  const confirm = () => {
    if (asked === "try") {
      onTry();
    } else if (asked === "delete") {
      onDelete();
    } else if (asked) {
      onAct(asked);
    }
  };

  return (
    <>
      <AlertDialog
        isOpen={asked !== null}
        onOpenChange={(open) => (open ? undefined : setAsked(null))}
        isDismissable
        onConfirm={confirm}
        {...asks[worded]}
      />
      {live ? <Tip label="Try it on me" icon={RiTestTubeLine} busy={trying} onPress={() => ask("try")} /> : null}
      {live ? <Tip label="Pause" icon={RiPauseLine} onPress={() => ask("pause")} /> : null}
      {series.status === "paused" ? <Tip label="Resume" icon={RiPlayLine} onPress={() => onAct("resume")} /> : null}
      {ran && editable ? <Tip label="Archive" icon={RiArchiveLine} onPress={() => ask("archive")} /> : null}
      {ran ? null : <Tip label="Delete draft" icon={RiDeleteBinLine} onPress={() => ask("delete")} />}
      {editable ? (
        <Button
          size="small"
          disabled={saving || series.problems.length > 0 || (ran && !series.has_unpublished_changes)}
          onClick={onPublish}
        >
          {ran ? "Publish changes" : "Set live"}
        </Button>
      ) : null}
    </>
  );
}

function Builder({ workspace, series, step }: { workspace: string; series: Series; step?: string }) {
  const client = useQueryClient();
  const draft = useAutosave(workspace, series);
  const ran = series.live_version_id !== null;
  const editable = series.status !== "archived";
  const report = useQuery({ ...seriesReportQuery(workspace, series.id, series.status === "live"), enabled: ran }).data;
  const [selected, setSelected] = useState(step ?? "entry");
  const [results, setResults] = useState(ran);
  const wide = useWide(1280);
  const [inspecting, setInspecting] = useState(step !== undefined || wide);
  const [adding, setAdding] = useState<{ from: string; path: string } | null>(null);
  const [publishing, setPublishing] = useState(false);
  const [journey, setJourney] = useState<string | null>(null);
  const node = draft.spec.nodes.find((n) => n.id === selected) ?? draft.spec.nodes[0];
  const look = statusLook[series.status];
  const account = series.subject === "company";
  const refresh = () => void client.invalidateQueries({ queryKey: seriesKey(workspace) });
  const fail = (error: unknown) => void toast.error(error instanceof Error ? error.message : "That did not work.");

  const publish = useMutation({
    mutationFn: (note: string) => publishSeries(workspace, series.id, note),
    onSuccess: () => {
      setPublishing(false);
      refresh();
      toast.success(ran ? "Changes published." : "The series is live.");
    },
    onError: fail,
  });
  const act = useMutation({
    mutationFn: (action: SeriesAction) => actOnSeries(workspace, series.id, action),
    onSuccess: refresh,
    onError: fail,
  });
  const navigate = useNavigate();
  const remove = useMutation({
    mutationFn: () => deleteSeries(workspace, series.id),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: seriesKey(workspace) });
      toast.success("Draft deleted.");
      await navigate({ to: "/d/series" });
    },
    onError: fail,
  });
  const tryIt = useMutation({
    mutationFn: () => trySeries(workspace, series.id),
    onSuccess: (j) => setJourney(j.id),
    onError: fail,
  });

  const select = (next: string) => {
    setSelected(next);
    setInspecting(true);
  };
  const inspector = {
    workspace,
    series: series.id,
    spec: draft.spec,
    sending: draft.sending,
    account,
    report,
    results: results && ran,
    editable,
    problems: series.problems,
    onSpec: draft.setSpec,
    onSending: draft.setSending,
    onSelect: setSelected,
  };

  const canvas = (
    <Panel label="Series" ground className="h-full w-full">
      <header className="flex min-h-12 shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 ps-2.5 pe-2.5 pt-[7px]">
        <div className="flex min-w-40 flex-1 items-center gap-2">
          <Link
            to="/d/series"
            aria-label="Back to series"
            className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
          </Link>
          <h1 className="min-w-0 truncate text-body-medium text-text-primary">{series.name}</h1>
          <Chip variant="caption" color={look.tint}>
            {look.name}
          </Chip>
          <span className="hidden truncate text-body-2-regular text-text-tertiary sm:inline">
            {saveLine(draft.saving, series.problems.length)}
          </span>
        </div>
        <div className="ms-auto flex shrink-0 items-center gap-2">
          {ran ? <Mode results={results} onChange={setResults} /> : null}
          <Actions
            series={series}
            saving={draft.saving}
            trying={tryIt.isPending}
            onTry={() => tryIt.mutate()}
            onAct={(action) => act.mutate(action)}
            onDelete={() => remove.mutate()}
            onPublish={() => setPublishing(true)}
          />
        </div>
      </header>
      <SeriesCanvas
        moves={{
          selected,
          results: results && ran,
          editable,
          spec: draft.spec,
          report,
          problems: series.problems,
          onSelect: select,
          onAdd: (from, path) => setAdding({ from, path }),
        }}
      />
    </Panel>
  );

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1">
      <PushPane
        name="dimmi:series"
        label="Step settings"
        open={inspecting && node !== undefined}
        onOpenChange={setInspecting}
        main={canvas}
        side={
          node ? (
            <SeriesInspector
              {...inspector}
              node={node}
              onClose={() => setInspecting(false)}
              className="h-full w-full"
            />
          ) : null
        }
      />
      {adding ? (
        <AddStep
          account={account}
          onClose={() => setAdding(null)}
          onPick={(kind) => {
            const [spec, id] = addStep(draft.spec, adding.from, adding.path, kind);
            draft.setSpec(spec);
            setAdding(null);
            select(id);
          }}
        />
      ) : null}
      {publishing ? (
        <Publish
          series={series}
          saving={publish.isPending}
          onPublish={(note) => publish.mutate(note)}
          onClose={() => setPublishing(false)}
        />
      ) : null}
      {journey ? (
        <TryPanel
          workspace={workspace}
          series={series.id}
          journey={journey}
          spec={draft.spec}
          onClose={() => setJourney(null)}
        />
      ) : null}
    </div>
  );
}

export function SeriesBuilder({ id, step }: { id: string; step?: string }) {
  const workspace = useWorkspace();
  const series = useQuery(seriesQuery(workspace, id));

  if (!series.data) {
    return (
      <Panel label="Series" ground className="h-full flex-1">
        <div className="flex h-full items-center justify-center text-body-2-regular text-text-tertiary">
          {series.isError ? "This series does not exist or you cannot see it." : "Loading…"}
        </div>
      </Panel>
    );
  }

  return <Builder key={series.data.id} workspace={workspace} series={series.data} step={step} />;
}
