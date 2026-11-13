/*
 * One ticket field as a teammate fills it in, in the panel, the convert
 * dialog and the ticket page: the control fits the field's data type,
 * required ones are marked, the AI's suggestion shows under it with a
 * button that keeps it, and a value the server refused shows its reason
 * in place. Teammates are picked from the workspace's members, never
 * typed.
 */
import { RiSparkling2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import type { TicketField } from "@/lib/api/tickets";
import { membersQuery } from "@/lib/api/workspace";

import { fieldText } from "./labels";
import { filled } from "./settings/form";

/*
 * readValue turns what a control holds into what the API stores.
 */
function readValue(field: TicketField, typed: string): unknown {
  if (typed === "") {
    return null;
  }
  if (field.data_type === "number") {
    const n = Number.parseInt(typed, 10);
    return Number.isNaN(n) ? typed : n;
  }
  if (field.data_type === "decimal" || field.data_type === "currency") {
    const n = Number.parseFloat(typed);
    return Number.isNaN(n) ? typed : n;
  }

  return typed;
}

function Required({ field, purpose }: { field: TicketField; purpose?: "create" | "resolve" }) {
  const create = field.required_create && purpose !== "resolve";
  const resolve = field.required_resolve && purpose !== "create";
  if (!create && !resolve) {
    return null;
  }

  return (
    <span className="text-caption-1-medium text-text-tertiary">{create ? "Required" : "Required to resolve"}</span>
  );
}

function Suggestion({ value, onAccept }: { value: unknown; onAccept?: () => void }) {
  return (
    <span className="flex min-w-0 items-center gap-1.5 text-caption-1-medium text-text-secondary">
      <RiSparkling2Line aria-hidden className="size-3.5 shrink-0 text-foreground-icon-secondary" />
      <span className="min-w-0 truncate">AI · {fieldText(value)}</span>
      {onAccept ? (
        <LinkButton size="xs" variant="primary" onClick={onAccept}>
          Accept
        </LinkButton>
      ) : null}
    </span>
  );
}

export function FieldInput({
  workspace,
  field,
  value,
  suggestion,
  error,
  purpose,
  disabled,
  onChange,
  onAccept,
}: {
  workspace: string;
  field: TicketField;
  value: unknown;
  suggestion?: unknown;
  error?: string;
  purpose?: "create" | "resolve";
  disabled?: boolean;
  onChange: (value: unknown) => void;
  onAccept?: () => void;
}) {
  const label = (
    <span className="flex items-center gap-2">
      {field.label}
      <Required field={field} purpose={purpose} />
    </span>
  );

  return (
    <div className="flex flex-col gap-1" data-field={field.key}>
      <Control
        workspace={workspace}
        field={field}
        value={value}
        label={label}
        disabled={disabled}
        onChange={onChange}
      />
      {filled(suggestion) && fieldText(suggestion) !== fieldText(value) ? (
        <Suggestion value={suggestion} onAccept={onAccept} />
      ) : null}
      {error ? (
        <span role="alert" className="text-caption-1-medium text-text-error-primary">
          {error}
        </span>
      ) : null}
    </div>
  );
}

function Control({
  workspace,
  field,
  value,
  label,
  disabled,
  onChange,
}: {
  workspace: string;
  field: TicketField;
  value: unknown;
  label: React.ReactNode;
  disabled?: boolean;
  onChange: (value: unknown) => void;
}) {
  switch (field.data_type) {
    case "long_text":
      return (
        <Textarea
          label={label}
          rows={2}
          autoResize
          isDisabled={disabled}
          value={String(value ?? "")}
          onChange={(next) => onChange(next === "" ? null : next)}
        />
      );
    case "single_choice":
      return (
        <Select
          label={label}
          size="sm"
          placeholder="Choose one"
          isDisabled={disabled}
          selectedKey={typeof value === "string" ? value : null}
          onSelectionChange={(key) => onChange(key === null ? null : String(key))}
        >
          {field.options.map((o) => (
            <SelectItem key={o} id={o} textValue={o}>
              {o}
            </SelectItem>
          ))}
        </Select>
      );
    case "multiple_choice":
      return <Choices field={field} value={value} label={label} disabled={disabled} onChange={onChange} />;
    case "boolean":
      return (
        <Checkbox isDisabled={disabled} isSelected={value === true} onChange={(on) => onChange(on)}>
          {field.label}
        </Checkbox>
      );
    case "member":
      return <MemberPick workspace={workspace} value={value} label={label} disabled={disabled} onChange={onChange} />;
    default:
      return (
        <Input
          label={label}
          size="small"
          isDisabled={disabled}
          type={field.data_type === "date" ? "date" : "text"}
          inputMode={["number", "decimal", "currency"].includes(field.data_type) ? "decimal" : undefined}
          value={value === null || value === undefined ? "" : String(value)}
          onChange={(next) => onChange(readValue(field, next))}
        />
      );
  }
}

function Choices({
  field,
  value,
  label,
  disabled,
  onChange,
}: {
  field: TicketField;
  value: unknown;
  label: React.ReactNode;
  disabled?: boolean;
  onChange: (value: unknown) => void;
}) {
  const picked = Array.isArray(value) ? value.map(String) : [];

  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="pb-1 text-body-2-medium text-text-primary">{label}</legend>
      {field.options.map((o) => (
        <Checkbox
          key={o}
          size="sm"
          isDisabled={disabled}
          isSelected={picked.includes(o)}
          onChange={(on) => onChange(on ? [...picked, o] : picked.filter((v) => v !== o))}
        >
          {o}
        </Checkbox>
      ))}
    </fieldset>
  );
}

function MemberPick({
  workspace,
  value,
  label,
  disabled,
  onChange,
}: {
  workspace: string;
  value: unknown;
  label: React.ReactNode;
  disabled?: boolean;
  onChange: (value: unknown) => void;
}) {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });

  return (
    <Select
      label={label}
      size="sm"
      placeholder="Pick a teammate"
      isDisabled={disabled}
      selectedKey={typeof value === "string" ? value : null}
      onSelectionChange={(key) => onChange(key === null ? null : String(key))}
    >
      {members
        .filter((m) => m.status === "active")
        .map((m) => (
          <SelectItem key={m.id} id={m.id} textValue={m.user.name}>
            {m.user.name}
          </SelectItem>
        ))}
    </Select>
  );
}
