/*
 * The procedure editor. On the left, the procedure as dimmi reads it:
 * when to use it, where, the steps in order and when to hand over. A
 * step's words take @ chips for actions, customer details, articles,
 * inbox actions and sub-procedures; a condition is either a rule dimmi
 * decides exactly, with then and else steps under it, or plain words the
 * AI weighs. Beside the steps, the simulator and saved tests, the
 * published versions and how the procedure did. On the right, where it
 * applies, who takes a handover, what still stops it from being
 * published, and deleting it. Every change is saved as it is
 * made; publishing makes the saved copy what dimmi follows. Teammates
 * who may only read procedures see everything and change nothing. On
 * screens too narrow for both, the side opens in a drawer from the
 * header.
 */

import {
  RiAddLine,
  RiArrowLeftSLine,
  RiArrowRightLine,
  RiCheckLine,
  RiCloseLine,
  RiCodeSSlashLine,
  RiDeleteBinLine,
  RiDraggable,
  RiFacebookCircleLine,
  RiFlashlightLine,
  RiGitBranchLine,
  RiInstagramLine,
  RiListOrdered2,
  RiMailLine,
  RiMessage3Line,
  RiMore2Line,
  RiPauseCircleLine,
  RiSlackLine,
  RiSparkling2Line,
  RiSplitCellsHorizontal,
  RiTestTubeLine,
  RiWhatsappLine,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import {
  Dropdown,
  DropdownDivider,
  DropdownGroup,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { AlertDialog } from "@/components/base/modal/modal";
import { Switch } from "@/components/base/switch/switch";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { Textarea } from "@/components/base/textarea/textarea";
import { useSidePanes } from "@/features/inbox/use-side-panes";
import { Action, Field } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import { Row, Section } from "@/features/settings/layout";
import { Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useEasing } from "@/features/shell/use-easing";
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
import { cx } from "@/utils/cx";

import type { Choices } from "./choices";
import { KindMark, StatusChip } from "./parts";
import { MultiPick, Pick } from "./pick";
import { ProcedureAnalytics } from "./procedure-analytics";
import { ProcedureHistory } from "./procedure-history";
import { ProcedureOperatorPanel } from "./procedure-operator-panel";
import { ProcedurePreview } from "./procedure-simulator/preview";
import { ProcedureSimulations } from "./procedure-simulator/simulations";
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
  withStepReordered,
} from "./procedures";
import { useProcedureDraft } from "./use-procedure-draft";
import { newId } from "./workflows";

const visible = { overflow: "visible" } as const;

/*
 * Pane ids under which each browser remembers its dragged layout.
 */
const pane = { group: "dimmi:procedure", editor: "procedure", details: "details", operator: "operator" } as const;
const pressed = "border-border-button-active bg-background-primary-active";

const stepMark = {
  instruction: { icon: RiArrowRightLine, tint: "neutral", name: "Instruction" },
  condition: { icon: RiGitBranchLine, tint: "purple", name: "Condition" },
  action: { icon: RiFlashlightLine, tint: "lime", name: "Action" },
  branch: { icon: RiSplitCellsHorizontal, tint: "purple", name: "Rule" },
  sub_procedure: { icon: RiListOrdered2, tint: "blue", name: "Sub-procedure" },
} as const;

const menuKinds: ProcedureStepKind[] = ["instruction", "condition", "branch", "sub_procedure"];

/*
 * maxDepth is how deep then and else steps nest, for readability.
 */
const maxDepth = 2;

type Draft = ReturnType<typeof useProcedureDraft>;

type Tabbed = "steps" | "history" | "analytics";
type SideTab = "preview" | "simulations";

interface StepInteractions {
  dragging: { id: string; scope: string } | null;
  over: string | null;
  start: (id: string, scope: string) => void;
  enter: (id: string, scope: string) => void;
  drop: (targetId: string, scope: string) => void;
  end: () => void;
  requestDelete: (step: ProcedureStep, number: string) => void;
}

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
  steps: StepInteractions;
}

/*
 * isRule reports whether a condition is decided in code.
 */
