/*
 * The procedure editor. On the left, the procedure as dimmi reads it:
 * when to use it, the steps in order (an instruction, a condition dimmi
 * weighs before going on, or an action it takes through one of the
 * workspace's published actions) and when to hand over. On the right,
 * whether it is live and how often dimmi followed it, what still stops it
 * from being published, and deleting it. Every change is saved as it is
 * made; publishing makes the saved copy what dimmi follows. Teammates who
 * may only read procedures see everything and change nothing. On screens
 * too narrow for both, the side opens in a drawer from the header.
 */
import {
  RiArrowDownLine,
  RiArrowLeftSLine,
  RiArrowRightLine,
  RiArrowUpLine,
  RiCloseLine,
  RiDeleteBinLine,
  RiFlashlightLine,
  RiGitBranchLine,
  RiSettings3Line,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Textarea } from "@/components/base/textarea/textarea";
import { Action, Danger, Field } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import { Row, Section } from "@/features/settings/layout";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";
import { actionsQuery } from "@/lib/api/ai-actions";
import type { Procedure, ProcedureIssue, ProcedureStep, ProcedureStepKind } from "@/lib/api/procedures";

import type { Choices } from "./choices";
import { KindMark, StatusChip } from "./parts";
import { Pick } from "./pick";
import { withHandover, withNewStep, withoutStepById, withStepChange, withStepMoved } from "./procedures";
import { useProcedureDraft } from "./use-procedure-draft";
import { newId } from "./workflows";

const stepMark = {
  instruction: { icon: RiArrowRightLine, tint: "neutral", name: "Instruction" },
  condition: { icon: RiGitBranchLine, tint: "purple", name: "Condition" },
  action: { icon: RiFlashlightLine, tint: "lime", name: "Action" },
} as const;

const kindChoices: Choices = [
  ["instruction", "Instruction"],
  ["condition", "Condition"],
  ["action", "Action"],
];

type Draft = ReturnType<typeof useProcedureDraft>;

function StepLine({
  step,
  index,
  last,
  actions,
  issues,
  draft,
}: {
  step: ProcedureStep;
  index: number;
  last: boolean;
  actions: Choices;
  issues: ProcedureIssue[];
  draft: Draft;
}) {
  const look = stepMark[step.kind];
  const edit = (change: Partial<ProcedureStep>) => draft.change((d) => withStepChange(d, step.id, change));

  return (
    <div className="flex items-start gap-3 border-b border-separator-border py-2.5 pe-2.5 last:border-b-0">
      <span className="flex w-5 shrink-0 justify-end pt-2 text-body-2-medium text-text-tertiary tabular-nums">
        {index + 1}
      </span>
      <span className="pt-1">
        <KindMark icon={look.icon} tint={look.tint} size="sm" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <Textarea
          aria-label={`${look.name} ${index + 1}`}
          defaultValue={step.text}
          placeholder={step.kind === "condition" ? "If …, then …" : "What dimmi does"}
          rows={1}
          autoResize
          maxRows={6}
          onChange={(text) => edit({ text })}
        />
        <span className="flex flex-wrap items-center gap-2">
          <Pick
            label={`Step ${index + 1} kind`}
            choices={kindChoices}
            value={step.kind}
            onChange={(kind) => edit({ kind: kind as ProcedureStepKind })}
          />
          {step.kind === "action" ? (
            <Pick
              key={step.action_id ?? "none"}
              label={`Step ${index + 1} action`}
              choices={actions}
              placeholder={actions.length === 0 ? "No published actions" : "Pick an action"}
              value={step.action_id ?? ""}
              onChange={(action) => edit({ action_id: action })}
            />
          ) : null}
        </span>
        {issues.map((issue) => (
          <span key={issue.message} className="text-body-2-regular text-text-error-primary">
            {issue.message}
          </span>
        ))}
      </div>
      <span className="flex shrink-0 items-center gap-0.5 pt-0.5">
        <IconButton
          size="small"
          icon={RiArrowUpLine}
          aria-label={`Move step ${index + 1} up`}
          disabled={index === 0}
          onClick={() => draft.change((d) => withStepMoved(d, step.id, -1))}
        />
        <IconButton
          size="small"
          icon={RiArrowDownLine}
          aria-label={`Move step ${index + 1} down`}
          disabled={last}
          onClick={() => draft.change((d) => withStepMoved(d, step.id, 1))}
        />
        <IconButton
          size="small"
          icon={RiDeleteBinLine}
          aria-label={`Remove step ${index + 1}`}
          onClick={() => draft.change((d) => withoutStepById(d, step.id))}
        />
      </span>
    </div>
  );
}

