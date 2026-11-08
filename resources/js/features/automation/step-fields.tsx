/*
 * The settings of one step in the inspector. Every kind has its own few
 * settings, stored the way the server reads them; every change goes back
 * to the step through onEdit, and its card says the new sentence at
 * once. Teams, teammates and tags are picked from the workspace's own.
 * A kind the server does not run yet only says so.
 */
import type { ReactNode } from "react";

import { Textarea } from "@/components/base/textarea/textarea";
import { Action, Danger, Field, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import type { Condition } from "@/lib/api/workflows";
import { type Choices, choicesOf } from "./choices";
import {
  changeField,
  changeOp,
  conditionFields,
  conditionOf,
  fieldsFor,
  isSimple,
  needsValue,
  opsFor,
  valueList,
  valueText,
} from "./conditions";
import { Pick, Picks } from "./pick";
import {
  collectTargets,
  contactFields,
  durations,
  list,
  priorities,
  type Settings,
  text,
  unassignTargets,
  unsetSummary,
  withNewPath,
  withoutPath,
  withPathChange,
  withPathLabel,
  withSettings,
} from "./step-settings";
import type { TriggerKind } from "./triggers";
import { pathName, type Step } from "./workflows";

type Edit = (edit: (step: Step) => Step) => void;

/*
 * Lists are the workspace's teams, teammates and tags, as choices, and
 * whether its agent is on, unknown until read.
 */
export interface Lists {
  teams: Choices;
  members: Choices;
  tags: Choices;
  agentOn?: boolean;
}

interface Props {
  step: Step;
  trigger: TriggerKind;
  lists: Lists;
  onEdit: Edit;
}

function Words({
  label,
  value,
  placeholder = unsetSummary,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="py-2.5 pe-2.5">
      <Textarea
        aria-label={label}
        defaultValue={value}
        placeholder={placeholder}
        onChange={onChange}
        rows={3}
        autoResize
        maxRows={8}
      />
    </div>
  );
}

function useSet(onEdit: Edit) {
  return (changes: Settings) => onEdit((current) => withSettings(current, changes));
}

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
  onChange,
}: {
  trigger: TriggerKind;
  condition: Condition | undefined;
  onChange: (condition: Condition) => void;
}) {
  if (!isSimple(condition)) {
    return <Row label="Condition" description="Set through the API. Edit it there." />;
  }
  const field = condition?.field ?? "";
  const op = condition?.op ?? "";
  const options = conditionFields[field]?.options;

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
          choices={choicesOf(options)}
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

function BranchFields({ step, trigger, onEdit }: Props) {
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
    </Section>
  );
}

const replyLimits: Record<string, string> = {
  agent: "The agent's own limit",
  "3": "3 replies",
  "5": "5 replies",
  "10": "10 replies",
  "20": "20 replies",
};

const quietWaits: Record<string, string> = {
  channel: "10 minutes in chat, 1 day by email",
  "5m": "5 minutes",
  "30m": "30 minutes",
  "1h": "1 hour",
  "4h": "4 hours",
  "1d": "1 day",
};

/*
 * DimmiFields shows how long dimmi keeps the conversation and what it
 * is told, and repeats the rules it always keeps.
 */
function DimmiFields({ step, lists, onEdit }: Props) {
  const set = useSet(onEdit);
  const limit = step.settings?.max_replies;

  return (
    <>
      {lists.agentOn === false ? (
        <Section
          title="dimmi is off"
          description="Until you turn dimmi on in Agent settings, this step hands every conversation to the team."
        />
      ) : null}
      <Section title="Extra instructions" description="Added to dimmi's guidance while this step is active.">
        <Words
          label="Extra instructions"
          value={text(step.settings, "instructions")}
          placeholder="Only answer billing questions; hand refunds to the team."
          onChange={(value) => set({ instructions: value || undefined })}
        />
      </Section>
      <Section title="While dimmi handles it">
        <Row label="Replies at most">
          <Pick
            label="Replies at most"
            choices={choicesOf(replyLimits)}
            value={typeof limit === "number" ? String(limit) : "agent"}
            onChange={(value) => set({ max_replies: value === "agent" ? undefined : Number(value) })}
          />
        </Row>
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
        description="dimmi stops the moment a teammate replies. Silence never counts as resolved: it takes the quiet path."
      />
    </>
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
  note: MessageFields,
  buttons: ButtonsFields,
  collect: CollectFields,
  dimmi: DimmiFields,
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
};

export function StepFields(props: Props): ReactNode {
  const Editor = editors[props.step.kind] ?? Unavailable;

  return <Editor {...props} />;
}
