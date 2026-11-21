/*
 * The procedure editor. On the left, the procedure as dimmi reads it:
 * when to use it, where, the steps in order and when to hand over. A
 * step's words take @ chips for actions, customer details, articles,
 * inbox actions and sub-procedures; a condition is either a rule dimmi
 * decides exactly, with then and else steps under it, or plain words the
 * AI weighs. Beside the steps, the simulator and saved tests, the
 * published versions and how the procedure did. On the right, whether
 * it is live and how often dimmi followed it, what still stops it from
 * being published, and deleting it. Every change is saved as it is
 * made; publishing makes the saved copy what dimmi follows. Teammates
 * who may only read procedures see everything and change nothing. On
 * screens too narrow for both, the side opens in a drawer from the
 * header.
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
  RiListOrdered2,
  RiSettings3Line,
  RiSplitCellsHorizontal,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Badge } from "@/components/base/badges/badge";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Drawer } from "@/components/base/drawer/drawer";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { Textarea } from "@/components/base/textarea/textarea";
import { Action, Danger, Field } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import { Row, Section } from "@/features/settings/layout";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";
import { audiencesQuery } from "@/lib/api/audiences";
import { inboxesQuery } from "@/lib/api/inboxes";
import {
  type Procedure,
  type ProcedureDefinition,
  type ProcedureIssue,
  type ProcedureScope,
  type ProcedureStep,
  type ProcedureStepKind,
  testCasesQuery,
} from "@/lib/api/procedures";
import { countOf } from "@/lib/i18n/format";

import type { Choices } from "./choices";
import { KindMark, StatusChip } from "./parts";
import { Pick, Picks } from "./pick";
import { ProcedureAnalytics } from "./procedure-analytics";
import { ProcedureHistory } from "./procedure-history";
import { ProcedureTests } from "./procedure-simulator/procedure-tests";
import { ConditionBuilder } from "./procedure-writer/condition-builder";
import { type ChipSources, fieldsOf, labelOf } from "./procedure-writer/procedure-suggestions";
import { StepWriter } from "./procedure-writer/step-writer";
import { useChipSources } from "./procedure-writer/use-chip-sources";
import {
  earlierActions,
  stepNumbers,
  type Where,
  withHandover,
  withNewStep,
  withoutStepById,
  withStepChange,
  withStepMoved,
} from "./procedures";
import { useProcedureDraft } from "./use-procedure-draft";
import { newId } from "./workflows";

const stepMark = {
  instruction: { icon: RiArrowRightLine, tint: "neutral", name: "Instruction" },
  condition: { icon: RiGitBranchLine, tint: "purple", name: "Condition" },
  action: { icon: RiFlashlightLine, tint: "lime", name: "Action" },
  branch: { icon: RiSplitCellsHorizontal, tint: "purple", name: "Branch" },
  sub_procedure: { icon: RiListOrdered2, tint: "blue", name: "Sub-procedure" },
} as const;

const kindChoices: Choices = [
  ["instruction", "Instruction"],
  ["condition", "Condition"],
  ["action", "Action"],
  ["sub_procedure", "Sub-procedure"],
];

/*
 * maxDepth is how deep then and else steps nest, for readability.
 */
const maxDepth = 2;

type Draft = ReturnType<typeof useProcedureDraft>;

type Tabbed = "steps" | "test" | "history" | "analytics";

/*
 * Context is what every step reads: the draft, what chips can name, the
 * workspace's published actions, the issues to show and the numbers of
 * the steps.
 */
interface Context {
  draft: Draft;
  sources: ChipSources;
  actions: Choices;
  issues: ProcedureIssue[];
  numbers: Map<string, string>;
  readOnly: boolean;
}

const decideChoices = [
  ["rule", "Rule (exact)"],
  ["words", "Plain English (AI)"],
] as const;

/*
 * isRule reports whether a condition is decided in code.
 */
function isRule(step: ProcedureStep): boolean {
  return step.kind === "branch" || (step.kind === "condition" && step.condition !== undefined);
}

