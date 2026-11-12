/*
 * The settings of the ticket steps and of applying an SLA. Types, states,
 * trackers, teams and policies are picked from the workspace's own, and
 * field values are set per field of the chosen type, each a value that
 * may hold variables such as {{run.vars.order_number}}, so answers a
 * collect step saved fill the ticket.
 */
import type { ReactNode } from "react";

import { Danger, Field } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";

import type { Choices } from "./choices";
import { type EditorProps, Note } from "./field-parts";
import { Pick } from "./pick";
import { text, useSet, withSettings } from "./step-settings";

type Props = EditorProps;

/*
 * TicketValue is one field of a ticket step, by key, and its value.
 */
export interface TicketValue {
  key: string;
  value: string;
}

/*
 * TicketType is a type as the pickers offer it, with its fields.
 */
export interface TicketTypeChoice {
  id: string;
  name: string;
  kind: string;
  fields: Choices;
}

/*
 * TicketLists are the workspace's ticket setup the ticket steps pick
 * from, and its SLA policies.
 */
export interface TicketLists {
  types: TicketTypeChoice[];
  states: Choices;
  trackers: Choices;
  policies: Choices;
}

const noTicketLists: TicketLists = { types: [], states: [], trackers: [], policies: [] };

function valuesOf(settings: Record<string, unknown> | undefined): TicketValue[] {
  const fields = settings?.fields;

  return Array.isArray(fields) ? (fields as TicketValue[]) : [];
}

function typesOf(lists: TicketLists, kind: string): Choices {
  return lists.types.filter((type) => type.kind === kind).map((type) => [type.id, type.name] as const);
}

function fieldsOf(lists: TicketLists, typeId: string): Choices {
  return lists.types.find((type) => type.id === typeId)?.fields ?? [];
}

/*
 * customerFields are the fields of every customer type, once each, for
 * a step that works whatever ticket the conversation has.
 */
function customerFields(lists: TicketLists): Choices {
  const seen = new Map<string, string>();
  for (const type of lists.types.filter((entry) => entry.kind === "customer")) {
    for (const [key, name] of type.fields) {
      if (!seen.has(key)) {
        seen.set(key, name);
      }
    }
  }

  return [...seen.entries()];
}

/*
 * ValueRows sets field values: one row per field set, named by the
 * field, with its value; a picker of the fields not set yet adds one.
 */
function ValueRows({ step, onEdit, fields }: Props & { fields: Choices }) {
  const values = valuesOf(step.settings);
  const save = (next: TicketValue[]) =>
    onEdit((current) => withSettings(current, { fields: next.length > 0 ? next : undefined }));
  const unused = fields.filter(([key]) => !values.some((value) => value.key === key));
  const nameOf = (key: string) => fields.find(([id]) => id === key)?.[1] ?? key;

  if (fields.length === 0) {
    return <Note>Pick a type with fields to fill them in.</Note>;
  }

  return (
    <>
      {values.map((item, index) => (
        <Row key={item.key} label={nameOf(item.key)}>
          <div className="flex items-center gap-2">
            <Field
              label={`${nameOf(item.key)} value`}
              value={item.value}
              width="w-40"
              placeholder="{{run.vars.order_number}}"
              onChange={(value) => save(values.map((old, at) => (at === index ? { ...old, value } : old)))}
            />
            <Danger onPress={() => save(values.filter((_, at) => at !== index))}>Remove</Danger>
          </div>
        </Row>
      ))}
      {unused.length > 0 ? (
        <Row label="Set a field">
          <Pick
            key={values.length}
            label="Field to set"
            placeholder="Pick a field"
            choices={unused}
            value=""
            onChange={(key) => save([...values, { key, value: "" }])}
          />
        </Row>
      ) : null}
    </>
  );
}

