/*
 * The workflow builder: the canvas of steps on the left and the
 * inspector for the selected step on the right, under a header with the
 * workflow's name, its status, whether its changes are saved, what still
 * needs fixing, and the buttons that publish, pause or resume it. Every
 * change is saved to the working copy as it is made; publishing makes
 * that copy the version that runs. Steps with problems carry a mark, and
 * the problems count jumps to the first of them. Teammates who may only
 * read workflows see the same builder without anything to change. On
 * screens too narrow for both, the inspector opens in a drawer when a
 * card is picked, or from the header.
 */
import { RiArrowLeftSLine, RiErrorWarningLine, RiSettings3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useMemo, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { usePermission } from "@/features/settings/inboxes";
import { Panel } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";
import { tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";
import type { Issue, Workflow } from "@/lib/api/workflows";
import { membersQuery } from "@/lib/api/workspace";

import { Canvas } from "./canvas";
import { freshStep, withEdited, withoutStep, withStep } from "./edit-steps";
import { type Head, Inspector } from "./inspector";
import { StatusChip } from "./parts";
import type { Lists } from "./step-fields";
import { type Names, sentence } from "./step-settings";
import { allowsStep, type TriggerKind } from "./triggers";
import { type SaveState, useWorkflowDraft } from "./use-workflow-draft";
import { findStep, readyKinds, type Step, type StepKind, triggerStep } from "./workflows";

/*
 * useLists reads the workspace's teams, active teammates and tags, as the
 * pickers offer them and as the cards name them.
 */
function useLists(workspace: string): { lists: Lists; names: Names } {
  const teams = useQuery(teamsQuery(workspace)).data;
  const members = useQuery(membersQuery(workspace)).data;
  const tags = useQuery(tagsQuery(workspace)).data;

  return useMemo(() => {
    const lists: Lists = {
      teams: (teams ?? []).map((team) => [team.id, team.name] as const),
      members: (members ?? [])
        .filter((member) => member.status === "active")
        .map((member) => [member.id, member.user.name] as const),
      tags: (tags ?? []).map((tag) => [tag.id, tag.name] as const),
    };

    return {
      lists,
      names: { teams: new Map(lists.teams), members: new Map(lists.members), tags: new Map(lists.tags) },
    };
  }, [teams, members, tags]);
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
        <Button size="small" disabled={busy || blocked} onClick={() => void draft.publish()}>
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
      variant="ghost"
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
  const { lists, names } = useLists(workspace);
  const [selected, setSelected] = useState<string>(triggerStep);
  const trigger = draft.spec.trigger as TriggerKind;
  const step = selected === triggerStep ? undefined : findStep(draft.spec.steps, selected);
  const beside = useWide(1280);
  const [drawer, setDrawer] = useState(false);
  const issues = [...draft.report.errors, ...draft.report.warnings];
  const flagged = useMemo(
    () => new Set(draft.report.errors.flatMap((issue) => (issue.step_id ? [issue.step_id] : []))),
    [draft.report],
  );
  const kinds = readOnly ? [] : readyKinds.filter((kind) => allowsStep(trigger, kind));
  const select = (id: string) => {
    setSelected(id);
    setDrawer(!beside);
  };
  const editSteps = (edit: (steps: Step[]) => Step[]) => draft.change((spec) => ({ ...spec, steps: edit(spec.steps) }));
  const remove = (id: string) => {
    editSteps((steps) => withoutStep(steps, id));
    setSelected(triggerStep);
    setDrawer(false);
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

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 gap-3">
      <Panel label="Workflow" ground className="h-full flex-1">
        <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
          <div className="flex min-w-0 items-center gap-2">
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
          <div className="flex shrink-0 items-center gap-2">
            <ProblemCount errors={draft.report.errors} onShow={(issue) => select(issue.step_id ?? triggerStep)} />
            {beside ? null : (
              <IconButton
                size="small"
                icon={RiSettings3Line}
                aria-label="Step settings"
                onClick={() => setDrawer(true)}
              />
            )}
            <Actions draft={draft} readOnly={readOnly} />
          </div>
        </header>
        <Canvas trigger={trigger} steps={draft.spec.steps} moves={moves} />
      </Panel>
      {beside ? (
        <Inspector {...inspector} className="h-full w-[360px] shrink-0" />
      ) : (
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Step settings"
          className="p-0"
        >
          <Inspector {...inspector} onClose={() => setDrawer(false)} className="h-full" />
        </Drawer>
      )}
    </div>
  );
}