function RuleToggle({ step, ctx }: { step: ProcedureStep; ctx: Context }) {
  const edit = (change: Partial<ProcedureStep>) => ctx.draft.change((d) => withStepChange(d, step.id, change));

  return (
    <SegmentedControl
      aria-label="How dimmi decides"
      selectedKeys={new Set([isRule(step) ? "rule" : "words"])}
      onSelectionChange={(keys) => {
        const rule = [...keys][0] === "rule";
        edit(
          rule
            ? {
                condition: step.condition ?? { mode: "all", rules: [{ field: "", operator: "eq", value: "" }] },
                then_steps: step.then_steps ?? [],
                else_steps: step.else_steps ?? [],
              }
            : { condition: undefined, kind: "condition" },
        );
      }}
    >
      {decideChoices.map(([id, name]) => (
        <SegmentedControlItem key={id} id={id}>
          {name}
        </SegmentedControlItem>
      ))}
    </SegmentedControl>
  );
}

function Side({
  step,
  side,
  depth,
  ctx,
}: {
  step: ProcedureStep;
  side: "then_steps" | "else_steps";
  depth: number;
  ctx: Context;
}) {
  const steps = step[side] ?? [];
  const where: Where = { parent: step.id, side };
  const title = side === "then_steps" ? (isRule(step) ? "Then" : "If so") : isRule(step) ? "Else" : "If not";

  return (
    <div className="flex flex-col gap-1 border-separator-border border-s-2 ps-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-body-2-medium text-text-secondary">{title}</span>
        {ctx.readOnly ? null : (
          <Action size="xs" onPress={() => ctx.draft.change((d) => withNewStep(d, "instruction", where))}>
            Step
          </Action>
        )}
      </div>
      {steps.length === 0 ? <p className="text-body-2-regular text-text-tertiary">No steps: dimmi goes on.</p> : null}
      {steps.map((child, index) => (
        <StepCard
          key={child.id}
          step={child}
          first={index === 0}
          last={index === steps.length - 1}
          depth={depth}
          ctx={ctx}
        />
      ))}
    </div>
  );
}

/*
 * placeholderOf is what an empty step's writer says.
 */
function placeholderOf(step: ProcedureStep): string {
  if (isRule(step)) {
    return "Name this rule, such as Inside the 30-day window";
  }
  if (step.kind === "condition") {
    return "If …, then …  (type @ for actions and details)";
  }
  return "What dimmi does (type @ for actions, details, articles)";
}

/*
 * StepWords is a step's writer, or its sub-procedure picker, with the
 * rule of a condition decided in code.
 */
function StepWords({ step, number, ctx }: { step: ProcedureStep; number: string; ctx: Context }) {
  const edit = (change: Partial<ProcedureStep>) => ctx.draft.change((d) => withStepChange(d, step.id, change));
  const look = stepMark[step.kind] ?? stepMark.instruction;
  const procedures: Choices = ctx.sources.procedures.map((p) => [p.id, p.name] as const);
  const earlier = earlierActions(ctx.draft.definition, step.id).map(
    (id) => [id, labelOf(ctx.sources, { kind: "action", ref: id }) || id] as const,
  );

  return (
    <>
      {step.kind === "condition" || step.kind === "branch" ? <RuleToggle step={step} ctx={ctx} /> : null}
      {step.kind === "sub_procedure" ? (
        <Pick
          key={step.sub_procedure_id ?? "none"}
          label={`Step ${number} sub-procedure`}
          choices={procedures}
          placeholder={procedures.length === 0 ? "No live procedures" : "Pick a procedure"}
          value={step.sub_procedure_id ?? ""}
          onChange={(id) => edit({ sub_procedure_id: id })}
        />
      ) : (
        <StepWriter
          step={step}
          label={`${look.name} ${number}`}
          placeholder={placeholderOf(step)}
          sources={ctx.sources}
          earlier={earlier}
          readOnly={ctx.readOnly}
          onChange={edit}
        />
      )}
      {isRule(step) && step.condition ? (
        <ConditionBuilder
          condition={step.condition}
          fields={fieldsOf(ctx.sources)}
          onChange={(condition) => edit({ condition })}
        />
      ) : null}
    </>
  );
}

/*
 * StepKind picks a step's kind, and a legacy action step's action.
 */
