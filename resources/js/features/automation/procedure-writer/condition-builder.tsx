/*
 * The rule of a condition dimmi decides in code: match all or any of the
 * rules, each a field, a comparison and a value. Fields are the
 * customer's and company's details, the conversation's own, and the
 * fields earlier actions returned in their last live test; any other
 * dotted field can be typed. A rule that reads an action's result waits
 * for that action before either side of the condition is shown.
 */
import { RiDeleteBinLine } from "@remixicon/react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { Input } from "@/components/base/input/input";
import { Action } from "@/features/settings/controls";
import type { ConditionExpr, ConditionOperator, ConditionRule } from "@/lib/api/procedures";

import { Pick } from "../pick";
import type { Field } from "./procedure-suggestions";
import { modeChoices, operatorChoices, readValue, writeValue } from "./rules";

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
  const valued = rule.operator !== "exists" && rule.operator !== "not_exists";

  return (
    <div className="flex flex-wrap items-center gap-2">
      <ComboBox
        aria-label={`Rule ${index + 1} field`}
        className="w-56"
        size="small"
        placeholder="Pick a field"
        allowsCustomValue
        menuTrigger="focus"
        defaultItems={fields.map((f) => ({ id: f.field, ...f }))}
        defaultInputValue={rule.field}
        onInputChange={(field) => onChange({ ...rule, field })}
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
        label={`Rule ${index + 1} comparison`}
        choices={operatorChoices}
        value={rule.operator}
        onChange={(operator) => onChange({ ...rule, operator: operator as ConditionOperator })}
      />
      {valued ? (
        <div className="w-40">
          <Input
            aria-label={`Rule ${index + 1} value`}
            size="small"
            placeholder={rule.operator === "in" || rule.operator === "not_in" ? "a, b, c" : "Value"}
            defaultValue={writeValue(rule.value)}
            onChange={(text) => onChange({ ...rule, value: readValue(rule.operator, text) })}
          />
        </div>
      ) : null}
      <IconButton size="small" icon={RiDeleteBinLine} aria-label={`Remove rule ${index + 1}`} onClick={onRemove} />
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
        <span className="text-body-2-regular text-text-tertiary">dimmi decides this itself, exactly.</span>
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
        <Action onPress={() => onChange({ ...condition, rules: [...rules, { field: "", operator: "eq", value: "" }] })}>
          Add rule
        </Action>
      </span>
    </div>
  );
}