/*
 * Handovers lists when to hand over. Handovers are plain lines, so the
 * list keeps a key for each line itself, added and dropped with it.
 */
function Handovers({ draft }: { draft: Draft }) {
  const lines = draft.definition.handover;
  const [keys, setKeys] = useState(() => lines.map(() => newId("p_")));
  const add = () => {
    setKeys((before) => [...before, newId("p_")]);
    draft.change((d) => withHandover(d, d.handover.length, ""));
  };
  const remove = (index: number) => {
    setKeys((before) => before.filter((_, at) => at !== index));
    draft.change((d) => withHandover(d, index, null));
  };

  return (
    <Section
      title="Hand over to the team when"
      description="dimmi also hands over when the conversation goes in circles."
      action={<Action onPress={add}>Add</Action>}
    >
      {lines.map((line, index) => (
        <div key={keys[index] ?? line} className="flex items-start gap-2 py-2.5 pe-2.5">
          <div className="min-w-0 flex-1">
            <Textarea
              aria-label={`Handover ${index + 1}`}
              defaultValue={line}
              placeholder="The customer …"
              rows={1}
              autoResize
              maxRows={4}
              onChange={(text) => draft.change((d) => withHandover(d, index, text))}
            />
          </div>
          <IconButton
            size="small"
            icon={RiDeleteBinLine}
            aria-label={`Remove handover ${index + 1}`}
            onClick={() => remove(index)}
          />
        </div>
      ))}
    </Section>
  );
}

function Document({ draft, actions }: { draft: Draft; actions: Choices }) {
  const { definition, issues } = draft;
  const general = issues.filter((issue) => !issue.step_id && issue.field === "description");

  return (
    <div className="mx-auto flex w-full max-w-[680px] flex-col gap-8 px-6 pt-8 pb-16">
      <Section title="Name">
        <Row label="Name">
          <Field label="Procedure name" value={draft.name} onChange={draft.rename} />
        </Row>
      </Section>
      <Section title="When dimmi uses this" description="dimmi follows this procedure when a request matches.">
        <div className="py-2.5 pe-2.5">
          <Textarea
            aria-label="When dimmi uses this"
            defaultValue={definition.description}
            placeholder="The customer cannot sign in because …"
            rows={2}
            autoResize
            maxRows={6}
            onChange={(description) => draft.change((d) => ({ ...d, description }))}
          />
          {general.map((issue) => (
            <p key={issue.message} className="pt-1 text-body-2-regular text-text-error-primary">
              {issue.message}
            </p>
          ))}
        </div>
      </Section>
      <Section
        title="Steps"
        description="In order. Conditions decide what happens next; actions run in your systems."
        action={
          <span className="flex gap-2">
            <Action onPress={() => draft.change((d) => withNewStep(d, "instruction"))}>Instruction</Action>
            <Action onPress={() => draft.change((d) => withNewStep(d, "condition"))}>Condition</Action>
            <Action onPress={() => draft.change((d) => withNewStep(d, "action"))}>Action</Action>
          </span>
        }
      >
        {definition.steps.length === 0 ? (
          <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">No steps yet. Add the first one.</p>
        ) : null}
        {definition.steps.map((step, index) => (
          <StepLine
            key={step.id}
            step={step}
            index={index}
            last={index === definition.steps.length - 1}
            actions={actions}
            issues={issues.filter((issue) => issue.step_id === step.id)}
            draft={draft}
          />
        ))}
      </Section>
      <Handovers draft={draft} />
    </div>
  );
}