function StepKind({ step, number, ctx }: { step: ProcedureStep; number: string; ctx: Context }) {
  const edit = (change: Partial<ProcedureStep>) => ctx.draft.change((d) => withStepChange(d, step.id, change));
  if (step.kind === "branch") {
    return null;
  }

  return (
    <span className="flex flex-wrap items-center gap-2">
      <Pick
        label={`Step ${number} kind`}
        choices={kindChoices}
        value={step.kind}
        onChange={(kind) => edit({ kind: kind as ProcedureStepKind })}
      />
      {step.kind === "action" ? (
        <Pick
          key={step.action_id ?? "none"}
          label={`Step ${number} action`}
          choices={ctx.actions}
          placeholder={ctx.actions.length === 0 ? "No published actions" : "Pick an action"}
          value={step.action_id ?? ""}
          onChange={(action) => edit({ action_id: action })}
        />
      ) : null}
    </span>
  );
}

/*
 * StepTools moves and removes a step.
 */
function StepTools({
  step,
  number,
  first,
  last,
  ctx,
}: {
  step: ProcedureStep;
  number: string;
  first: boolean;
  last: boolean;
  ctx: Context;
}) {
  if (ctx.readOnly) {
    return null;
  }
  const { draft } = ctx;

  return (
    <span className="flex shrink-0 items-center gap-0.5 pt-0.5">
      <IconButton
        size="small"
        icon={RiArrowUpLine}
        aria-label={`Move step ${number} up`}
        disabled={first}
        onClick={() => draft.change((d) => withStepMoved(d, step.id, -1))}
      />
      <IconButton
        size="small"
        icon={RiArrowDownLine}
        aria-label={`Move step ${number} down`}
        disabled={last}
        onClick={() => draft.change((d) => withStepMoved(d, step.id, 1))}
      />
      <IconButton
        size="small"
        icon={RiDeleteBinLine}
        aria-label={`Remove step ${number}`}
        onClick={() => draft.change((d) => withoutStepById(d, step.id))}
      />
    </span>
  );
}

function StepCard({
  step,
  first,
  last,
  depth,
  ctx,
}: {
  step: ProcedureStep;
  first: boolean;
  last: boolean;
  depth: number;
  ctx: Context;
}) {
  const look = stepMark[step.kind] ?? stepMark.instruction;
  const number = ctx.numbers.get(step.id) ?? "";
  const issues = ctx.issues.filter((issue) => issue.step_id === step.id);
  const nests = (step.kind === "condition" || step.kind === "branch") && depth < maxDepth;

  return (
    <div className="flex items-start gap-3 border-b border-separator-border py-2.5 pe-2.5 last:border-b-0">
      <span className="flex min-w-6 shrink-0 justify-end pt-2 text-body-2-medium text-text-tertiary tabular-nums">
        {number}
      </span>
      <span className="pt-1">
        <KindMark icon={look.icon} tint={look.tint} size="sm" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <StepWords step={step} number={number} ctx={ctx} />
        <StepKind step={step} number={number} ctx={ctx} />
        {nests ? (
          <div className="flex flex-col gap-2">
            <Side step={step} side="then_steps" depth={depth + 1} ctx={ctx} />
            <Side step={step} side="else_steps" depth={depth + 1} ctx={ctx} />
          </div>
        ) : null}
        {issues.map((issue) => (
          <span key={issue.message} className="text-body-2-regular text-text-error-primary">
            {issue.message}
          </span>
        ))}
      </div>
      <StepTools step={step} number={number} first={first} last={last} ctx={ctx} />
    </div>
  );
}

/*
 * Handovers lists when to hand over. Handovers are plain lines, so the
 * list keeps a key for each line itself, added and dropped with it.
 */
