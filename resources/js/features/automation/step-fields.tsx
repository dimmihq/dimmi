/*
 * The settings of one step in the inspector. Every kind has its own few
 * settings, stored the way the server reads them; every change goes back
 * to the step through onEdit, and its card says the new sentence at
 * once. Teams, teammates and tags are picked from the workspace's own.
 * A kind the server does not run yet only says so.
 */
import type { ReactNode } from "react";

import { Action, Danger, Field, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import type { Condition } from "@/lib/api/workflows";
import { useAIConditionFields } from "./ai-condition-fields";
import { ArticleFields } from "./article-fields";
import { CallFields } from "./call-fields";
import { type Choices, choicesOf } from "./choices";
import { ClassifyFields } from "./classify-fields";
import {
  changeField,
  changeOp,
  conditionOf,
  fieldsFor,
  isSimple,
  needsValue,
  opsFor,
  optionsOf,
  valueList,
  valueText,
} from "./conditions";
import { ExtractFields } from "./extract-fields";
import { type EditorProps, Words } from "./field-parts";
import { Pick, Picks } from "./pick";
import { ReplyTimeFields } from "./reply-time-fields";
import { RunActionFields } from "./run-action-fields";
import { RunWorkflowFields } from "./run-workflow-fields";
import { SplitFields } from "./split-fields";
import {
  collectTargets,
  contactFields,
  durations,
  list,
  priorities,
  text,
  unassignTargets,
  useSet,
  withNewPath,
  withoutPath,
  withPathChange,
  withPathLabel,
  withSettings,
} from "./step-settings";
import { type TicketLists, TicketStepFields } from "./ticket-fields";
import { ticketStepKinds } from "./ticket-lists";
import type { TriggerKind } from "./triggers";
import { pathName } from "./workflows";

/*
 * Lists are the workspace's teams, teammates, tags and AI agents, as
 * choices, and whether its default agent is on, unknown until read. Workflows are the live
 * reusable ones another workflow may run, connections the systems a
 * call may use, and schedules the office hours. Tickets are the ticket
 * setup, trackers and SLA policies the ticket steps pick from.
 */
export interface Lists {
  teams: Choices;
  members: Choices;
  tags: Choices;
  workflows: Choices;
  connections: Choices;
  schedules: Choices;
  articles: Choices;
  agents?: Choices;
  agentOn?: boolean;
  tickets?: TicketLists;
}

type Props = EditorProps;

function MessageFields({ step, onEdit }: Props) {
  const set = useSet(onEdit);
  const name = step.kind === "note" ? "Note" : "Message";

  return (
    <Section title={name} description="Use {{contact.first_name}} and other variables to fill in details.">
      <Words label={name} value={text(step.settings, "text")} onChange={(value) => set({ text: value })} />
    </Section>
  );
}

function ButtonsFields({ step, onEdit }: Props) {
  const set = useSet(onEdit);
  const buttons = (step.paths ?? []).filter((path) => !path.outcome);

  return (
    <>
      <Section
        title="Question and buttons"
        description="Customers answer with a number, a button's name or in their own words."
      >
        <Words label="Question" value={text(step.settings, "text")} onChange={(value) => set({ text: value })} />
        {buttons.map((path, index) => (
          <Row key={path.id} label={`Button ${index + 1}`}>
            <div className="flex items-center gap-2">
              <Field
                label={`Button ${index + 1}`}
                value={path.label}
                width="w-40"
                onChange={(label) => onEdit((current) => withPathLabel(current, path.id, label))}
              />
              {buttons.length > 2 ? (
                <Danger onPress={() => onEdit((current) => withoutPath(current, path.id))}>Remove</Danger>
              ) : null}
            </div>
          </Row>
        ))}
        {buttons.length < 6 ? (
          <Row label="Another button">
            <Action onPress={() => onEdit((current) => withNewPath(current, `Option ${buttons.length + 1}`))}>
              Add button
            </Action>
          </Row>
        ) : null}
      </Section>
      <Section
        title="When the answer matches no button"
        description="Sent once, before the workflow stops waiting for an answer."
      >
        <Words
          label="Message when no button matches"
          value={text(step.settings, "retry_text")}
          placeholder="Please reply with one of the numbers above."
          onChange={(value) => set({ retry_text: value || undefined })}
        />
      </Section>
    </>
  );
}

function CollectFields({ step, onEdit }: Props) {
  const set = useSet(onEdit);
  const field = text(step.settings, "field");

  return (
    <Section title="Question">
      <Words label="Question" value={text(step.settings, "text")} onChange={(value) => set({ text: value })} />
      <Row label="Save the answer as">
        <Pick
          label="Save the answer as"
          choices={Object.entries(collectTargets).map(([key, target]) => [key, target.name] as const)}
          value={field}
          onChange={(key) => set({ field: key, type: collectTargets[key]?.type ?? "text" })}
        />
      </Row>
      <Row label="Skip when already known">
        <Toggle
          label="Skip when already known"
          on={step.settings?.skip_if_known !== false}
          onChange={(on) => set({ skip_if_known: on ? undefined : false })}
        />
      </Row>
    </Section>
  );
}

function ConditionFields({
  trigger,
  condition,
  lists,
  onChange,
}: {
  trigger: TriggerKind;
  condition: Condition | undefined;
  lists: Lists;
  onChange: (condition: Condition) => void;
}) {
  if (!isSimple(condition)) {
    return <Row label="Condition" description="Set through the API. Edit it there." />;
  }
  const field = condition?.field ?? "";
  const op = condition?.op ?? "";
  const options = optionsOf(field, lists.tickets, lists.agents);

  return (
    <>
      <Row label="When">
        <Pick
          label="Condition field"
          choices={choicesOf(fieldsFor(trigger))}
          value={field}
          onChange={(key) => onChange(changeField(condition, key))}
        />
      </Row>
      {field ? (
        <Row label="Comparison">
          <Pick
            key={field}
            label="Comparison"
            choices={choicesOf(opsFor(field))}
            value={op}
            onChange={(key) => onChange(changeOp(condition, key))}
          />
        </Row>
      ) : null}
      {field && needsValue(op) && options ? (
        <Picks
          choices={options}
          value={valueList(condition)}
          empty=""
          onChange={(values) => onChange(conditionOf(field, op, values))}
        />
      ) : null}
      {field && needsValue(op) && !options ? (
        <Row label="Value">
          <Field
            key={`${field}-${op}`}
            label="Condition value"
            placeholder="Words, separated by commas"
            value={valueText(condition)}
            onChange={(typed) => onChange(conditionOf(field, op, typed))}
          />
        </Row>
      ) : null}
    </>
  );
}

function BranchFields({ step, trigger, lists, onEdit }: Props) {
  const named = (step.paths ?? []).filter((path) => !path.outcome);

  return (
    <>
      {named.map((path, index) => (
        <Section
          key={path.id}
          title={`Path ${index + 1}`}
          description={index === 0 ? "The first path whose condition matches wins." : undefined}
          action={
            named.length > 1 ? (
              <Danger onPress={() => onEdit((current) => withoutPath(current, path.id))}>Remove</Danger>
            ) : undefined
          }
        >
          <Row label="Name">
            <Field
              label={`Path ${index + 1} name`}
              value={path.label}
              onChange={(label) => onEdit((current) => withPathLabel(current, path.id, label))}
            />
          </Row>
          <ConditionFields
            trigger={trigger}
            condition={path.condition}
            lists={lists}
            onChange={(condition) => onEdit((current) => withPathChange(current, path.id, { condition }))}
          />
        </Section>
      ))}
      <Section title="Everyone else" description="Conversations no path matches go down the last path.">
        <Row label="Another path">
          <Action onPress={() => onEdit((current) => withNewPath(current, `Path ${named.length + 1}`))}>
            Add path
          </Action>
        </Row>
      </Section>
    </>
  );
}

const waitKinds = { length: "A length of time", open: "Office hours to open" };

/*
 * LengthFields shows how long a wait or snooze lasts: a length of time,
 * which a wait may count in office hours only, or until office hours
 * open.
 */
function LengthFields({ step, onEdit, title }: Props & { title: string }) {
  const set = useSet(onEdit);
  const untilOpen = (step.settings?.until as { kind?: string } | undefined)?.kind === "office_hours_open";

  return (
    <Section title={title} description="Office hours are set in Settings, Office hours.">
      <Row label="Until">
        <Pick
          label={`${title} until`}
          choices={choicesOf(waitKinds)}
          value={untilOpen ? "open" : "length"}
          onChange={(value) =>
            set(
              value === "open"
                ? { for: undefined, count: undefined, until: { kind: "office_hours_open" } }
                : { for: "1h", until: undefined },
            )
          }
        />
      </Row>
      {untilOpen ? null : (
        <Row label="For">
          <Pick
            key={text(step.settings, "for") || "none"}
            label={`${title} for`}
            choices={choicesOf(durations)}
            value={text(step.settings, "for")}
            onChange={(value) => set({ for: value, until: undefined })}
          />
        </Row>
      )}
      {untilOpen || step.kind !== "wait" ? null : (
        <Row label="Count only office hours" description="Evenings, weekends and holidays don't count.">
          <Toggle
            label="Count only office hours"
            on={text(step.settings, "count") === "business"}
            onChange={(on) => set({ count: on ? "business" : undefined })}
          />
        </Row>
      )}
    </Section>
  );
}

function AssignFields({ step, lists, onEdit }: Props) {
  const set = useSet(onEdit);
  const mode = text(step.settings, "mode") || "team";
  const modes = {
    team: "A team",
    team_round_robin: "A team's teammates, in turn",
    team_balanced: "Whoever in a team holds the fewest",
    member: "A teammate",
  };

  return (
    <Section title="Who takes it">
      <Row label="Assign to">
        <Pick
          label="Assign to"
          choices={choicesOf(modes)}
          value={mode}
          onChange={(value) => set({ mode: value, member_id: undefined, team_id: undefined })}
        />
      </Row>
      {mode === "member" ? (
        <Row label="Teammate">
          <Pick
            key={mode}
            label="Teammate"
            choices={lists.members}
            value={text(step.settings, "member_id")}
            onChange={(id) => set({ member_id: id })}
          />
        </Row>
      ) : (
        <Row label="Team">
          <Pick
            key={mode}
            label="Team"
            choices={lists.teams}
            value={text(step.settings, "team_id")}
            onChange={(id) => set({ team_id: id })}
          />
        </Row>
      )}
      <Row label="Leave assigned conversations alone">
        <Toggle
          label="Leave assigned conversations alone"
          on={step.settings?.keep_if_assigned !== false}
          onChange={(on) => set({ keep_if_assigned: on ? undefined : false })}
        />
      </Row>
    </Section>
  );
}

function TagFields({ step, lists, onEdit }: Props) {
  const set = useSet(onEdit);

  return (
    <Section title={step.kind === "tag" ? "Tags to add" : "Tags to remove"}>
      <Picks
        choices={lists.tags}
        value={list(step.settings, "tag_ids")}
        empty="This workspace has no tags yet. Add them in settings."
        onChange={(ids) => set({ tag_ids: ids })}
      />
    </Section>
  );
}

function ContactFields({ step, onEdit }: Props) {
  const changes = Array.isArray(step.settings?.set) ? (step.settings.set as { field: string; value: string }[]) : [];
  const change = (index: number, next: { field: string; value: string }) =>
    onEdit((current) => withSettings(current, { set: changes.map((old, at) => (at === index ? next : old)) }));

  return (
    <Section title="Contact fields">
      {changes.map((item, index) => (
        <Row
          key={`${item.field}-${String(index)}`}
          label={
            <Pick
              label={`Field ${index + 1}`}
              choices={choicesOf(contactFields)}
              value={item.field}
              onChange={(field) => change(index, { ...item, field })}
            />
          }
        >
          <Field
            label={`Value ${index + 1}`}
            value={item.value}
            width="w-40"
            onChange={(value) => change(index, { ...item, value })}
          />
        </Row>
      ))}
    </Section>
  );
}

function OneChoice({ step, onEdit, title, name, choices }: Props & { title: string; name: string; choices: Choices }) {
  const set = useSet(onEdit);

  return (
    <Section title={title}>
      <Row label={title}>
        <Pick
          label={title}
          choices={choices}
          value={text(step.settings, name)}
          onChange={(value) => set({ [name]: value })}
        />
      </Row>
    </Section>
  );
}

function CloseFields({ step, onEdit }: Props) {
  const set = useSet(onEdit);

  return (
    <Section title="Before closing">
      <Row label="Only if no teammate replied">
        <Toggle
          label="Only if no teammate replied"
          on={step.settings?.only_if_no_teammate_reply === true}
          onChange={(on) => set({ only_if_no_teammate_reply: on || undefined })}
        />
      </Row>
      <Row label="Ask for a rating">
        <Toggle
          label="Ask for a rating"
          on={step.settings?.ask_rating === true}
          onChange={(on) => set({ ask_rating: on || undefined })}
        />
      </Row>
    </Section>
  );
}

const quietWaits: Record<string, string> = {
  channel: "10 minutes in chat, 1 day by email",
  "5m": "5 minutes",
  "30m": "30 minutes",
  "1h": "1 hour",
  "4h": "4 hours",
  "1d": "1 day",
};

/*
 * DimmiFields picks which AI agent takes the conversation and how long
 * it keeps it. The agent answers by its own guidance and procedures;
 * the step adds none. Inside the step its handover goes down the
 * handed-over path instead of to the agent's own handover team.
 */
function DimmiFields({ step, lists, onEdit }: Props) {
  const set = useSet(onEdit);
  const agent = text(step.settings, "agent_id");
  const placed = "placed";

  return (
    <>
      {lists.agentOn === false ? (
        <Section
          title="The agent is off"
          description="Until the agent that would answer is published, this step hands every conversation to the team."
        />
      ) : null}
      <Section title="Who answers" description="The agent answers with its own guidance and procedures.">
        <Row label="Agent">
          <Pick
            label="Agent"
            choices={[[placed, "The agent placed on the channel"], ...(lists.agents ?? [])]}
            value={agent || placed}
            onChange={(value) => set({ agent_id: value === placed ? undefined : value, allow_transfers: undefined })}
          />
        </Row>
        {agent ? (
          <Row label="May transfer" description="To the agents it lists under May transfer to.">
            <Toggle
              label="May transfer"
              on={step.settings?.allow_transfers === true}
              onChange={(on) => set({ allow_transfers: on ? true : undefined })}
            />
          </Row>
        ) : null}
      </Section>
      <Section title="While the agent handles it">
        <Row label="Customer gone quiet after">
          <Pick
            label="Customer gone quiet after"
            choices={choicesOf(quietWaits)}
            value={text(step.settings, "quiet_after") || "channel"}
            onChange={(value) => set({ quiet_after: value === "channel" ? undefined : value })}
          />
        </Row>
        <Row label="Ask the customer to confirm answers" description="Resolved only when the customer says so.">
          <Toggle
            label="Ask the customer to confirm answers"
            on={step.settings?.confirm_resolution !== false}
            onChange={(on) => set({ confirm_resolution: on ? undefined : false })}
          />
        </Row>
        <Row label="Close when resolved" description="When the resolved path has no steps.">
          <Toggle
            label="Close when resolved"
            on={step.settings?.close_on_resolved !== false}
            onChange={(on) => set({ close_on_resolved: on ? undefined : false })}
          />
        </Row>
      </Section>
      <Section
        title="Always"
        description="The agent stops the moment a teammate replies. A handover takes the handed-over path, not the agent's own handover team. Silence never counts as resolved: it takes the quiet path."
      />
    </>
  );
}

/*
 * AssignAgentFields picks the agent the conversation moves to. It
 * answers from the customer's next message on.
 */
function AssignAgentFields({ step, lists, onEdit }: Props) {
  const set = useSet(onEdit);

  return (
    <Section title="Agent" description="Answers from the customer's next message, until a teammate steps in.">
      <Row label="Agent">
        <Pick
          label="Agent"
          choices={lists.agents ?? []}
          value={text(step.settings, "agent_id")}
          onChange={(id) => set({ agent_id: id })}
        />
      </Row>
    </Section>
  );
}

/*
 * AIStateFields explains a step that turns AI off or on; it takes no
 * settings.
 */
function AIStateFields({ step }: Props) {
  return step.kind === "turn_off_ai" ? (
    <Section
      title="Turn off AI"
      description="No AI agent answers this conversation until a teammate or a Turn on AI step lets one back in. A Hand to an AI agent step still can."
    />
  ) : (
    <Section
      title="Turn on AI"
      description="The agent answers this conversation again from the customer's next message, even after a handover to the team."
    />
  );
}

/*
 * Unavailable stands in for a kind the server does not run yet, which
 * only a definition written through the API can hold.
 */
function Unavailable({ step }: Props) {
  return (
    <Section title="Not available yet">
      <Row
        label="This step cannot run yet"
        description={`Remove it to publish. Paths: ${(step.paths ?? []).map(pathName).join(", ") || "none"}.`}
      />
    </Section>
  );
}

const editors: Record<string, (props: Props) => ReactNode> = {
  message: MessageFields,
  article: ArticleFields,
  note: MessageFields,
  buttons: ButtonsFields,
  collect: CollectFields,
  dimmi: DimmiFields,
  assign_ai_agent: AssignAgentFields,
  turn_off_ai: AIStateFields,
  turn_on_ai: AIStateFields,
  branch: BranchFields,
  wait: (props) => <LengthFields {...props} title="Wait" />,
  snooze: (props) => <LengthFields {...props} title="Snooze" />,
  assign: AssignFields,
  tag: TagFields,
  untag: TagFields,
  update_contact: ContactFields,
  priority: (props) => <OneChoice {...props} title="Priority" name="value" choices={choicesOf(priorities)} />,
  unassign: (props) => <OneChoice {...props} title="Unassign" name="what" choices={choicesOf(unassignTargets)} />,
  close: CloseFields,
  reply_time: ReplyTimeFields,
  classify: ClassifyFields,
  split: SplitFields,
  run_workflow: RunWorkflowFields,
  http_request: CallFields,
  extract: ExtractFields,
  run_action: RunActionFields,
  ...Object.fromEntries(ticketStepKinds.map((kind) => [kind, TicketStepFields])),
};

export function StepFields(props: Props): ReactNode {
  useAIConditionFields();
  const Editor = editors[props.step.kind] ?? Unavailable;

  return <Editor {...props} />;
}