function Side({ draft, onClose, className }: { draft: Draft; onClose?: () => void; className?: string }) {
  const { server, issues } = draft;
  const rest = issues.filter((issue) => !issue.step_id && issue.field !== "description");

  return (
    <Panel label="Procedure settings" ground className={className}>
      <PageHeader title="Settings" level={2}>
        {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close settings" onClick={onClose} /> : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <div className="flex flex-col gap-6 pt-2">
          <Section title="Status">
            <Row
              label={server.status === "live" ? "dimmi follows this" : "Not live"}
              description={
                server.status === "live"
                  ? `Followed in ${server.uses} conversations in the last 30 days.`
                  : "Publish it for dimmi to follow."
              }
            />
          </Section>
          {rest.length > 0 ? (
            <Section title="To fix before publishing">
              {rest.map((issue) => (
                <Row key={issue.message} label={issue.message} />
              ))}
            </Section>
          ) : null}
          <Section title="Delete">
            <Row label="Delete this procedure" description="dimmi stops following it at once.">
              <Danger onPress={() => void draft.remove()}>Delete</Danger>
            </Row>
          </Section>
        </div>
      </ScrollArea>
    </Panel>
  );
}

function saveLine(draft: Draft): string {
  const words = { saving: "Saving…", conflict: "Changed by someone else. Reload to see it.", failed: "Not saved" };
  if (draft.state !== "saved") {
    return words[draft.state];
  }
  if (draft.server.status === "live") {
    return draft.server.has_unpublished_changes ? "Unpublished changes" : "Published";
  }

  return "Saved as draft";
}

function Actions({ draft }: { draft: Draft }) {
  const { server, busy } = draft;
  const changed = server.status === "draft" || server.has_unpublished_changes;

  return (
    <>
      {server.status === "live" ? (
        <Button variant="secondary" size="small" disabled={busy} onClick={() => void draft.unpublish()}>
          Turn off
        </Button>
      ) : null}
      {changed ? (
        <Button
          size="small"
          loading={busy}
          disabled={busy || draft.state === "conflict"}
          onClick={() => void draft.publish()}
        >
          {server.status === "live" ? "Publish changes" : "Publish"}
        </Button>
      ) : null}
    </>
  );
}

export function ProcedureEditor({ workspace, procedure }: { workspace: string; procedure: Procedure }) {
  const navigate = useNavigate();
  const readOnly = !usePermission(workspace, "ai_agent:manage");
  const draft = useProcedureDraft(workspace, procedure, () => void navigate({ to: "/d/procedures" }));
  const published = useQuery({ ...actionsQuery(workspace, { status: "published" }), retry: false }).data;
  const actions: Choices = (published ?? []).map((a) => [a.id, a.title || a.name] as const);
  const beside = useWide(1280);
  const [drawer, setDrawer] = useState(false);

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 gap-3">
      <Panel label="Procedure" ground className="h-full flex-1">
        <header className="flex min-h-12 shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 ps-2.5 pe-2.5 pt-[7px]">
          <div className="flex min-w-40 flex-1 items-center gap-2">
            <Link
              to="/d/procedures"
              aria-label="Back to procedures"
              className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
            </Link>
            <h1 className="min-w-0 truncate text-body-medium text-text-primary">{draft.name || procedure.name}</h1>
            <StatusChip status={draft.server.status} />
            <span className="hidden truncate text-body-2-regular text-text-tertiary sm:inline">{saveLine(draft)}</span>
          </div>
          <div className="ms-auto flex shrink-0 items-center gap-2">
            {beside ? null : (
              <IconButton
                size="small"
                icon={RiSettings3Line}
                aria-label="Procedure settings"
                onClick={() => setDrawer(true)}
              />
            )}
            {readOnly ? null : <Actions draft={draft} />}
          </div>
        </header>
        <ScrollArea ground>
          <fieldset disabled={readOnly} className="min-w-0">
            <Document draft={draft} actions={actions} />
          </fieldset>
        </ScrollArea>
      </Panel>
      {beside ? (
        <Side draft={draft} className="h-full w-[360px] shrink-0" />
      ) : (
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Procedure settings"
          className="p-0"
        >
          <Side draft={draft} onClose={() => setDrawer(false)} className="h-full" />
        </Drawer>
      )}
    </div>
  );
}