function Handovers({ draft, teams }: { draft: Draft; teams: Choices }) {
  const lines = draft.definition.handover;
  const [keys, setKeys] = useState(() => lines.map(() => newId("p_")));
  const rule = draft.definition.handover_rule ?? {};
  const add = () => {
    setKeys((before) => [...before, newId("p_")]);
    draft.change((d) => withHandover(d, d.handover.length, ""));
  };
  const remove = (index: number) => {
    setKeys((before) => before.filter((_, at) => at !== index));
    draft.change((d) => withHandover(d, index, null));
  };
  const route = (change: Partial<NonNullable<ProcedureDefinition["handover_rule"]>>) =>
    draft.change((d) => ({ ...d, handover_rule: { ...d.handover_rule, ...change } }));

  return (
    <Section
      title="Hand over to the team when"
      description="dimmi also hands over when the conversation goes in circles, or a required action keeps being skipped."
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
      <Row label="Route handovers to" description="The team and priority a handover from this procedure gets.">
        <Pick
          label="Handover team"
          choices={teams}
          placeholder={teams.length === 0 ? "No teams" : "Any team"}
          value={rule.team_id ?? ""}
          onChange={(team) => route({ team_id: team })}
        />
        <Pick
          label="Handover priority"
          choices={[
            ["low", "Low"],
            ["medium", "Medium"],
            ["high", "High"],
            ["urgent", "Urgent"],
          ]}
          placeholder="Priority as is"
          value={rule.priority ?? ""}
          onChange={(priority) => route({ priority: priority as "low" })}
        />
      </Row>
    </Section>
  );
}

const channelChoices: Choices = [
  ["widget", "Messenger"],
  ["email", "Email"],
  ["slack", "Slack"],
  ["whatsapp", "WhatsApp"],
  ["instagram", "Instagram"],
  ["facebook", "Facebook"],
  ["api", "API"],
];

/*
 * Scope narrows where dimmi offers the procedure: channels, inboxes,
 * saved audiences, signed-in customers only, or only from inside other
 * procedures.
 */
function Scope({ draft, workspace }: { draft: Draft; workspace: string }) {
  const scope = draft.definition.scope ?? {};
  const inboxes = useQuery({ ...inboxesQuery(workspace), retry: false }).data ?? [];
  const audiences = useQuery({ ...audiencesQuery(workspace), retry: false }).data ?? [];
  const set = (change: Partial<ProcedureScope>) => draft.change((d) => ({ ...d, scope: { ...d.scope, ...change } }));

  return (
    <Section title="Where it applies" description="Empty lists mean everywhere dimmi answers.">
      <div className="flex flex-col gap-1 py-2.5 pe-2.5">
        <span className="px-3 text-body-2-medium text-text-secondary">Channels</span>
        <Picks
          choices={channelChoices}
          value={scope.channels ?? []}
          empty=""
          onChange={(channels) => set({ channels })}
        />
      </div>
      <div className="flex flex-col gap-1 py-2.5 pe-2.5">
        <span className="px-3 text-body-2-medium text-text-secondary">Inboxes</span>
        <Picks
          choices={inboxes.map((i) => [i.id, i.name] as const)}
          value={scope.inbox_ids ?? []}
          empty="No inboxes."
          onChange={(inbox_ids) => set({ inbox_ids })}
        />
      </div>
      <div className="flex flex-col gap-1 py-2.5 pe-2.5">
        <span className="px-3 text-body-2-medium text-text-secondary">Saved audiences</span>
        <Picks
          choices={audiences.map((a) => [a.id, a.name] as const)}
          value={scope.audience_ids ?? []}
          empty="No saved audiences."
          onChange={(audience_ids) => set({ audience_ids })}
        />
      </div>
      <div className="flex flex-col gap-2 px-3 py-2.5">
        <Checkbox
          isSelected={scope.require_verified_identity ?? false}
          onChange={(on) => set({ require_verified_identity: on })}
        >
          Only for signed-in customers
        </Checkbox>
        <Checkbox isSelected={scope.reusable_only ?? false} onChange={(on) => set({ reusable_only: on })}>
          Only as a sub-procedure of other procedures
        </Checkbox>
      </div>
    </Section>
  );
}