function CreateTicketFields(props: Props) {
  const { step, lists } = props;
  const set = useSet(props.onEdit);
  const tickets = lists.tickets ?? noTicketLists;
  const type = text(step.settings, "type_id");

  return (
    <>
      <Section title="Ticket" description="The conversation becomes this ticket. Required fields must be filled.">
        <Row label="Type">
          <Pick
            label="Ticket type"
            choices={typesOf(tickets, "customer")}
            value={type}
            onChange={(id) => set({ type_id: id, fields: undefined })}
          />
        </Row>
        <Row label="Title" description="Leave empty to use the conversation's subject.">
          <Field
            label="Ticket title"
            value={text(step.settings, "title")}
            placeholder="{{conversation.subject}}"
            onChange={(value) => set({ title: value || undefined })}
          />
        </Row>
        <Row label="Starts in">
          <Pick
            label="Starting state"
            placeholder="The type's first state"
            choices={tickets.states}
            value={text(step.settings, "state_id")}
            onChange={(id) => set({ state_id: id })}
          />
        </Row>
      </Section>
      <Section title="Field values" description="Map answers and variables, such as {{run.vars.order_number}}.">
        <ValueRows {...props} key={type} fields={fieldsOf(tickets, type)} />
      </Section>
    </>
  );
}

function SetStateFields({ step, lists, onEdit }: Props) {
  const set = useSet(onEdit);

  return (
    <Section title="Move the ticket" description="Resolving needs every field required to resolve.">
      <Row label="State">
        <Pick
          label="Ticket state"
          choices={(lists.tickets ?? noTicketLists).states}
          value={text(step.settings, "state_id")}
          onChange={(id) => set({ state_id: id })}
        />
      </Row>
    </Section>
  );
}

function SetFieldsFields(props: Props) {
  return (
    <Section title="Field values" description="Fields the ticket's type does not have are refused.">
      <ValueRows {...props} fields={customerFields(props.lists.tickets ?? noTicketLists)} />
    </Section>
  );
}

function EscalateFields(props: Props) {
  const { step, lists } = props;
  const set = useSet(props.onEdit);
  const tickets = lists.tickets ?? noTicketLists;
  const type = text(step.settings, "type_id");

  return (
    <>
      <Section title="Back-office work" description="The ticket waits on the team until the work is done.">
        <Row label="Type">
          <Pick
            label="Back-office type"
            choices={typesOf(tickets, "back_office")}
            value={type}
            onChange={(id) => set({ type_id: id, fields: undefined })}
          />
        </Row>
        <Row label="Title">
          <Field
            label="Work title"
            value={text(step.settings, "title")}
            placeholder="Refund for {{ticket.reference}}"
            onChange={(value) => set({ title: value })}
          />
        </Row>
        <Row label="Team">
          <Pick
            label="Team"
            placeholder="The type's team"
            choices={lists.teams}
            value={text(step.settings, "team_id")}
            onChange={(id) => set({ team_id: id })}
          />
        </Row>
      </Section>
      <Section title="Field values">
        <ValueRows {...props} key={type} fields={fieldsOf(tickets, type)} />
      </Section>
    </>
  );
}

function TrackerFields({ step, lists, onEdit }: Props) {
  const set = useSet(onEdit);
  const trackers = (lists.tickets ?? noTicketLists).trackers;

  return (
    <Section title="Tracker" description="The conversation joins the tracker and hears its updates.">
      {trackers.length === 0 ? (
        <Note>No open trackers yet. Create one in Tickets.</Note>
      ) : (
        <Row label="Tracker">
          <Pick
            label="Tracker"
            choices={trackers}
            value={text(step.settings, "tracker_id")}
            onChange={(id) => set({ tracker_id: id })}
          />
        </Row>
      )}
    </Section>
  );
}

function PolicyFields({ step, lists, onEdit }: Props) {
  const set = useSet(onEdit);
  const policies = (lists.tickets ?? noTicketLists).policies;

  return (
    <Section title="SLA policy" description="Applies to the conversation, and to its ticket when it has one.">
      {policies.length === 0 ? (
        <Note>No SLA policies yet. Add one through the API.</Note>
      ) : (
        <Row label="Policy">
          <Pick
            label="SLA policy"
            choices={policies}
            value={text(step.settings, "sla_policy_id")}
            onChange={(id) => set({ sla_policy_id: id })}
          />
        </Row>
      )}
    </Section>
  );
}

const ticketEditors: Record<string, (props: Props) => ReactNode> = {
  create_ticket: CreateTicketFields,
  set_ticket_state: SetStateFields,
  set_ticket_fields: SetFieldsFields,
  escalate: EscalateFields,
  add_to_tracker: TrackerFields,
  apply_sla: PolicyFields,
};

/*
 * TicketStepFields shows the settings of whichever ticket step it is
 * given; the builder maps every ticket step kind to it.
 */
export function TicketStepFields(props: Props): ReactNode {
  const Editor = ticketEditors[props.step.kind];

  return Editor ? <Editor {...props} /> : null;
}
