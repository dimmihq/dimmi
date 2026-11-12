/*
 * Adding or changing one field of a ticket type: its label, its key and
 * data type (both fixed once the field exists), the options of a choice,
 * when it is required (to create, separately for teammates and for
 * customers, and to resolve), what the customer sees of it, when it
 * shows (only once another field has a value, or one of the values
 * picked from that field's options), and how the AI fills it.
 */
import { useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { type FieldChange, type FieldDataType, fieldDataTypes, type TicketField } from "@/lib/api/tickets";

import { dataTypeLabel } from "../labels";
import { fieldKey } from "./form";

const always = "__always";
const choices: FieldDataType[] = ["single_choice", "multiple_choice"];

function Heading({ children }: { children: string }) {
  return <span className="-mb-1 text-body-2-medium text-text-secondary">{children}</span>;
}

/*
 * optionList reads the options textarea: one per line, trimmed, no
 * blanks or repeats.
 */
function optionList(text: string): string[] {
  return [...new Set(text.split("\n").map((line) => line.trim()))].filter(Boolean);
}

type Flags = Pick<
  TicketField,
  | "required_create"
  | "required_create_customer"
  | "required_resolve"
  | "visible_customer"
  | "editable_customer"
  | "ai_auto"
>;

/*
 * startOf is what the dialog starts with: the field's settings, or a new
 * field's defaults, shown to customers and editable by them.
 */
function startOf(field: TicketField | null) {
  if (!field) {
    return {
      label: "",
      key: "",
      dataType: "short_text" as FieldDataType,
      options: "",
      description: "",
      flags: {
        required_create: false,
        required_create_customer: false,
        required_resolve: false,
        visible_customer: true,
        editable_customer: true,
        ai_auto: false,
      } as Flags,
      when: always,
      equals: [] as string[],
      hint: "",
    };
  }

  return {
    label: field.label,
    key: field.key,
    dataType: field.data_type,
    options: field.options.join("\n"),
    description: field.description,
    flags: {
      required_create: field.required_create,
      required_create_customer: field.required_create_customer,
      required_resolve: field.required_resolve,
      visible_customer: field.visible_customer,
      editable_customer: field.editable_customer,
      ai_auto: field.ai_auto,
    },
    when: field.show_when?.field ?? always,
    equals: field.show_when?.equals ?? [],
    hint: field.ai_hint ?? "",
  };
}

/*
 * changeOf is what saving the dialog sends. Values to match only apply
 * when the field depended on is a choice.
 */
function changeOf(
  form: {
    label: string;
    description: string;
    flags: Flags;
    dataType: FieldDataType;
    options: string;
    when: string;
    equals: string[];
    hint: string;
  },
  others: TicketField[],
): FieldChange {
  const parent = others.find((f) => f.key === form.when);
  const matches = parent && choices.includes(parent.data_type) ? form.equals : [];

  return {
    label: form.label.trim(),
    description: form.description,
    ...form.flags,
    options: choices.includes(form.dataType) ? optionList(form.options) : undefined,
    show_when: form.when === always ? null : { field: form.when, equals: matches },
    ai_hint: form.hint.trim() || null,
  };
}

export function FieldDialog({
  field,
  others,
  onSave,
  onClose,
}: {
  field: TicketField | null;
  others: TicketField[];
  onSave: (change: FieldChange) => Promise<unknown>;
  onClose: () => void;
}) {
  const formId = useId();
  const start = startOf(field);
  const [label, setLabel] = useState(start.label);
  const [key, setKey] = useState(start.key);
  const [keyTouched, setKeyTouched] = useState(false);
  const [dataType, setDataType] = useState<FieldDataType>(start.dataType);
  const [options, setOptions] = useState(start.options);
  const [description, setDescription] = useState(start.description);
  const [flags, setFlags] = useState(start.flags);
  const [when, setWhen] = useState(start.when);
  const [equals, setEquals] = useState<string[]>(start.equals);
  const [hint, setHint] = useState(start.hint);
  const [saving, setSaving] = useState(false);
  const shownKey = field ? field.key : keyTouched ? key : fieldKey(label);
  const flag = (name: keyof Flags) => (on: boolean) => setFlags((was) => ({ ...was, [name]: on }));

  const submit = async () => {
    setSaving(true);
    const change = changeOf({ label, description, flags, dataType, options, when, equals, hint }, others);
    if (!field) {
      change.key = shownKey;
      change.data_type = dataType;
    }
    try {
      await onSave(change);
      onClose();
    } catch {
      // The caller said why.
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={field ? `Edit ${field.label}` : "New field"}
        description={
          field ? "The key and data type stay as they are." : "Teammates fill it in; customers too if it shows to them."
        }
      />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <form
          id={formId}
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          className="flex flex-col gap-4"
        >
          <Input label="Label" value={label} onChange={setLabel} maxLength={80} isRequired autoFocus />
          <div className="flex flex-col gap-4 sm:flex-row">
            <Input
              label="Key"
              value={shownKey}
              onChange={(next) => {
                setKeyTouched(true);
                setKey(next);
              }}
              isDisabled={field !== null}
              maxLength={40}
              hint="Used by the API and workflows."
              className="flex-1"
            />
            <Select
              label="Data type"
              selectedKey={dataType}
              isDisabled={field !== null}
              onSelectionChange={(next) => setDataType(next as FieldDataType)}
              className="flex-1"
            >
              {fieldDataTypes.map((t) => (
                <SelectItem key={t} id={t} textValue={dataTypeLabel(t)}>
                  {dataTypeLabel(t)}
                </SelectItem>
              ))}
            </Select>
          </div>
          {choices.includes(dataType) ? (
            <Textarea label="Options" value={options} onChange={setOptions} rows={4} hint="One option per line." />
          ) : null}
          <Input label="Help text" value={description} onChange={setDescription} maxLength={300} />
        </form>
        <Heading>Required</Heading>
        <div className="flex flex-col gap-2">
          <Checkbox isSelected={flags.required_create} onChange={flag("required_create")}>
            To create, when a teammate opens the ticket
          </Checkbox>
          <Checkbox isSelected={flags.required_create_customer} onChange={flag("required_create_customer")}>
            To create, when a customer submits the form
          </Checkbox>
          <Checkbox isSelected={flags.required_resolve} onChange={flag("required_resolve")}>
            To resolve, for everyone: inbox, API, AI agent and workflows
          </Checkbox>
        </div>
        <Heading>Customer</Heading>
        <div className="flex flex-col gap-2">
          <Switch size="sm" isSelected={flags.visible_customer} onChange={flag("visible_customer")}>
            Visible to the customer
          </Switch>
          <Switch
            size="sm"
            isSelected={flags.visible_customer && flags.editable_customer}
            isDisabled={!flags.visible_customer}
            onChange={flag("editable_customer")}
          >
            The customer can fill it in
          </Switch>
        </div>
        <ShowWhenPicker
          others={others}
          when={when}
          equals={equals}
          onWhen={(next) => {
            setWhen(next);
            setEquals([]);
          }}
          onEquals={setEquals}
        />
        <Heading>AI</Heading>
        <Textarea
          label="How the AI fills it"
          value={hint}
          onChange={setHint}
          rows={2}
          maxLength={500}
          placeholder="The order number the customer mentions, such as 8812."
        />
        <Switch size="sm" isSelected={flags.ai_auto} onChange={flag("ai_auto")}>
          Fill it automatically, without waiting for a teammate to accept
        </Switch>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button
          size="small"
          type="submit"
          form={formId}
          loading={saving}
          disabled={label.trim() === "" || shownKey === ""}
        >
          {field ? "Save field" : "Add field"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * ShowWhenPicker picks the field this one depends on, and for a choice
 * field the values that show it; none picked means any value.
 */
function ShowWhenPicker({
  others,
  when,
  equals,
  onWhen,
  onEquals,
}: {
  others: TicketField[];
  when: string;
  equals: string[];
  onWhen: (key: string) => void;
  onEquals: (values: string[]) => void;
}) {
  const parent = others.find((f) => f.key === when);

  return (
    <>
      <Select label="Show" selectedKey={when} onSelectionChange={(next) => onWhen(String(next))}>
        {[
          <SelectItem key={always} id={always} textValue="Always">
            Always
          </SelectItem>,
          ...others.map((f) => (
            <SelectItem key={f.key} id={f.key} textValue={`When ${f.label} is filled`}>
              When {f.label} is filled
            </SelectItem>
          )),
        ]}
      </Select>
      {parent && choices.includes(parent.data_type) && parent.options.length > 0 ? (
        <div className="flex flex-col gap-2">
          <span className="text-body-2-regular text-text-secondary">
            Only for these answers to {parent.label} (none picked means any)
          </span>
          {parent.options.map((option) => (
            <Checkbox
              key={option}
              isSelected={equals.includes(option)}
              onChange={(on) => onEquals(on ? [...equals, option] : equals.filter((v) => v !== option))}
            >
              {option}
            </Checkbox>
          ))}
        </div>
      ) : null}
    </>
  );
}
