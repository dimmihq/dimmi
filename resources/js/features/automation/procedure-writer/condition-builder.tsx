/*
 * The rule of a condition dimmi decides in code: match all or any of the
 * rules, each a field, a comparison and a value. Fields are the
 * customer's and company's details, the conversation's own, and the
 * fields earlier actions returned in their last live test; any other
 * dotted field can be typed. A rule that reads an action's result waits
 * for that action before either side of the condition is shown.
 */
import { RiAddLine, RiDeleteBinLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { Input } from "@/components/base/input/input";
import type { ConditionExpr, ConditionOperator, ConditionRule } from "@/lib/api/procedures";
import type { Choices } from "../choices";
import { Pick } from "../pick";
import type { Field } from "./procedure-suggestions";
import { type FieldType, fitOperator, modeChoices, operatorsFor, readTyped, writeValue } from "./rules";

/*
 * ValueInput is how a rule's value is entered, by the field's type: yes
 * or no for a true-or-false field, a pick from the list for a field with
 * fixed values, a number field for numbers, and text for the rest.
 */
function ValueInput({
  rule,
  index,
  field,
  onChange,
}: {
  rule: ConditionRule;
  index: number;
  field: Field | undefined;
  onChange: (value: unknown) => void;
}) {
  const type: FieldType = field?.type ?? "text";
  const label = `Rule ${index + 1} value`;
  const many = rule.operator === "in" || rule.operator === "not_in";
  if (type === "boolean") {
    return <YesNo rule={rule} label={label} onChange={onChange} />;
  }
  if (field?.options && field.options.length > 0 && !many) {
    return (
      <Pick
        key={`${rule.field}:choice`}
        label={label}
        choices={field.options}
        value={typeof rule.value === "string" ? rule.value : ""}
        placeholder="Pick one"
        className="w-32 shrink-0"
        onChange={onChange}
      />
    );
  }

  return (
    <div className="w-32 shrink-0">
      <Input
        key={`${rule.field}:${type}`}
        aria-label={label}
        size="small"
        inputMode={type === "number" ? "decimal" : undefined}
        placeholder={placeholderFor(type, many)}
        defaultValue={writeValue(rule.value)}
        onChange={(text) => onChange(readTyped(type, rule.operator, text))}
      />
    </div>
  );
}

/*
 * placeholderFor is what an empty value field suggests.
 */
function placeholderFor(type: FieldType, many: boolean): string {
  if (many) {
    return "a, b, c";
  }

  return { number: "Number", date: "YYYY-MM-DD" }[type as "number" | "date"] ?? "Value";
}

/*
 * YesNo picks the value of a true-or-false field.
 */
function YesNo({ rule, label, onChange }: { rule: ConditionRule; label: string; onChange: (v: boolean) => void }) {
  const picked = typeof rule.value === "boolean" ? String(rule.value) : "";

  return (
    <Pick
      key={`${rule.field}:bool`}
      label={label}
      choices={yesNo}
      value={picked}
      placeholder="Yes or no"
      className="w-32 shrink-0"
      onChange={(v) => onChange(v === "true")}
    />
  );
}

const yesNo: Choices = [
  ["true", "Yes"],
  ["false", "No"],
];

function RuleRow({
  rule,
  index,
  fields,
  onChange,
  onRemove,
}: {
  rule: ConditionRule;
  index: number;
  fields: Field[];
  onChange: (rule: ConditionRule) => void;
  onRemove: () => void;
}) {
  const field = fields.find((f) => f.field === rule.field);
  const type: FieldType = field?.type ?? "text";
  const valued = rule.operator !== "exists" && rule.operator !== "not_exists";
  /* A new field keeps a comparison it allows and starts with no value. */
  const pickField = (name: string) => {
    if (name === rule.field) {
      return;
    }
    const next = fields.find((f) => f.field === name);
    onChange({ field: name, operator: fitOperator(next?.type ?? "text", rule.operator), value: "" });
  };

  return (
    <div className="flex items-center gap-2">
      <ComboBox
        aria-label={`Rule ${index + 1} field`}
        className="min-w-0 flex-1"
        size="small"
        placeholder="Pick a field"
        allowsCustomValue
        menuTrigger="focus"
        defaultItems={fields.map((f) => ({ id: f.field, ...f }))}
        defaultInputValue={rule.field}
        onInputChange={pickField}
      >
        {(item) => (
          <ComboBoxItem id={item.field} textValue={item.field}>
            <span className="flex min-w-0 flex-col">
              <span className="truncate">{item.label}</span>
              <span className="truncate text-body-2-regular text-text-tertiary">{item.field}</span>
            </span>
          </ComboBoxItem>
        )}
      </ComboBox>
      <Pick
        key={`${rule.field}:op`}
        label={`Rule ${index + 1} comparison`}
        choices={operatorsFor(type)}
        value={fitOperator(type, rule.operator)}
        onChange={(operator) => onChange({ ...rule, operator: operator as ConditionOperator })}
      />
      {valued ? (
        <ValueInput rule={rule} index={index} field={field} onChange={(value) => onChange({ ...rule, value })} />
      ) : null}
      <IconButton
        variant="quiet"
        size="small"
        icon={RiDeleteBinLine}
        aria-label={`Remove rule ${index + 1}`}
        onClick={onRemove}
      />
    </div>
  );
}

export function ConditionBuilder({
  condition,
  fields,
  onChange,
}: {
  condition: ConditionExpr;
  fields: Field[];
  onChange: (condition: ConditionExpr) => void;
}) {
  const rules = condition.rules;
  const set = (index: number, rule: ConditionRule | null) =>
    onChange({
      ...condition,
      rules: rule === null ? rules.filter((_, at) => at !== index) : rules.map((r, at) => (at === index ? rule : r)),
    });

  return (
    <div className="flex flex-col gap-2 rounded-xl bg-background-secondary-default p-2.5">
      <div className="flex items-center gap-2">
        <Pick
          label="Match rules"
          choices={modeChoices}
          value={condition.mode}
          onChange={(mode) => onChange({ ...condition, mode: mode as "all" | "any" })}
        />
        <span className="text-body-2-regular text-text-tertiary">
          {condition.mode === "any"
            ? "Takes Then when any one of these rules holds."
            : "Takes Then only when every one of these rules holds."}
        </span>
      </div>
      {rules.map((rule, index) => (
        <RuleRow
          // biome-ignore lint/suspicious/noArrayIndexKey: rules have no id of their own and only change in place.
          key={index}
          rule={rule}
          index={index}
          fields={fields}
          onChange={(next) => set(index, next)}
          onRemove={() => set(index, null)}
        />
      ))}
      <span>
        <Button
          variant="tertiary"
          size="small"
          leadingIcon={RiAddLine}
          onClick={() => onChange({ ...condition, rules: [...rules, { field: "", operator: "eq", value: "" }] })}
        >
          Add rule
        </Button>
      </span>
    </div>
  );
}
