/*
 * A ticket's state and fields as a teammate edits them, in the
 * conversation's ticket panel and on the ticket page. The state picker
 * lists the states the type uses and says what the customer sees.
 * Field edits collect in a draft that saves in one go; the AI's
 * suggestions show under their field with a button that keeps them. A
 * clash with someone else's edit shows a prompt to load the latest, and
 * fields a move needs are marked where they sit.
 */
import { RiErrorWarningLine } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import type { Ticket, TicketField, TicketState, TicketType } from "@/lib/api/tickets";
import { i18n } from "@/lib/i18n";

import { FieldInput } from "./field-input";
import { shownFields } from "./field-rules";
import { groupLabel } from "./labels";
import type { TicketSave } from "./use-ticket-save";

export function ClashPrompt({ save }: { save: TicketSave }) {
  if (!save.clash) {
    return null;
  }

  return (
    <div
      role="alert"
      className="flex items-center gap-2 rounded-xl bg-background-tertiary-default px-3 py-2 text-body-2-regular text-text-primary"
    >
      <RiErrorWarningLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="min-w-0 flex-1">{i18n.t("tickets:errors.conflict")}</span>
      <Button size="xs" variant="secondary" onClick={() => void save.reload()}>
        {i18n.t("tickets:errors.reload")}
      </Button>
    </div>
  );
}

export function StatePicker({
  ticket,
  states,
  save,
  compact = false,
}: {
  ticket: Ticket;
  states: TicketState[];
  save: TicketSave;
  compact?: boolean;
}) {
  const current = states.find((s) => s.id === ticket.state_id);

  return (
    <div className="flex flex-col gap-1">
      <Select
        aria-label="State"
        label={compact ? undefined : "State"}
        size="sm"
        selectedKey={ticket.state_id}
        isDisabled={save.busy}
        onSelectionChange={(key) => {
          if (key !== null && key !== ticket.state_id) {
            void save.save({ state_id: String(key) });
          }
        }}
      >
        {states.map((s) => (
          <SelectItem key={s.id} id={s.id} textValue={s.name}>
            <span className="flex min-w-0 flex-col">
              <span className="truncate">{s.name}</span>
              <span className="truncate text-caption-1-regular text-text-tertiary">{groupLabel(s.group)}</span>
            </span>
          </SelectItem>
        ))}
      </Select>
      <span className="text-caption-1-regular text-text-tertiary">
        Customer sees “{current?.customer_label ?? ticket.state.customer_label}”
      </span>
    </div>
  );
}

/*
 * changedValues are the draft's values that differ from the ticket's.
 */
function changedValues(draft: Record<string, unknown>, saved: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(draft).filter(
      ([key, value]) => JSON.stringify(value ?? null) !== JSON.stringify(saved[key] ?? null),
    ),
  );
}

export function FieldsEditor({
  workspace,
  ticket,
  type,
  save,
}: {
  workspace: string;
  ticket: Ticket;
  type: TicketType | undefined;
  save: TicketSave;
}) {
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const values = { ...ticket.fields, ...draft };
  const fields: TicketField[] = shownFields(type?.fields ?? [], values);
  const changes = changedValues(draft, ticket.fields);
  const dirty = Object.keys(changes).length > 0;

  if (fields.length === 0) {
    return <p className="text-body-2-regular text-text-tertiary">This type has no fields.</p>;
  }

  return (
    <div className="flex flex-col gap-3">
      {fields.map((field) => (
        <FieldInput
          key={field.id}
          workspace={workspace}
          field={field}
          value={values[field.key]}
          suggestion={ticket.suggestions[field.key]}
          error={save.errorOf(field.key)}
          disabled={save.busy}
          onChange={(value) => setDraft((was) => ({ ...was, [field.key]: value }))}
          onAccept={() => void save.save({ accept_suggestions: [field.key] })}
        />
      ))}
      {dirty ? (
        <div className="flex justify-end gap-2">
          <Button size="small" variant="secondary" onClick={() => setDraft({})}>
            Discard
          </Button>
          <Button
            size="small"
            loading={save.busy}
            onClick={async () => {
              if (await save.save({ fields: changes })) {
                setDraft({});
              }
            }}
          >
            Save fields
          </Button>
        </div>
      ) : null}
    </div>
  );
}