function Document({ draft, ctx, workspace }: { draft: Draft; ctx: Context; workspace: string }) {
  const { definition, issues } = draft;
  const general = issues.filter((issue) => !issue.step_id && issue.field === "description");
  const teams: Choices = ctx.sources.teams.map((t) => [t.id, t.name] as const);
  const add = (kind: ProcedureStepKind) => draft.change((d) => withNewStep(d, kind));

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-8 pb-16">
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
        description="In order. Type @ to call an action, read a detail, cite an article or follow a sub-procedure. Conditions decide what happens next."
        action={
          ctx.readOnly ? null : (
            <span className="flex flex-wrap gap-2">
              <Action onPress={() => add("instruction")}>Instruction</Action>
              <Action onPress={() => add("condition")}>Condition</Action>
              <Action onPress={() => add("branch")}>Rule</Action>
              <Action onPress={() => add("sub_procedure")}>Sub-procedure</Action>
            </span>
          )
        }
      >
        {definition.steps.length === 0 ? (
          <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">No steps yet. Add the first one.</p>
        ) : null}
        {definition.steps.map((step, index) => (
          <StepCard
            key={step.id}
            step={step}
            first={index === 0}
            last={index === definition.steps.length - 1}
            depth={0}
            ctx={ctx}
          />
        ))}
      </Section>
      <Handovers draft={draft} teams={teams} />
      <Scope draft={draft} workspace={workspace} />
    </div>
  );
}

function Settings({ draft, onClose, className }: { draft: Draft; onClose?: () => void; className?: string }) {
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
                  ? `Followed in ${countOf(server.uses, "conversation", "conversations")} in the last 30 days.`
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

/*
 * TestsPill says how the saved tests did on their last run.
 */
function TestsPill({ workspace, procedure }: { workspace: string; procedure: string }) {
  const tests = useQuery({ ...testCasesQuery(workspace, procedure), retry: false }).data ?? [];
  const ran = tests.filter((t) => t.last_result);
  if (ran.length === 0) {
    return null;
  }
  const passing = ran.filter((t) => t.last_result?.passed).length;

  return (
    <Badge color={passing === tests.length ? "neutral" : "primary"}>
      {passing}/{tests.length} tests passing
    </Badge>
  );
}

function Actions({ draft, workspace }: { draft: Draft; workspace: string }) {
  const { server, busy } = draft;
  const changed = server.status === "draft" || server.has_unpublished_changes;

  return (
    <>
      <TestsPill workspace={workspace} procedure={server.id} />
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

const tabs: { id: Tabbed; label: string }[] = [
  { id: "steps", label: "Steps" },
  { id: "test", label: "Test" },
  { id: "history", label: "History" },
  { id: "analytics", label: "Analytics" },
];

export function ProcedureEditor({ workspace, procedure }: { workspace: string; procedure: Procedure }) {
  const navigate = useNavigate();
  const readOnly = !usePermission(workspace, "ai_agent:manage");
  const draft = useProcedureDraft(workspace, procedure, () => void navigate({ to: "/d/procedures" }));
  const sources = useChipSources(workspace, procedure.id);
  const actions: Choices = sources.actions
    .filter((a) => a.status === "published")
    .map((a) => [a.id, a.title || a.name] as const);
  const beside = useWide(1280);
  const [drawer, setDrawer] = useState(false);
  const [tab, setTab] = useState<Tabbed>("steps");
  const ctx: Context = {
    draft,
    sources,
    actions,
    issues: draft.issues,
    numbers: stepNumbers(draft.definition.steps),
    readOnly,
  };

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
            {readOnly ? null : <Actions draft={draft} workspace={workspace} />}
          </div>
        </header>
        <div className="shrink-0 px-4">
          <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key) as Tabbed)}>
            <TabList aria-label="Procedure" items={tabs}>
              {(item) => <Tab id={item.id}>{item.label}</Tab>}
            </TabList>
          </Tabs>
        </div>
        <ScrollArea ground>
          {tab === "steps" ? (
            <fieldset disabled={readOnly} className="min-w-0">
              <Document key={draft.generation} draft={draft} ctx={ctx} workspace={workspace} />
            </fieldset>
          ) : null}
          {tab === "test" ? (
            <ProcedureTests workspace={workspace} draft={draft} sources={sources} readOnly={readOnly} />
          ) : null}
          {tab === "history" ? <ProcedureHistory workspace={workspace} draft={draft} readOnly={readOnly} /> : null}
          {tab === "analytics" ? <ProcedureAnalytics workspace={workspace} draft={draft} /> : null}
        </ScrollArea>
      </Panel>
      {beside ? (
        <Settings draft={draft} className="h-full w-[360px] shrink-0" />
      ) : (
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Procedure settings"
          className="p-0"
        >
          <Settings draft={draft} onClose={() => setDrawer(false)} className="h-full" />
        </Drawer>
      )}
    </div>
  );
}
