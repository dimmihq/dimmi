/*
 * One series, in the studio layout Agent Studio and workflows share.
 * The header holds its name, where it stands and the buttons that set
 * it live or publish changes; pausing, resuming, archiving and deleting
 * a draft sit in its menu. Build holds the canvas of its steps; picking
 * a card pushes its settings out beside it. Analytics reads the same
 * canvas with each step's results; History lists published versions.
 * Beside them sit Preview (the steps in words and what needs fixing),
 * Simulations and Operator. Every change saves a moment
 * after the last keystroke, with the lock version it read, so two
 * teammates never undo each other; nothing reaches customers until it
 * is published.
 */
import {
  RiArchiveLine,
  RiArrowLeftSLine,
  RiDeleteBinLine,
  RiMore2Line,
  RiPauseCircleLine,
  RiPlayCircleLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import {
  Dropdown,
  DropdownDivider,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { StudioArrive } from "@/features/agents/agent-arrive";
import { KindMark } from "@/features/automation/parts";
import { seriesSlots } from "@/features/automation/studio/slots";
import { type SideTabId, Studio, StudioSide, type StudioTabId } from "@/features/automation/studio/studio";
import { SuggestionBar } from "@/features/automation/studio/suggestion-bar";
import { useWorkspace } from "@/features/customers/names";
import { Panel, ScrollArea } from "@/features/shell/page";
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
} from "@/lib/api/series";

import { addGroups, nodeLook, statusLook } from "./looks";
import { addStep, type Sending, sendingOf } from "./model";
import { type Moves, SeriesCanvas } from "./series-canvas";
import { SeriesInspector } from "./series-inspector";
import { SeriesPreview } from "./series-preview";
import { useSeriesRequest } from "./studio/series-review";

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
    /*
     * adopt takes a copy the server wrote, such as the one saved when the
     * teammate accepts Operator's change, as the working copy, dropping a
     * save still waiting so it never overwrites it.
     */
    adopt: (next: Series) => {
      pending.current = null;
      setDirty(false);
      lock.current = next.lock_version;
      setSpec(next.spec);
      setSending(sendingOf(next));
      client.setQueryData(seriesQuery(workspace, series.id).queryKey, next);
    },
  };
}

/*
 * ReviewedCanvas is the series canvas, or, while Operator's suggestion is
 * under review, the suggested graph with its added and changed steps
 * marked and nothing to edit until the teammate decides.
 */
function ReviewedCanvas({ moves, suggestion }: { moves: Moves; suggestion: ReturnType<typeof useSeriesRequest> }) {
  const review = suggestion.review;
  if (!review) {
    return <SeriesCanvas moves={moves} />;
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <SuggestionBar
        added={review.added.size}
        changed={review.changed.size + (review.head ? 1 : 0)}
        removed={review.removed.size}
        deciding={suggestion.deciding}
        onAccept={suggestion.accept}
        onDecline={suggestion.decline}
      />
      <SeriesCanvas
        moves={{
          ...moves,
          editable: false,
          spec: review.proposed,
          marks: { added: review.added, changed: review.changed },
        }}
      />
    </div>
  );
}

function saveLine(saving: boolean, problems: number): string {
  if (saving) {
    return "Saving…";
  }

  return problems > 0 ? `${problems} to fix before going live` : "Saved";
}

type Asked = "pause" | "archive" | "delete";

/*
 * asks words each question the menu puts before acting: what pausing,
 * archiving or deleting does to the people on the series.
 */