function isRule(step: ProcedureStep): boolean {
  return step.kind === "branch" || (step.kind === "condition" && step.condition !== undefined);
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
  const scope = `${step.id}:${side}`;
  const positive = side === "then_steps";
  const title = positive ? (isRule(step) ? "Then" : "If so") : isRule(step) ? "Else" : "If not";

  return (
    <div
      className={cx(
        "flex flex-col gap-1 border-s-2 ps-3",
        positive ? "border-status-lime-text/60" : "border-border-error-default/60",
      )}
    >
      <div className="flex items-center justify-between gap-2 pt-0.5">
        <span className={cx("text-body-2-medium", positive ? "text-status-lime-text" : "text-text-error-primary")}>
          {title}
        </span>
      </div>
      {steps.length === 0 ? (
        <p className="text-body-2-regular text-text-tertiary">No steps: continue to next.</p>
      ) : (
        <ol className="flex flex-col">
          {steps.map((child) => (
            <StepCard key={child.id} step={child} scope={scope} depth={depth} ctx={ctx} />
          ))}
        </ol>
      )}
      {ctx.readOnly ? null : (
        <div className="pt-1">
          <Button
            variant="tertiary"
            size="small"
            leadingIcon={RiAddLine}
            onClick={() => ctx.draft.change((d) => withNewStep(d, "instruction", where))}
          >
            Step
          </Button>
        </div>
      )}
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
      {step.kind === "sub_procedure" ? (
        <span className="flex min-h-9 items-center gap-2 text-body-regular text-text-secondary">
          Follow
          <Pick
            key={step.sub_procedure_id ?? "none"}
            label={`Step ${number} sub-procedure`}
            choices={procedures}
            placeholder={procedures.length === 0 ? "No live procedures" : "Pick a procedure"}
            value={step.sub_procedure_id ?? ""}
            className="w-fit max-w-80"
            onChange={(id) => edit({ sub_procedure_id: id })}
          />
        </span>
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
 * LegacyActionPicker keeps older standalone action steps editable.
 */
function LegacyActionPicker({ step, number, ctx }: { step: ProcedureStep; number: string; ctx: Context }) {
  if (step.kind !== "action") {
    return null;
  }
  const edit = (change: Partial<ProcedureStep>) => ctx.draft.change((d) => withStepChange(d, step.id, change));

  return (
    <Pick
      key={step.action_id ?? "none"}
      label={`Step ${number} action`}
      choices={ctx.actions}
      placeholder={ctx.actions.length === 0 ? "No published actions" : "Pick an action"}
      value={step.action_id ?? ""}
      onChange={(action) => edit({ action_id: action })}
    />
  );
}

/*
 * KindPicker is the step's type mark. Pressing it opens the types the
 * step can switch to, so changing a step's type happens where its type
 * is shown.
 */
function KindPicker({ step, number, ctx }: { step: ProcedureStep; number: string; ctx: Context }) {
  const [open, setOpen] = useState(false);
  const activeKind: ProcedureStepKind = isRule(step) ? "branch" : step.kind;
  const look = stepMark[activeKind] ?? stepMark.instruction;
  if (ctx.readOnly) {
    return <KindMark icon={look.icon} tint={look.tint} size="sm" />;
  }
  const kinds = step.kind === "action" ? ([...menuKinds, "action"] as ProcedureStepKind[]) : menuKinds;

  const switchKind = (kind: ProcedureStepKind) => {
    setOpen(false);
    if (kind === activeKind) {
      return;
    }
    ctx.draft.change((d) =>
      withStepChange(d, step.id, () => {
        if (kind === "branch") {
          return {
            kind: "branch",
            condition: step.condition ?? { mode: "all", rules: [{ field: "", operator: "eq", value: "" }] },
            then_steps: step.then_steps ?? [],
            else_steps: step.else_steps ?? [],
          };
        }
        if (kind === "condition") {
          return { kind: "condition", condition: undefined };
        }
        return { kind };
      }),
    );
  };

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label={`Step ${number} type: ${look.name}. Change type`}
        className="rounded-md outline-none transition-opacity duration-150 hover:opacity-80 focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <KindMark icon={look.icon} tint={look.tint} size="sm" />
      </DropdownTrigger>
      <DropdownPopover aria-label={`Step ${number} type`} placement="bottom start" className="w-52">
        <DropdownGroup label="Step type">
          {kinds.map((kind) => {
            const info = stepMark[kind];
            const Icon = info.icon;
            const selected = activeKind === kind;
            return (
              <DropdownItem key={kind} selected={selected} onSelect={() => switchKind(kind)}>
                <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                <span className="min-w-0 flex-1 text-start">{info.name}</span>
                {selected ? <RiCheckLine aria-hidden className="size-4 shrink-0 text-foreground-icon-primary" /> : null}
              </DropdownItem>
            );
          })}
        </DropdownGroup>
      </DropdownPopover>
    </Dropdown>
  );
}

/*
 * StepMenu holds what is done to a step as a whole: deleting it.
 */
function StepMenu({ step, number, ctx }: { step: ProcedureStep; number: string; ctx: Context }) {
  const [open, setOpen] = useState(false);
  if (ctx.readOnly) {
    return null;
  }

  return (
    <span className="shrink-0 pt-0.5">
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label={`Step ${number} options`}
          className="flex size-8 items-center justify-center rounded-lg text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover"
        >
          <RiMore2Line aria-hidden className="size-4" />
        </DropdownTrigger>
        <DropdownPopover aria-label={`Step ${number} options`} placement="bottom end" className="w-44">
          <DropdownItem
            onSelect={() => {
              setOpen(false);
              ctx.steps.requestDelete(step, number);
            }}
            className="text-text-error-primary"
          >
            <RiDeleteBinLine aria-hidden className="size-4 shrink-0" />
            <span>Delete step</span>
          </DropdownItem>
        </DropdownPopover>
      </Dropdown>
    </span>
  );
}

function StepCard({ step, scope, depth, ctx }: { step: ProcedureStep; scope: string; depth: number; ctx: Context }) {
  const number = ctx.numbers.get(step.id) ?? "";
  const issues = ctx.issues.filter((issue) => issue.step_id === step.id);
  const nests = (step.kind === "condition" || step.kind === "branch") && depth < maxDepth;
  const isDragging = ctx.steps.dragging?.id === step.id;
  const isDropTarget =
    ctx.steps.over === step.id &&
    ctx.steps.dragging !== null &&
    ctx.steps.dragging.id !== step.id &&
    ctx.steps.dragging.scope === scope;

  return (
    <li
      onDragOver={(event) => {
        if (ctx.steps.dragging?.scope !== scope) {
          return;
        }
        event.preventDefault();
        event.stopPropagation();
        ctx.steps.enter(step.id, scope);
      }}
      onDrop={(event) => {
        if (ctx.steps.dragging?.scope !== scope) {
          return;
        }
        event.preventDefault();
        event.stopPropagation();
        ctx.steps.drop(step.id, scope);
      }}
      className={cx(
        "flex items-start gap-2.5 border-b border-separator-border py-2.5 pe-2 transition-opacity last:border-b-0",
        isDragging && "opacity-50",
        isDropTarget && "rounded-xl ring-2 ring-accent-500",
      )}
    >
      {ctx.readOnly ? null : (
        <button
          type="button"
          draggable
          aria-label={`Drag step ${number}`}
          onDragStart={(event) => {
            event.stopPropagation();
            event.dataTransfer.effectAllowed = "move";
            event.dataTransfer.setData("text/plain", step.id);
            ctx.steps.start(step.id, scope);
          }}
          onDragEnd={(event) => {
            event.stopPropagation();
            ctx.steps.end();
          }}
          onKeyDown={(event) => {
            if (event.key === "ArrowUp" || event.key === "ArrowDown") {
              event.preventDefault();
              ctx.draft.change((d) => withStepMoved(d, step.id, event.key === "ArrowUp" ? -1 : 1));
            }
          }}
          className="mt-1 flex size-7 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring active:cursor-grabbing"
        >
          <RiDraggable aria-hidden className="size-4" />
        </button>
      )}
      <span className="flex min-w-5 shrink-0 justify-end pt-2 text-body-2-medium text-text-tertiary tabular-nums">
        {number}
      </span>
      <span className="pt-1">
        <KindPicker step={step} number={number} ctx={ctx} />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <StepWords step={step} number={number} ctx={ctx} />
        <LegacyActionPicker step={step} number={number} ctx={ctx} />
        {nests ? (
          <div className="flex flex-col gap-2.5 pt-1">
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
      <StepMenu step={step} number={number} ctx={ctx} />
    </li>
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
      description="dimmi also hands over when the conversation goes in circles, or a required action keeps being skipped."
      action={<Action onPress={add}>Add</Action>}
    >
      {lines.length === 0 ? (
        <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">
          No extra handover triggers yet. Add one if specific cases should always go to a teammate.
        </p>
      ) : null}
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

const channelChoices: Choices = [
  ["widget", "Messenger"],
  ["email", "Email"],
  ["slack", "Slack"],
  ["whatsapp", "WhatsApp"],
  ["instagram", "Instagram"],
  ["facebook", "Facebook"],
  ["api", "API"],
];

const channelIcons = {
  widget: RiMessage3Line,
  email: RiMailLine,
  slack: RiSlackLine,
  whatsapp: RiWhatsappLine,
  instagram: RiInstagramLine,
  facebook: RiFacebookCircleLine,
  api: RiCodeSSlashLine,
};

function Document({ workspace, draft, ctx }: { workspace: string; draft: Draft; ctx: Context }) {
  const { definition, issues } = draft;
  const general = issues.filter((issue) => !issue.step_id && issue.field === "description");
  const rest = issues.filter((issue) => !issue.step_id && issue.field !== "description");
  const teams: Choices = ctx.sources.teams.map((t) => [t.id, t.name] as const);
  const add = (kind: ProcedureStepKind) => draft.change((d) => withNewStep(d, kind));

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      {rest.length > 0 ? (
        <Section title="To fix before publishing">
          <ul className="flex flex-col gap-1.5 py-3 pe-3">
            {rest.map((issue) => (
              <li key={issue.message} className="text-body-2-regular text-text-error-primary">
                {issue.message}
              </li>
            ))}
          </ul>
        </Section>
      ) : null}
      <Section title="Name">
        <div className="py-2.5 pe-2.5">
          <Field label="Procedure name" value={draft.name} width="w-full" onChange={draft.rename} />
        </div>
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
        description="In order. Drag to reorder. Type @ to call an action, read a detail, cite an article or follow a sub-procedure."
      >
        {definition.steps.length === 0 ? (
          <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">No steps yet. Add the first one below.</p>
        ) : (
          <ol className="flex flex-col">
            {definition.steps.map((step) => (
              <StepCard key={step.id} step={step} scope="root" depth={0} ctx={ctx} />
            ))}
          </ol>
        )}
        {ctx.readOnly ? null : (
          <div className="flex flex-wrap items-center gap-1 border-t border-separator-border px-1 py-2">
            <Button variant="tertiary" size="small" leadingIcon={RiAddLine} onClick={() => add("instruction")}>
              Instruction
            </Button>
            <Button variant="tertiary" size="small" leadingIcon={RiAddLine} onClick={() => add("condition")}>
              Condition
            </Button>
            <Button variant="tertiary" size="small" leadingIcon={RiAddLine} onClick={() => add("branch")}>
              Rule
            </Button>
            <Button variant="tertiary" size="small" leadingIcon={RiAddLine} onClick={() => add("sub_procedure")}>
              Sub-procedure
            </Button>
          </div>
        )}
      </Section>
      <Handovers draft={draft} />
      <Routing draft={draft} teams={teams} />
      <Applies workspace={workspace} draft={draft} />
    </div>
  );
}

const sideTabs: { id: SideTab; label: string }[] = [
  { id: "preview", label: "Preview" },
  { id: "simulations", label: "Simulations" },
];

/* Room to spell out several picked channels or inboxes; the control still hugs what it says. */
const widePick = "max-w-96 shrink-0";

/*
 * Applies says where dimmi may follow the procedure: on which channels,
 * in which inboxes, for which audiences, and whether only for signed-in
 * customers or only from inside another procedure.
 */
function Applies({ workspace, draft }: { workspace: string; draft: Draft }) {
  const scope = draft.definition.scope ?? {};
  const inboxes = useQuery({ ...inboxesQuery(workspace), retry: false }).data ?? [];
  const audiences = useQuery({ ...audiencesQuery(workspace), retry: false }).data ?? [];
  const setScope = (change: Partial<ProcedureScope>) =>
    draft.change((d) => ({ ...d, scope: { ...d.scope, ...change } }));

  return (
    <Section title="Where it applies" description="dimmi follows it only in conversations that match all of these.">
      <Row label="Channels">
        <MultiPick
          label="Channels"
          choices={channelChoices}
          value={scope.channels ?? []}
          allLabel="All channels"
          shown={5}
          className={widePick}
          icons={channelIcons}
          onChange={(channels) => setScope({ channels })}
        />
      </Row>
      <Row label="Inboxes">
        <MultiPick
          label="Inboxes"
          choices={inboxes.map((i) => [i.id, i.name] as const)}
          value={scope.inbox_ids ?? []}
          allLabel="All inboxes"
          shown={5}
          className={widePick}
          empty="No inboxes"
          onChange={(inbox_ids) => setScope({ inbox_ids })}
        />
      </Row>
      <Row label="Audiences">
        <MultiPick
          label="Saved audiences"
          choices={audiences.map((a) => [a.id, a.name] as const)}
          value={scope.audience_ids ?? []}
          allLabel="Everyone"
          shown={5}
          className={widePick}
          empty="No saved audiences"
          onChange={(audience_ids) => setScope({ audience_ids })}
        />
      </Row>
      <Row label="Signed-in customers only" description="Skipped for visitors dimmi cannot tell apart.">
        <Switch
          size="sm"
          aria-label="Only for signed-in customers"
          isSelected={scope.require_verified_identity ?? false}
          onChange={(on) => setScope({ require_verified_identity: on })}
        />
      </Row>
      <Row label="Sub-procedure only" description="Followed only when another procedure calls it.">
        <Switch
          size="sm"
          aria-label="Only as a sub-procedure"
          isSelected={scope.reusable_only ?? false}
          onChange={(on) => setScope({ reusable_only: on })}
        />
      </Row>
    </Section>
  );
}

/*
 * Routing says who takes the conversation when dimmi hands it over, and
 * how urgent it is marked.
 */
function Routing({ draft, teams }: { draft: Draft; teams: Choices }) {
  const rule = draft.definition.handover_rule ?? {};
  const setRoute = (change: Partial<NonNullable<ProcedureDefinition["handover_rule"]>>) =>
    draft.change((d) => ({ ...d, handover_rule: { ...d.handover_rule, ...change } }));

  return (
    <Section title="Handover routing" description="Where a handover from this procedure goes.">
      <Row label="Team">
        <Pick
          label="Handover team"
          choices={teams}
          placeholder={teams.length === 0 ? "No teams" : "Any team"}
          value={rule.team_id ?? ""}
          onChange={(team) => setRoute({ team_id: team })}
        />
      </Row>
      <Row label="Priority">
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
          onChange={(priority) => setRoute({ priority: priority as "low" })}
        />
      </Row>
    </Section>
  );
}

/*
 * ProcedureMenu holds what is done to the procedure as a whole: turning
 * it off, so dimmi stops following it while its steps are kept, and
 * deleting it, which asks first.
 */
function ProcedureMenu({ draft }: { draft: Draft }) {
  const [open, setOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [stopping, setStopping] = useState(false);

  return (
    <>
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label="Procedure actions"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover"
        >
          <RiMore2Line aria-hidden className="size-4" />
        </DropdownTrigger>
        <DropdownPopover aria-label="Procedure actions" placement="bottom end" className="w-52">
          {draft.server.status === "live" ? (
            <>
              <DropdownItem
                className="gap-2"
                onSelect={() => {
                  setOpen(false);
                  setStopping(true);
                }}
              >
                <RiPauseCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                Turn off
              </DropdownItem>
              <DropdownDivider />
            </>
          ) : null}
          <DropdownItem
            className="gap-2 text-text-error-primary"
            onSelect={() => {
              setOpen(false);
              setDeleting(true);
            }}
          >
            <RiDeleteBinLine aria-hidden className="size-4 shrink-0" />
            Delete procedure
          </DropdownItem>
        </DropdownPopover>
      </Dropdown>
      <AlertDialog
        isOpen={stopping}
        onOpenChange={setStopping}
        variant="default"
        icon={false}
        title={`Turn off ${draft.name || "this procedure"}?`}
        description="dimmi stops following it for customers. Your steps, simulations and history stay, and you can publish it again at any time."
        confirmLabel="Turn off"
        onConfirm={() => {
          setStopping(false);
          void draft.unpublish();
        }}
      />
      <AlertDialog
        isOpen={deleting}
        onOpenChange={setDeleting}
        variant="danger"
        title={`Delete ${draft.name || "this procedure"}?`}
        description={
          draft.server.status === "live"
            ? "It is live. dimmi stops following it at once, and its simulations and history go with it. This cannot be undone."
            : "Its simulations and history go with it. This cannot be undone."
        }
        confirmLabel="Delete procedure"
        onConfirm={() => {
          setDeleting(false);
          void draft.remove();
        }}
      />
    </>
  );
}

/*
 * ProcedureSidePanel is where the procedure is tried before anyone meets
 * it: Preview, a chat by hand, and Simulations, the saved conversations
 * that are run before a change is published. Both stay mounted, so a
 * chat in progress survives a look at the simulations. A teammate who
 * may only read sees the simulations and how they did.
 */
function ProcedureSidePanel({
  workspace,
  draft,
  sources,
  readOnly,
  sideTab,
  onSideTab,
  onClose,
  className,
}: {
  workspace: string;
  draft: Draft;
  sources: ChipSources;
  readOnly: boolean;
  sideTab: SideTab;
  onSideTab: (tab: SideTab) => void;
  onClose?: () => void;
  className?: string;
}) {
  const tests = useQuery({ ...testCasesQuery(workspace, draft.server.id), retry: false }).data ?? [];
  const shown: SideTab = readOnly ? "simulations" : sideTab;
  const offered = readOnly ? sideTabs.filter((tab) => tab.id === "simulations") : sideTabs;
  const pane = (tab: SideTab) => cx("flex min-h-0 flex-1 flex-col", shown !== tab && "hidden");

  return (
    <Panel label="Testing" className={className}>
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5 pt-[7px]">
        <Tabs selectedKey={shown} onSelectionChange={(key) => onSideTab(String(key) as SideTab)}>
          <TabList aria-label="Testing" items={offered}>
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
        {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close testing" onClick={onClose} /> : null}
      </header>
      {readOnly ? null : (
        <div className={pane("preview")}>
          <ProcedurePreview
            workspace={workspace}
            draft={draft}
            sources={sources}
            onSaved={() => onSideTab("simulations")}
          />
        </div>
      )}
      <div className={pane("simulations")}>
        <ProcedureSimulations workspace={workspace} draft={draft} sources={sources} readOnly={readOnly} list={tests} />
      </div>
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
 * TestsPill says how many simulations passed on their last run, in the
 * colour of how it went: green when all pass, red when one fails, plain
 * when some have not run. Pressing it opens the simulations.
 */
function TestsPill({ workspace, procedure, onOpen }: { workspace: string; procedure: string; onOpen: () => void }) {
  const tests = useQuery({ ...testCasesQuery(workspace, procedure), retry: false }).data ?? [];
  const ran = tests.filter((t) => t.last_result);
  if (ran.length === 0) {
    return null;
  }
  const passing = ran.filter((t) => t.last_result?.passed).length;
  const failing = ran.length - passing;
  const color = failing > 0 ? "rose" : passing === tests.length ? "lime" : "neutral";

  return (
    <button
      type="button"
      aria-label={`${passing} of ${tests.length} simulations passing. Open simulations.`}
      onClick={onOpen}
      className="flex cursor-pointer rounded-md outline-none transition-opacity duration-150 hover:opacity-80 focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <Chip variant="caption" color={color} className="py-0.5 tabular-nums">
        {passing}/{tests.length}
      </Chip>
    </button>
  );
}

/*
 * publishWarning says what a teammate should know before publishing:
 * that customers get the change at once, and how the simulations did.
 */
function publishWarning(live: boolean, failing: number, unrun: number): string {
  const reach = live
    ? "dimmi follows the new version for customers from their next message."
    : "dimmi starts following it for customers at once.";
  if (failing > 0) {
    return `${failing} ${failing === 1 ? "simulation" : "simulations"} failed on the last run. ${reach}`;
  }
  if (unrun > 0) {
    return `${unrun} ${unrun === 1 ? "simulation has" : "simulations have"} not run against this copy. ${reach}`;
  }

  return reach;
}

function Actions({ draft, workspace, onTests }: { draft: Draft; workspace: string; onTests: () => void }) {
  const { server, busy } = draft;
  const [publishing, setPublishing] = useState(false);
  const tests = useQuery({ ...testCasesQuery(workspace, server.id), retry: false }).data ?? [];
  const failing = tests.filter((t) => t.last_result && !t.last_result.passed).length;
  const unrun = tests.filter((t) => !t.last_result).length;
  const changed = server.status === "draft" || server.has_unpublished_changes;
  const live = server.status === "live";

  return (
    <>
      <TestsPill workspace={workspace} procedure={server.id} onOpen={onTests} />
      {changed ? (
        <Button
          size="small"
          loading={busy}
          disabled={busy || draft.state === "conflict"}
          onClick={() => setPublishing(true)}
        >
          {live ? "Publish changes" : "Publish"}
        </Button>
      ) : null}
      <ProcedureMenu draft={draft} />
      <AlertDialog
        isOpen={publishing}
        onOpenChange={setPublishing}
        variant="default"
        icon={false}
        title={
          live ? `Publish changes to ${draft.name || "this procedure"}?` : `Publish ${draft.name || "this procedure"}?`
        }
        description={publishWarning(live, failing, unrun)}
        confirmLabel={live ? "Publish changes" : "Publish"}
        onConfirm={() => {
          setPublishing(false);
          void draft.publish();
        }}
      />
    </>
  );
}

const tabs: { id: Tabbed; label: string }[] = [
  { id: "steps", label: "Steps" },
  { id: "history", label: "History" },
  { id: "analytics", label: "Analytics" },
];

function useStepInteractions(draft: Draft) {
  const [dragging, setDragging] = useState<{ id: string; scope: string } | null>(null);
  const [over, setOver] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<{ step: ProcedureStep; number: string } | null>(null);

  const interactions: StepInteractions = {
    dragging,
    over,
    start: (id, scope) => setDragging({ id, scope }),
    enter: (id, scope) => {
      if (dragging?.scope === scope) {
        setOver(id);
      }
    },
    drop: (targetId, scope) => {
      if (dragging && dragging.scope === scope && dragging.id !== targetId) {
        draft.change((d) => withStepReordered(d, dragging.id, targetId));
      }
      setDragging(null);
      setOver(null);
    },
    end: () => {
      setDragging(null);
      setOver(null);
    },
    requestDelete: (step, number) => setDeleting({ step, number }),
  };

  return { interactions, deleting, closeDelete: () => setDeleting(null) };
}

function DeleteStepDialog({
  deleting,
  onClose,
  onConfirm,
}: {
  deleting: { step: ProcedureStep; number: string } | null;
  onClose: () => void;
  onConfirm: (id: string) => void;
}) {
  const nestedCount = deleting ? (deleting.step.then_steps?.length ?? 0) + (deleting.step.else_steps?.length ?? 0) : 0;

  return (
    <AlertDialog
      isOpen={deleting !== null}
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      variant="danger"
      title={deleting ? `Delete step ${deleting.number}?` : "Delete step?"}
      description={
        nestedCount > 0
          ? `This removes the step and the ${countOf(nestedCount, "nested step", "nested steps")} inside it.`
          : "This step will be removed from the procedure."
      }
      confirmLabel="Delete step"
      onConfirm={() => {
        if (deleting) {
          onConfirm(deleting.step.id);
          onClose();
        }
      }}
    />
  );
}

export function ProcedureEditor({ workspace, procedure }: { workspace: string; procedure: Procedure }) {
  const navigate = useNavigate();
  const readOnly = !usePermission(workspace, "ai_agent:manage");
  const draft = useProcedureDraft(workspace, procedure, () => void navigate({ to: "/d/procedures" }));
  const sources = useChipSources(workspace, procedure.id);
  const actions: Choices = sources.actions
    .filter((a) => a.status === "published")
    .map((a) => [a.id, a.title || a.name] as const);
  const sideBySide = useWide(1024);
  const sides = useSidePanes(
    pane.editor,
    { [pane.details]: 380, [pane.operator]: 380 },
    () => document.getElementById(pane.group)?.clientWidth ?? 0,
  );
  const [detailsOpen, setDetailsOpen] = useState(true);
  const [operatorOpen, setOperatorOpen] = useState(false);
  const [detailsDrawer, setDetailsDrawer] = useState(false);
  const [operatorDrawer, setOperatorDrawer] = useState(false);
  const [easing, ease] = useEasing();
  const [tab, setTab] = useState<Tabbed>("steps");
  const [sideTab, setSideTab] = useState<SideTab>("preview");
  const stepUI = useStepInteractions(draft);

  const toggle = (id: string) => ease(() => sides.toggle(id));
  const toggleDetails = () => (sideBySide ? toggle(pane.details) : setDetailsDrawer((open) => !open));
  const toggleOperator = () => (sideBySide ? toggle(pane.operator) : setOperatorDrawer((open) => !open));
  const detailsShown = sideBySide ? detailsOpen : detailsDrawer;
  const openSimulations = () => {
    setSideTab("simulations");
    if (!detailsShown) {
      toggleDetails();
    }
  };
  const operatorShown = sideBySide ? operatorOpen : operatorDrawer;

  const ctx: Context = {
    draft,
    sources,
    actions,
    issues: draft.issues,
    numbers: stepNumbers(draft.definition.steps),
    readOnly,
    steps: stepUI.interactions,
  };

  const mainPanel = (
    <Panel label="Procedure" ground className="h-full w-full">
      <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
        <div className="flex min-w-0 flex-1 items-center gap-2">
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
        <div className="flex shrink-0 items-center gap-2">
          {readOnly ? null : <Actions draft={draft} workspace={workspace} onTests={openSimulations} />}
          <IconButton
            size="small"
            icon={RiTestTubeLine}
            className={cx(detailsShown && pressed)}
            aria-label={detailsShown ? "Hide testing" : "Show testing"}
            aria-pressed={detailsShown}
            onClick={toggleDetails}
          />
          <IconButton
            size="small"
            icon={RiSparkling2Line}
            className={cx(operatorShown && pressed)}
            aria-label="Operator"
            aria-pressed={operatorShown}
            onClick={toggleOperator}
          />
        </div>
      </header>
      <div className="shrink-0 px-4">
        <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key) as Tabbed)}>
          <TabList aria-label="Procedure" items={tabs}>
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
      </div>
      <ScrollArea>
        {tab === "steps" ? (
          <fieldset disabled={readOnly} className="min-w-0">
            <Document key={draft.generation} workspace={workspace} draft={draft} ctx={ctx} />
          </fieldset>
        ) : null}
        {tab === "history" ? (
          <ProcedureHistory workspace={workspace} draft={draft} sources={sources} readOnly={readOnly} />
        ) : null}
        {tab === "analytics" ? <ProcedureAnalytics workspace={workspace} draft={draft} sources={sources} /> : null}
      </ScrollArea>
    </Panel>
  );

  const deleteDialog = (
    <DeleteStepDialog
      deleting={stepUI.deleting}
      onClose={stepUI.closeDelete}
      onConfirm={(id) => draft.change((d) => withoutStepById(d, id))}
    />
  );

  if (!sideBySide) {
    return (
      <div className="flex h-full min-h-0 min-w-0 flex-1">
        {mainPanel}
        <Drawer
          isOpen={detailsDrawer}
          onOpenChange={setDetailsDrawer}
          placement="end"
          size="md"
          aria-label="Testing"
          className="p-0"
        >
          <ProcedureSidePanel
            workspace={workspace}
            draft={draft}
            sources={sources}
            readOnly={readOnly}
            sideTab={sideTab}
            onSideTab={setSideTab}
            onClose={() => setDetailsDrawer(false)}
            className="h-full w-full"
          />
        </Drawer>
        <Drawer
          isOpen={operatorDrawer}
          onOpenChange={setOperatorDrawer}
          placement="end"
          size="md"
          aria-label="Operator"
          className="p-0"
        >
          <ProcedureOperatorPanel
            workspace={workspace}
            procedureId={procedure.id}
            procedureName={draft.name || procedure.name}
            onClose={() => setOperatorDrawer(false)}
            className="h-full w-full"
          />
        </Drawer>
        {deleteDialog}
      </div>
    );
  }

  return (
    <>
      <PaneGroup
        id={pane.group}
        panelIds={[pane.editor, pane.details, pane.operator]}
        easing={easing}
        groupRef={sides.group}
      >
        <Pane id={pane.editor} minSize={380} style={visible} className="flex h-full">
          {mainPanel}
        </Pane>
        <PaneHandle hidden={!detailsOpen} />
        <Pane
          id={pane.details}
          defaultSize={380}
          minSize={300}
          maxSize={640}
          collapsible
          groupResizeBehavior="preserve-pixel-size"
          onResize={(size) => {
            sides.remember(pane.details)(size);
            setDetailsOpen(size.inPixels > 0);
          }}
          style={visible}
          className={cx("flex h-full", !detailsOpen && "invisible")}
        >
          <ProcedureSidePanel
            workspace={workspace}
            draft={draft}
            sources={sources}
            readOnly={readOnly}
            sideTab={sideTab}
            onSideTab={setSideTab}
            onClose={() => toggle(pane.details)}
            className="h-full w-full"
          />
        </Pane>
        <PaneHandle hidden={!operatorOpen} />
        <Pane
          id={pane.operator}
          defaultSize={0}
          minSize={300}
          maxSize={600}
          collapsible
          groupResizeBehavior="preserve-pixel-size"
          onResize={(size) => {
            sides.remember(pane.operator)(size);
            setOperatorOpen(size.inPixels > 0);
          }}
          style={visible}
          className={cx("flex h-full", !operatorOpen && "invisible")}
        >
          <ProcedureOperatorPanel
            workspace={workspace}
            procedureId={procedure.id}
            procedureName={draft.name || procedure.name}
            onClose={() => toggle(pane.operator)}
            className="h-full w-full"
          />
        </Pane>
      </PaneGroup>
      {deleteDialog}
    </>
  );
}
