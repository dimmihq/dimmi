/*
 * One ticket field as a teammate fills it in. In the convert dialog it
 * is a labelled form field; in the details panel and on the ticket page
 * it is a row whose value is edited in place. Either way the control
 * fits the field's data type,
 * required ones are marked, the AI's suggestion shows under it with a
 * button that keeps it, and a value the server refused shows its reason
 * in place. Teammates are picked from the workspace's members, never
 * typed.
 */
import { RiCheckLine, RiSparkling2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { quiet } from "@/features/inbox/customer/quiet";
import { QuietTextarea, Row } from "@/features/inbox/customer/rows";
import type { TicketField } from "@/lib/api/tickets";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

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
  inline = false,
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
  /* A row in a details list, its value edited in place, instead of a labelled form field. */
  inline?: boolean;
  onChange: (value: unknown) => void;
  onAccept?: () => void;
}) {
  const extras = (
    <>
      {filled(suggestion) && fieldText(suggestion) !== fieldText(value) ? (
        <Suggestion value={suggestion} onAccept={onAccept} />
      ) : null}
      {error ? (
        <span role="alert" className="text-caption-1-medium text-text-error-primary">
          {error}
        </span>
      ) : null}
    </>
  );

  if (inline) {
    const needed = field.required_create || field.required_resolve;

    const name = (
      <span className="truncate" title={needed ? `${field.label}, required` : field.label}>
        {field.label}
        {needed ? <span className="text-text-tertiary"> *</span> : null}
      </span>
    );
    const control = (
      <InlineControl workspace={workspace} field={field} value={value} disabled={disabled} onChange={onChange} />
    );

    /* Long text gets the panel's whole width under its label: beside it, several lines read as a cramped box. */
    if (field.data_type === "long_text") {
      return (
        <div className="flex flex-col pb-1" data-field={field.key}>
          <dt className="flex min-h-8 items-center text-body-2-regular text-text-tertiary">{name}</dt>
          <dd className="flex min-w-0 flex-col gap-0.5">
            {control}
            {extras}
          </dd>
        </div>
      );
    }

    return (
      <div className="flex flex-col" data-field={field.key}>
        <Row label={name}>{control}</Row>
        <div className="flex flex-col gap-0.5 ps-35 empty:hidden">{extras}</div>
      </div>
    );
  }

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
      {extras}
    </div>
  );
}

const empty = "Empty";
const yesNo = [
  { id: "true", name: "Yes" },
  { id: "false", name: "No" },
];

/*
 * InlineControl is the field's value as a details row shows it, edited
 * where it stands.
 */
function InlineControl({
  workspace,
  field,
  value,
  disabled,
  onChange,
}: {
  workspace: string;
  field: TicketField;
  value: unknown;
  disabled?: boolean;
  onChange: (value: unknown) => void;
}) {
  switch (field.data_type) {
    case "long_text":
      return (
        <QuietTextarea
          aria-label={field.label}
          placeholder={empty}
          isDisabled={disabled}
          value={String(value ?? "")}
          onChange={(next) => onChange(next === "" ? null : next)}
        />
      );
    case "single_choice":
      return (
        <Select
          aria-label={field.label}
          size="sm"
          variant="quiet"
          placeholder={empty}
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
      return <InlineChoices field={field} value={value} disabled={disabled} onChange={onChange} />;
    case "boolean":
      return (
        <Select
          aria-label={field.label}
          size="sm"
          variant="quiet"
          placeholder={empty}
          isDisabled={disabled}
          selectedKey={typeof value === "boolean" ? String(value) : null}
          onSelectionChange={(key) => onChange(key === null ? null : key === "true")}
        >
          {yesNo.map((choice) => (
            <SelectItem key={choice.id} id={choice.id} textValue={choice.name}>
              {choice.name}
            </SelectItem>
          ))}
        </Select>
      );
    case "member":
      return <MemberPick workspace={workspace} value={value} disabled={disabled} inline onChange={onChange} />;
    default:
      return (
        <Input
          aria-label={field.label}
          size="small"
          placeholder={empty}
          isDisabled={disabled}
          className="-ms-1.5 min-w-0 flex-1"
          fieldClassName="bg-transparent hover:bg-background-secondary-hover focus-within:bg-background-tertiary-default"
          inputClassName="text-body-2-regular"
          type={field.data_type === "date" ? "date" : "text"}
          inputMode={["number", "decimal", "currency"].includes(field.data_type) ? "decimal" : undefined}
          value={value === null || value === undefined ? "" : String(value)}
          onChange={(next) => onChange(readValue(field, next))}
        />
      );
  }
}

/*
 * InlineChoices shows the picked options as one line of text that opens
 * the list to tick and untick them.
 */
function InlineChoices({
  field,
  value,
  disabled,
  onChange,
}: {
  field: TicketField;
  value: unknown;
  disabled?: boolean;
  onChange: (value: unknown) => void;
}) {
  const picked = Array.isArray(value) ? value.map(String) : [];

  return (
    <Dropdown>
      <DropdownTrigger aria-label={field.label} isDisabled={disabled} className={quiet}>
        <span className={cx("truncate", picked.length === 0 && "text-text-tertiary")}>
          {picked.length === 0 ? empty : picked.join(", ")}
        </span>
      </DropdownTrigger>
      <DropdownPopover aria-label={field.label} className="w-56 p-1.5">
        {field.options.map((o) => (
          <DropdownItem
            key={o}
            selected={picked.includes(o)}
            onSelect={() => onChange(picked.includes(o) ? picked.filter((v) => v !== o) : [...picked, o])}
            className="px-2 py-1.5 text-body-2-medium"
          >
            <span className="min-w-0 flex-1 truncate">{o}</span>
            {picked.includes(o) ? (
              <RiCheckLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
            ) : null}
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
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
  inline = false,
  onChange,
}: {
  workspace: string;
  value: unknown;
  label?: React.ReactNode;
  disabled?: boolean;
  inline?: boolean;
  onChange: (value: unknown) => void;
}) {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });

  return (
    <Select
      label={inline ? undefined : label}
      aria-label={inline ? "Teammate" : undefined}
      variant={inline ? "quiet" : "default"}
      size="sm"
      placeholder={inline ? empty : "Pick a teammate"}
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