const asks = {
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
 * SeriesMenu holds what the series' status allows beyond publishing.
 * Pausing, archiving and deleting a draft ask first; resuming does not,
 * since it only lets the series carry on. The question keeps its words
 * while it closes, so a dialog fading out never turns into a different
 * one.
 */
function SeriesMenu({
  series,
  onAct,
  onDelete,
}: {
  series: Series;
  onAct: (action: SeriesAction) => void;
  onDelete: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [asked, setAsked] = useState<Asked | null>(null);
  const [worded, setWorded] = useState<Asked>("pause");
  const ask = (next: Asked) => {
    setOpen(false);
    setWorded(next);
    setAsked(next);
  };
  const ran = series.live_version_id !== null;
  const live = series.status === "live";
  const editable = series.status !== "archived";
  const paused = series.status === "paused";
  if (!(live || paused || (ran && editable) || !ran)) {
    return null;
  }
  const confirm = () => {
    if (asked === "delete") {
      onDelete();
    } else if (asked) {
      onAct(asked);
    }
  };

  return (
    <>
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label="Series actions"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover"
        >
          <RiMore2Line aria-hidden className="size-4" />
        </DropdownTrigger>
        <DropdownPopover aria-label="Series actions" placement="bottom end" className="w-52">
          {live ? (
            <DropdownItem className="gap-2" onSelect={() => ask("pause")}>
              <RiPauseCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
              Pause
            </DropdownItem>
          ) : null}
          {paused ? (
            <DropdownItem
              className="gap-2"
              onSelect={() => {
                setOpen(false);
                onAct("resume");
              }}
            >
              <RiPlayCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
              Resume
            </DropdownItem>
          ) : null}
          {(live || paused) && ran && editable ? <DropdownDivider /> : null}
          {ran && editable ? (
            <DropdownItem className="gap-2 text-text-error-primary" onSelect={() => ask("archive")}>
              <RiArchiveLine aria-hidden className="size-4 shrink-0" />
              Archive
            </DropdownItem>
          ) : null}
          {ran ? null : (
            <DropdownItem className="gap-2 text-text-error-primary" onSelect={() => ask("delete")}>
              <RiDeleteBinLine aria-hidden className="size-4 shrink-0" />
              Delete draft
            </DropdownItem>
          )}
        </DropdownPopover>
      </Dropdown>
      <AlertDialog
        isOpen={asked !== null}
        onOpenChange={(next) => (next ? undefined : setAsked(null))}
        isDismissable
        onConfirm={confirm}
        {...asks[worded]}
      />
    </>
  );
}

function Builder({ workspace, series, step }: { workspace: string; series: Series; step?: string }) {
  const client = useQueryClient();
  const draft = useAutosave(workspace, series);
  const suggestion = useSeriesRequest(workspace, series.id, draft.spec, draft.adopt);
  const ran = series.live_version_id !== null;
  const editable = series.status !== "archived";
  const report = useQuery({ ...seriesReportQuery(workspace, series.id, series.status === "live"), enabled: ran }).data;
  const [selected, setSelected] = useState(step ?? "entry");
  const [tab, setTab] = useState<StudioTabId>(ran ? "analytics" : "build");
  const [sideTab, setSideTab] = useState<SideTabId>("preview");
  const wide = useWide(1280);
  const [inspecting, setInspecting] = useState(step !== undefined || wide);
  const [adding, setAdding] = useState<{ from: string; path: string } | null>(null);
  const [publishing, setPublishing] = useState(false);
  const node = draft.spec.nodes.find((n) => n.id === selected) ?? draft.spec.nodes[0];
  const look = statusLook[series.status];
  const account = series.subject === "company";
  const results = tab === "analytics" && ran;
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

  const select = (next: string) => {
    setSelected(next);
    if (tab === "history") {
      setTab(ran ? "analytics" : "build");
    }
    setInspecting(true);
  };
  const inspector = {
    workspace,
    series: series.id,
    spec: draft.spec,
    sending: draft.sending,
    account,
    report,
    results,
    editable,
    problems: series.problems,
    onSpec: draft.setSpec,
    onSending: draft.setSending,
    onSelect: setSelected,
  };
  const current = { ...series, spec: draft.spec };
  const slot = { workspace, series: current, onAdopt: draft.adopt, step: node?.id };
  const History = seriesSlots.history;
  const Simulations = seriesSlots.simulations;
  const Operator = seriesSlots.operator;

  const start = (
    <>
      <Link
        to="/d/series"
        aria-label="Back to series"
        className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
      </Link>
      <h1 className="min-w-0 truncate px-1.5 text-body-medium text-text-primary">{series.name}</h1>
      <span className="flex shrink-0 items-center gap-1.5">
        <Chip variant="caption" color={look.tint}>
          {look.name}
        </Chip>
      </span>
      <span role="status" className="hidden truncate text-body-2-regular text-text-tertiary @lg:inline">
        {saveLine(draft.saving, series.problems.length)}
      </span>
    </>
  );
  const end = (
    <>
      {editable ? (
        <Button
          size="small"
          disabled={draft.saving || series.problems.length > 0 || (ran && !series.has_unpublished_changes)}
          onClick={() => setPublishing(true)}
        >
          {ran ? "Publish changes" : "Set live"}
        </Button>
      ) : null}
      <SeriesMenu series={series} onAct={(action) => act.mutate(action)} onDelete={() => remove.mutate()} />
    </>
  );
  const canvas = (
    <ReviewedCanvas
      suggestion={suggestion}
      moves={{
        selected,
        results,
        editable,
        spec: draft.spec,
        report,
        problems: series.problems,
        onSelect: select,
        onAdd: (from, path) => setAdding({ from, path }),
      }}
    />
  );

  return (
    <>
      <Studio
        name="dimmi:series"
        label="Series"
        start={start}
        end={end}
        tab={tab}
        onTab={setTab}
        body={
          tab === "history" ? (
            <ScrollArea ground className="px-4 pb-6">
              <div className="mx-auto flex w-full max-w-220 min-w-0 flex-col gap-6 pt-4">
                <History {...slot} />
              </div>
            </ScrollArea>
          ) : (
            canvas
          )
        }
        inspector={{
          label: "Step settings",
          open: inspecting && node !== undefined && tab !== "history",
          onOpenChange: setInspecting,
          node: node ? (
            <SeriesInspector
              {...inspector}
              node={node}
              onClose={() => setInspecting(false)}
              className="h-full w-full"
            />
          ) : null,
        }}
        side={(onClose) => (
          <StudioSide
            tab={sideTab}
            onTab={setSideTab}
            preview={
              <SeriesPreview
                series={series}
                spec={draft.spec}
                onShow={(problem) => select(problem.node_id ?? "entry")}
              />
            }
            simulations={<Simulations {...slot} />}
            onClose={onClose}
            className="h-full w-full"
          />
        )}
        operator={(onClose) => <Operator {...slot} onClose={onClose} className="h-full w-full" />}
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
    </>
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

  return (
    <StudioArrive key={series.data.id}>
      <Builder workspace={workspace} series={series.data} step={step} />
    </StudioArrive>
  );
}
