/*
 * What a customer sees when they open a type's form in the messenger or
 * portal, kept live beside the editor: the type's name and description,
 * then only the fields shown to customers, with the ones they must fill
 * marked. Answers typed into the preview drive each field's show-when,
 * so a conditional field appears just as it would for the customer.
 * Nothing typed here is saved.
 */
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import type { TicketField } from "@/lib/api/tickets";

import { customerFields } from "./form";

type Values = Record<string, unknown>;

function PreviewInput({
  field,
  value,
  onChange,
}: {
  field: TicketField;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const label = field.label;
  const common = { label, hint: field.description || undefined, isRequired: field.required_create_customer };

  switch (field.data_type) {
    case "long_text":
      return <Textarea {...common} rows={3} value={String(value ?? "")} onChange={onChange} />;
    case "single_choice":
      return (
        <Select
          label={label}
          isRequired={field.required_create_customer}
          placeholder="Choose one"
          selectedKey={(value as string | undefined) ?? null}
          onSelectionChange={(key) => onChange(key === null ? undefined : String(key))}
        >
          {field.options.map((o) => (
            <SelectItem key={o} id={o} textValue={o}>
              {o}
            </SelectItem>
          ))}
        </Select>
      );
    case "multiple_choice": {
      const picked = (value as string[] | undefined) ?? [];
      return (
        <fieldset className="flex flex-col gap-2">
          <legend className="pb-1 text-body-2-medium text-text-primary">
            {label}
            {field.required_create_customer ? <span className="text-text-error-primary"> *</span> : null}
          </legend>
          {field.options.map((o) => (
            <Checkbox
              key={o}
              isSelected={picked.includes(o)}
              onChange={(on) => onChange(on ? [...picked, o] : picked.filter((v) => v !== o))}
            >
              {o}
            </Checkbox>
          ))}
        </fieldset>
      );
    }
    case "boolean":
      return (
        <Checkbox isSelected={value === true} onChange={(on) => onChange(on || undefined)}>
          {label}
        </Checkbox>
      );
    case "file":
      return (
        <div className="flex flex-col gap-1.5">
          <span className="text-body-2-medium text-text-primary">{label}</span>
          <div>
            <Button size="small" variant="secondary" disabled>
              Attach a file
            </Button>
          </div>
        </div>
      );
    default:
      return (
        <Input
          {...common}
          type={field.data_type === "date" ? "date" : "text"}
          inputMode={["number", "decimal", "currency"].includes(field.data_type) ? "decimal" : undefined}
          value={String(value ?? "")}
          onChange={(next) => onChange(next === "" ? undefined : next)}
        />
      );
  }
}

export function CustomerPreview({
  name,
  description,
  fields,
  shown,
}: {
  name: string;
  description: string;
  fields: TicketField[];
  shown: boolean;
}) {
  const [values, setValues] = useState<Values>({});
  const visible = customerFields(fields, values);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between gap-2 px-1">
        <span className="text-body-2-medium text-text-secondary">Customer form preview</span>
        <span className="text-caption-1-regular text-text-tertiary">
          {shown ? "Shown in the messenger" : "Not in the messenger"}
        </span>
      </div>
      <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-primary-default p-4 shadow-xs">
        <div className="flex flex-col gap-1">
          <span className="text-headline-medium text-text-primary">{name || "Untitled type"}</span>
          {description ? <span className="text-body-2-regular text-text-secondary">{description}</span> : null}
        </div>
        <Input label="Subject" placeholder="What do you need help with?" isRequired />
        {visible.map((field) =>
          field.editable_customer ? (
            <PreviewInput
              key={field.id}
              field={field}
              value={values[field.key]}
              onChange={(value) => setValues((was) => ({ ...was, [field.key]: value }))}
            />
          ) : (
            <div key={field.id} className="flex flex-col gap-0.5">
              <span className="text-body-2-medium text-text-primary">{field.label}</span>
              <span className="text-body-2-regular text-text-tertiary">Filled in by the team</span>
            </div>
          ),
        )}
        <Button size="small" disabled>
          Submit request
        </Button>
      </div>
      <p className="px-1 text-caption-1-regular text-text-tertiary">
        Answers you type here only drive the preview; fields marked * are required for customers.
      </p>
    </div>
  );
}
