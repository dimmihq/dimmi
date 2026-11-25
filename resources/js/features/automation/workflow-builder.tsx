/*
 * The workflow studio, laid out like Agent Studio. Build holds the
 * canvas of steps; picking a card, or the settings button, pushes the
 * step settings out beside it. Analytics shows how the workflow does
 * and its latest runs, each opening to what its steps did; History its
 * published versions. Beside them sit Preview (the draft in words, and
 * what still needs fixing), Simulations, and Operator. Every change is
 * saved to the working copy as it is made; publishing makes that copy
 * the version that runs. Steps with problems carry a mark, and the
 * problems count jumps to the first of them. Teammates who may only
 * read workflows see the same studio without anything to change.
 */
import {
  RiArchiveLine,
  RiArrowLeftSLine,
  RiErrorWarningLine,
  RiMore2Line,
  RiPauseCircleLine,
  RiPlayCircleLine,
  RiSettings3Line,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useMemo, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import {
  Dropdown,
  DropdownDivider,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { ScrollArea } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";
import { agentsQuery } from "@/lib/api/agents";
import { agentQuery } from "@/lib/api/ai";
import { actionConnectionsQuery } from "@/lib/api/ai-actions";
import { articlesQuery, helpCentersQuery } from "@/lib/api/help-centers";
import { schedulesQuery } from "@/lib/api/schedules";
import { tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";
import { type Issue, type Workflow, workflowsQuery } from "@/lib/api/workflows";
import { membersQuery } from "@/lib/api/workspace";

import { Canvas, type CanvasMoves } from "./canvas";
import type { Choices } from "./choices";
import { freshStep, withEdited, withoutStep, withStep } from "./edit-steps";
import { type Head, Inspector } from "./inspector";
import { StatusChip } from "./parts";
import { withWinner } from "./split";
import type { Lists } from "./step-fields";
import { type Names, sentence } from "./step-settings";
import { workflowSlots } from "./studio/slots";
import { StepCounts } from "./studio/step-counts";
import { type SideTabId, Studio, StudioSide, type StudioTabId } from "./studio/studio";
import { SuggestionBar } from "./studio/suggestion-bar";
import { useWorkflowStepCounts } from "./studio/use-workflow-step-counts";
import { WorkflowPreview } from "./studio/workflow-preview";
import { useWorkflowRequest } from "./studio/workflow-review";
import { ticketNames, useTicketLists } from "./ticket-lists";
import { allowsStep, type TriggerKind } from "./triggers";
import { type SaveState, useWorkflowDraft } from "./use-workflow-draft";
import { RunList } from "./workflow-runs";
import { findStep, readyKinds, type Step, type StepKind, triggerStep } from "./workflows";

/*
 * useLists reads the workspace's teams, active teammates and tags, as the
 * pickers offer them and as the cards name them, and whether its agent
 * is on; and the live reusable workflows other than this one, the
 * connections to your systems, the office hours, the published help
 * articles a step may answer with, and the ticket setup, trackers and
 * SLA policies the ticket steps pick from.
 */
function useLists(workspace: string, current: string): { lists: Lists; names: Names } {
  const teams = useQuery(teamsQuery(workspace)).data;
  const members = useQuery(membersQuery(workspace)).data;
  const tags = useQuery(tagsQuery(workspace)).data;
  const agent = useQuery({ ...agentQuery(workspace), retry: false }).data;
  const agents = useQuery({ ...agentsQuery(workspace), retry: false }).data;
  const workflows = useQuery(workflowsQuery(workspace)).data;
  const connections = useQuery({ ...actionConnectionsQuery(workspace), retry: false }).data;
  const schedules = useQuery({ ...schedulesQuery(workspace), retry: false }).data;
  const tickets = useTicketLists(workspace);
  const articles = useArticleChoices(workspace);

  return useMemo(() => {
    const lists: Lists = {
      teams: (teams ?? []).map((team) => [team.id, team.name] as const),
      members: (members ?? [])
        .filter((member) => member.status === "active")
        .map((member) => [member.id, member.user.name] as const),
      tags: (tags ?? []).map((tag) => [tag.id, tag.name] as const),
      workflows: (workflows ?? [])
        .filter((flow) => flow.trigger === "reusable" && flow.status === "live" && flow.id !== current)
        .map((flow) => [flow.id, flow.name] as const),
      connections: (connections ?? [])
        .filter((connection) => connection.kind === "http")
        .map((connection) => [connection.id, connection.name] as const),
      schedules: (schedules ?? []).map((schedule) => [schedule.id, schedule.name] as const),
      articles,
      agents: (agents ?? []).map((one) => [one.id, one.draft.name || one.name] as const),
      agentOn: agent ? agent.agent.enabled && agent.ready : undefined,
      tickets,
    };

    return {
      lists,
      names: {
        teams: new Map(lists.teams),
        members: new Map(lists.members),
        tags: new Map(lists.tags),
        workflows: new Map((workflows ?? []).map((flow) => [flow.id, flow.name] as const)),
        connections: new Map((connections ?? []).map((connection) => [connection.id, connection.name] as const)),
        articles: new Map(articles),
        tickets: ticketNames(tickets),
        agents: new Map((agents ?? []).map((one) => [one.id, one.name] as const)),
      },
    };
  }, [teams, members, tags, agent, agents, workflows, connections, schedules, articles, tickets, current]);
}

/*
 * useArticleChoices reads the published articles of every help center,
 * as the article step picks them.
 */
function useArticleChoices(workspace: string): Choices {
  const centers = useQuery({ ...helpCentersQuery(workspace), retry: false }).data ?? [];
  const first = centers[0];
  const articles = useQuery({ ...articlesQuery(workspace, first?.id ?? ""), enabled: Boolean(first) }).data;

  return useMemo(
    () =>
      (articles ?? [])
        .filter((article) => article.status === "published" && article.visibility === "public")
        .map((article) => [article.id, article.title] as const),
    [articles],
  );
}

const saveWords: Record<SaveState, string> = {
  saved: "Saved",
  saving: "Saving…",
  conflict: "Changed by someone else. Reload to see it.",
  failed: "Not saved",
};

function stateLine(saveState: SaveState, workflow: Workflow): string {
  if (saveState !== "saved") {
    return saveWords[saveState];
  }
  if (workflow.status !== "draft" && workflow.has_unpublished_changes) {
    return "Unpublished changes";
  }

  return workflow.status === "draft" ? "Saved as draft" : "Published";
}

type Draft = ReturnType<typeof useWorkflowDraft>;

/*
 * Publish asks before it puts the working copy in front of customers,
 * as Agent Studio does. It is held back while something needs fixing.
 */
function Publish({ draft }: { draft: Draft }) {
  const [asking, setAsking] = useState(false);
  const { server, busy, report } = draft;
  const changed = server.status === "draft" || server.has_unpublished_changes;
  const blocked = report.errors.length > 0 || draft.saveState === "conflict";
  if (!changed) {
    return null;
  }
  const first = server.status === "draft";
  const name = draft.name || server.name;

  return (
    <>
      <Button size="small" loading={busy} disabled={busy || blocked} onClick={() => setAsking(true)}>
        {first ? "Publish" : "Publish changes"}
      </Button>
      <AlertDialog
        isOpen={asking}
        onOpenChange={setAsking}
        variant="default"
        icon={false}
        title={first ? `Publish ${name}?` : `Publish changes to ${name}?`}
        description={
          first
            ? "It starts running on the next conversation that matches its trigger."
            : "Runs that start from now on follow the new version."
        }
        confirmLabel={first ? "Publish" : "Publish changes"}
        onConfirm={() => {
          setAsking(false);

          return draft.publish();
        }}
      />
    </>
  );
}

/*
 * WorkflowMenu holds what the header does less often: pausing or
 * resuming a published workflow, and archiving it, which asks first.
 */
function WorkflowMenu({ draft }: { draft: Draft }) {
  const [open, setOpen] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const status = draft.server.status;

  return (
    <>
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label="Workflow actions"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover"
        >
          <RiMore2Line aria-hidden className="size-4" />
        </DropdownTrigger>
        <DropdownPopover aria-label="Workflow actions" placement="bottom end" className="w-52">
          {status === "live" ? (
            <DropdownItem
              className="gap-2"
              onSelect={() => {
                setOpen(false);
                void draft.setStatus("paused");
              }}
            >
              <RiPauseCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
              Pause
            </DropdownItem>
          ) : null}
          {status === "paused" ? (
            <DropdownItem
              className="gap-2"
              onSelect={() => {
                setOpen(false);
                void draft.setStatus("live");
              }}
            >
              <RiPlayCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
              Resume
            </DropdownItem>
          ) : null}
          {status === "draft" ? null : <DropdownDivider />}
          <DropdownItem
            className="gap-2 text-text-error-primary"
            onSelect={() => {
              setOpen(false);
              setArchiving(true);
            }}
          >
            <RiArchiveLine aria-hidden className="size-4 shrink-0" />
            Archive workflow
          </DropdownItem>
        </DropdownPopover>
      </Dropdown>
      <AlertDialog
        isOpen={archiving}
        onOpenChange={setArchiving}
        title={`Archive ${draft.name || draft.server.name}?`}
        description="It stops running and leaves the workflows list. Only the API can bring it back."
        confirmLabel="Archive"
        onConfirm={() => {
          setArchiving(false);

          return draft.archive();
        }}
      />
    </>
  );
}

function ProblemCount({ errors, onShow }: { errors: Issue[]; onShow: (issue: Issue) => void }) {
  const first = errors[0];
  if (!first) {
    return null;
  }

  return (
    <Button
      variant="secondary"
      size="small"
      leadingIcon={RiErrorWarningLine}
      onClick={() => onShow(first)}
      className="text-text-error-primary"
    >
      {errors.length === 1 ? "1 to fix" : `${errors.length} to fix`}
    </Button>
  );
}

/*
 * BuildTab is the canvas, or, while Operator's suggestion is under
 * review, the suggested copy with its added and changed steps marked and
 * nothing to add until the teammate decides.
 */
function BuildTab({
  trigger,
  steps,
  moves,
  suggestion,
}: {
  trigger: TriggerKind;
  steps: Step[];
  moves: CanvasMoves;
  suggestion: ReturnType<typeof useWorkflowRequest>;
}) {
  const review = suggestion.review;
  if (!review) {
    return <Canvas trigger={trigger} steps={steps} moves={moves} />;
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
      <Canvas
        trigger={trigger}
        steps={review.proposed.steps}
        moves={{ ...moves, kinds: [], marks: { added: review.added, changed: review.changed } }}
      />
    </div>
  );
}

export function WorkflowBuilder({ workspace, workflow }: { workspace: string; workflow: Workflow }) {
  const navigate = useNavigate();
  const readOnly = !usePermission(workspace, "workflows:manage");
  const draft = useWorkflowDraft(workspace, workflow, () => void navigate({ to: "/d/workflows" }));
  const { lists, names } = useLists(workspace, workflow.id);
  const [tab, setTab] = useState<StudioTabId>("build");
  const [sideTab, setSideTab] = useState<SideTabId>("preview");
  const [selected, setSelected] = useState<string>(triggerStep);
  const trigger = draft.spec.trigger as TriggerKind;
  const step = selected === triggerStep ? undefined : findStep(draft.spec.steps, selected);
  const [inspecting, setInspecting] = useState(useWide(1280));
  const issues = [...draft.report.errors, ...draft.report.warnings];
  const flagged = useMemo(
    () => new Set(draft.report.errors.flatMap((issue) => (issue.step_id ? [issue.step_id] : []))),
    [draft.report],
  );
  const kinds = readOnly ? [] : readyKinds.filter((kind) => allowsStep(trigger, kind));
  const select = (id: string) => {
    setSelected(id);
    setTab("build");
    setInspecting(true);
  };
  const show = (issue: Issue) => select(issue.step_id ?? triggerStep);
  const editSteps = (edit: (steps: Step[]) => Step[]) => draft.change((spec) => ({ ...spec, steps: edit(spec.steps) }));
  const remove = (id: string) => {
    editSteps((steps) => withoutStep(steps, id));
    setSelected(triggerStep);
    setInspecting(false);
  };
  const add = (path: string | null, kind: StepKind) => {
    const fresh = freshStep(kind, draft.spec.steps);
    editSteps((steps) => withStep(steps, path, fresh));
    select(fresh.id);
  };
  const edit = (id: string, change: (step: Step) => Step) => editSteps((steps) => withEdited(steps, id, change));
  const head: Head = {
    name: draft.name,
    onRename: draft.rename,
    onSpec: draft.change,
    onArchive: () => void draft.archive(),
  };
  const summarize = (card: Step) => sentence(card, names);
  const suggestion = useWorkflowRequest(workspace, workflow.id, draft.spec, draft.adopt);
  const moves = { selected, onSelect: select, onAdd: add, kinds, summarize, flagged };
  const stepCounts = useWorkflowStepCounts(workspace, workflow.id);
  const pickWinner = (id: string, variant: string) => {
    edit(id, (current) => withWinner(current, variant));
    toast.success("Winner set in the draft. Publish to send everyone down it.");
  };
  const inspector = {
    results:
      step && workflow.live_version_id ? (
        <StepCounts
          step={step}
          counts={stepCounts.counts.get(step.id)}
          goal={stepCounts.goal}
          onWinner={readOnly ? undefined : (variant) => pickWinner(step.id, variant)}
        />
      ) : null,
    spec: draft.spec,
    step,
    issues: issues.filter((issue) => (step ? issue.step_id === step.id : !issue.step_id)),
    lists,
    head,
    readOnly,
    onRemove: remove,
    onEdit: edit,
  };
  const current = { ...draft.server, name: draft.name || draft.server.name, spec: draft.spec };
  const slot = { workspace, workflow: current, onAdopt: draft.adopt, flush: draft.flush };
  const reading = { steps: draft.spec.steps, summarize, workflows: names.workflows ?? new Map() };
  const Analytics = workflowSlots.analytics;
  const Failures = workflowSlots.failures;
  const History = workflowSlots.history;
  const Simulations = workflowSlots.simulations;
  const Operator = workflowSlots.operator;

  const start = (
    <>
      <Link
        to="/d/workflows"
        aria-label="Back to workflows"
        className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
      </Link>
      <h1 className="min-w-0 truncate px-1.5 text-body-medium text-text-primary">{current.name}</h1>
      <span className="flex shrink-0 items-center gap-1.5">
        <StatusChip status={draft.server.status} />
      </span>
      <span role="status" className="hidden truncate text-body-2-regular text-text-tertiary @lg:inline">
        {stateLine(draft.saveState, draft.server)}
      </span>
    </>
  );
  const end = (
    <>
      <ProblemCount errors={draft.report.errors} onShow={show} />
      {tab === "build" ? (
        <IconButton
          size="small"
          icon={RiSettings3Line}
          aria-label="Step settings"
          aria-pressed={inspecting}
          onClick={() => setInspecting(!inspecting)}
        />
      ) : null}
      {readOnly ? null : <Publish draft={draft} />}
      {readOnly ? null : <WorkflowMenu draft={draft} />}
    </>
  );

  return (
    <Studio
      name="dimmi:workflow"
      label="Workflow"
      start={start}
      end={end}
      tab={tab}
      onTab={setTab}
      body={
        tab === "build" ? (
          <BuildTab trigger={trigger} steps={draft.spec.steps} moves={moves} suggestion={suggestion} />
        ) : (
          <ScrollArea ground className="px-4 pb-6">
            <div className="mx-auto flex w-full max-w-220 min-w-0 flex-col gap-6 pt-4">
              {tab === "analytics" ? (
                <>
                  <Analytics {...slot} />
                  <Failures {...slot} />
                  <RunList workspace={workspace} workflow={workflow.id} reading={reading} />
                </>
              ) : (
                <History {...slot} />
              )}
            </div>
          </ScrollArea>
        )
      }
      inspector={{
        label: "Step settings",
        open: inspecting && tab === "build",
        onOpenChange: setInspecting,
        node: <Inspector {...inspector} onClose={() => setInspecting(false)} className="h-full w-full" />,
      }}
      side={(onClose) => (
        <StudioSide
          tab={sideTab}
          onTab={setSideTab}
          preview={
            <WorkflowPreview
              trigger={trigger}
              steps={draft.spec.steps}
              report={draft.report}
              summarize={summarize}
              onShow={show}
            />
          }
          simulations={<Simulations {...slot} />}
          onClose={onClose}
          className="h-full w-full"
        />
      )}
      operator={(onClose) => <Operator {...slot} onClose={onClose} className="h-full w-full" />}
    />
  );
}
