/*
 * The workflow builder: the canvas of steps on the left and the
 * inspector for the selected step on the right, under a header with the
 * workflow's name, its status, whether its changes are saved, what still
 * needs fixing, and the buttons that publish, pause or resume it. Every
 * change is saved to the working copy as it is made; publishing makes
 * that copy the version that runs. Steps with problems carry a mark, and
 * the problems count jumps to the first of them. Teammates who may only
 * read workflows see the same builder without anything to change.
 * Picking a card, or the settings button, pushes the inspector out from
 * the right in a pane that drags to any width; on screens too narrow for
 * both, it opens in a drawer. The runs button shows the workflow's
 * latest runs in the same pane, each opening to what its steps did.
 */
import { RiArrowLeftSLine, RiErrorWarningLine, RiHistoryLine, RiSettings3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useMemo, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { usePermission } from "@/features/settings/inboxes";
import { Panel } from "@/features/shell/page";
import { PushPane } from "@/features/shell/push-pane";
import { useWide } from "@/features/shell/use-wide";
import { agentQuery } from "@/lib/api/ai";
import { actionConnectionsQuery } from "@/lib/api/ai-actions";
import { articlesQuery, helpCentersQuery } from "@/lib/api/help-centers";
import { schedulesQuery } from "@/lib/api/schedules";
import { tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";
import { type Issue, type Workflow, workflowsQuery } from "@/lib/api/workflows";
import { membersQuery } from "@/lib/api/workspace";

import { Canvas } from "./canvas";
import type { Choices } from "./choices";
import { freshStep, withEdited, withoutStep, withStep } from "./edit-steps";
import { type Head, Inspector } from "./inspector";
import { StatusChip } from "./parts";
import type { Lists } from "./step-fields";
import { type Names, sentence } from "./step-settings";
import { ticketNames, useTicketLists } from "./ticket-lists";
import { allowsStep, type TriggerKind } from "./triggers";
import { type SaveState, useWorkflowDraft } from "./use-workflow-draft";
import { WorkflowRuns } from "./workflow-runs";
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
      },
    };
  }, [teams, members, tags, agent, workflows, connections, schedules, articles, tickets, current]);
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

function Actions({ draft, readOnly }: { draft: ReturnType<typeof useWorkflowDraft>; readOnly: boolean }) {
  if (readOnly) {
    return null;
  }
  const { server, busy, report } = draft;
  const changed = server.status === "draft" || server.has_unpublished_changes;
  const blocked = report.errors.length > 0 || draft.saveState === "conflict";

  return (
    <>
      {server.status === "live" ? (
        <Button variant="secondary" size="small" disabled={busy} onClick={() => void draft.setStatus("paused")}>
          Pause
        </Button>
      ) : null}
      {server.status === "paused" ? (
        <Button variant="secondary" size="small" disabled={busy} onClick={() => void draft.setStatus("live")}>
          Resume
        </Button>
      ) : null}
      {changed ? (
        <Button size="small" loading={busy} disabled={busy || blocked} onClick={() => void draft.publish()}>
          {server.status === "draft" ? "Publish" : "Publish changes"}
        </Button>
      ) : null}
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

export function WorkflowBuilder({ workspace, workflow }: { workspace: string; workflow: Workflow }) {
  const navigate = useNavigate();
  const readOnly = !usePermission(workspace, "workflows:manage");
  const draft = useWorkflowDraft(workspace, workflow, () => void navigate({ to: "/d/workflows" }));
  const { lists, names } = useLists(workspace, workflow.id);
  const [showing, setShowing] = useState<"settings" | "runs">("settings");
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
    setShowing("settings");
    setInspecting(true);
  };
  const toggle = (pane: "settings" | "runs") => {
    setInspecting(!(inspecting && showing === pane));
    setShowing(pane);
  };
  const editSteps = (edit: (steps: Step[]) => Step[]) => draft.change((spec) => ({ ...spec, steps: edit(spec.steps) }));
  const remove = (id: string) => {
    editSteps((steps) => withoutStep(steps, id));
    setSelected(triggerStep);
    setInspecting(false);
  };
  const add = (path: string | null, kind: StepKind) => {
    const fresh = freshStep(kind);
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
  const moves = {
    selected,
    onSelect: select,
    onAdd: add,
    kinds,
    summarize: (card: Step) => sentence(card, names),
    flagged,
  };
  const inspector = {
    spec: draft.spec,
    step,
    issues: issues.filter((issue) => (step ? issue.step_id === step.id : !issue.step_id)),
    lists,
    head,
    readOnly,
    onRemove: remove,
    onEdit: edit,
  };

  const canvas = (
    <Panel label="Workflow" ground className="h-full w-full">
      <header className="flex min-h-12 shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 ps-2.5 pe-2.5 pt-[7px]">
        <div className="flex min-w-40 flex-1 items-center gap-2">
          <Link
            to="/d/workflows"
            aria-label="Back to workflows"
            className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
          </Link>
          <h1 className="min-w-0 truncate text-body-medium text-text-primary">{draft.name || workflow.name}</h1>
          <StatusChip status={draft.server.status} />
          <span role="status" className="hidden truncate text-body-2-regular text-text-tertiary sm:inline">
            {stateLine(draft.saveState, draft.server)}
          </span>
        </div>
        <div className="ms-auto flex shrink-0 items-center gap-2">
          <ProblemCount errors={draft.report.errors} onShow={(issue) => select(issue.step_id ?? triggerStep)} />
          <Button
            variant="secondary"
            size="small"
            leadingIcon={RiHistoryLine}
            aria-pressed={inspecting && showing === "runs"}
            onClick={() => toggle("runs")}
          >
            Runs
          </Button>
          <IconButton
            size="small"
            icon={RiSettings3Line}
            aria-label="Step settings"
            onClick={() => toggle("settings")}
          />
          <Actions draft={draft} readOnly={readOnly} />
        </div>
      </header>
      <Canvas trigger={trigger} steps={draft.spec.steps} moves={moves} />
    </Panel>
  );

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1">
      <PushPane
        name="dimmi:workflow"
        label={showing === "runs" ? "Recent runs" : "Step settings"}
        open={inspecting}
        onOpenChange={setInspecting}
        width={380}
        main={canvas}
        side={
          showing === "runs" ? (
            <WorkflowRuns
              workspace={workspace}
              workflow={workflow.id}
              reading={{ steps: draft.spec.steps, summarize: moves.summarize, workflows: names.workflows ?? new Map() }}
              onClose={() => setInspecting(false)}
              className="h-full w-full"
            />
          ) : (
            <Inspector {...inspector} onClose={() => setInspecting(false)} className="h-full w-full" />
          )
        }
      />
    </div>
  );
}
