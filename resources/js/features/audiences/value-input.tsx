/*
 * The value part of a condition, shaped by its operator: nothing for
 * "is known", a number of days for "within the last", two numbers for
 * "between", a day for "before", and for fields with fixed choices
 * (person type, device, tags, saved audiences) a picker of those
 * choices instead of free text.
 */
import { RiArrowDownSLine } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { Select, SelectItem } from "@/components/base/select/select";
import type { AudienceField, Condition } from "@/lib/api/audiences";

import { daysSuffix, listOf, listText, type Shape, shapeOf } from "./rules";

/*
 * Choice is one value a field may take: what is stored and what is shown.
 */
export type Choice = [string, string];

function MultiPick({
  label,
  choices,
  value,
  onChange,
}: {
  label: string;
  choices: Choice[];
  value: string[];
  onChange: (next: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const names = choices.filter(([id]) => value.includes(id)).map(([, name]) => name);

  return (
    <PopoverTrigger isOpen={open} onOpenChange={setOpen}>
      <Button variant="secondary" size="small" trailingIcon={RiArrowDownSLine} aria-label={label}>
        <span className="max-w-48 truncate">{names.length > 0 ? names.join(", ") : "Pick values"}</span>
      </Button>
      <Popover aria-label={label} placement="bottom start" className="max-h-72 w-60 overflow-y-auto">
        {choices.length === 0 ? (
          <p className="px-2 py-1.5 text-body-2-regular text-text-secondary">Nothing to pick yet.</p>
        ) : (
          <div className="flex flex-col gap-2 p-1.5">
            {choices.map(([id, name]) => (
              <Checkbox
                key={id}
                size="sm"
                isSelected={value.includes(id)}
                onChange={(on) => onChange(on ? [...value, id] : value.filter((v) => v !== id))}
              >
                {name}
              </Checkbox>
            ))}
          </div>
        )}
      </Popover>
    </PopoverTrigger>
  );
}

function OnePick({
  label,
  choices,
  value,
  onChange,
}: {
  label: string;
  choices: Choice[];
  value: string;
  onChange: (next: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      className="w-44"
      placeholder="Pick one"
      selectedKey={value || null}
      onSelectionChange={(key) => key !== null && onChange(String(key))}
    >
      {choices.map(([id, name]) => (
        <SelectItem key={id} id={id} textValue={name}>
          {name}
        </SelectItem>
      ))}
    </Select>
  );
}

function NumberInput({ label, value, onChange }: { label: string; value: unknown; onChange: (n: number) => void }) {
  return (
    <Input
      aria-label={label}
      size="small"
      inputMode="decimal"
      className="w-24"
      value={typeof value === "number" ? String(value) : ""}
      onChange={(text) => {
        const n = Number(text);
        if (text.trim() !== "" && Number.isFinite(n)) {
          onChange(n);
        }
      }}
    />
  );
}

function TextValue({
  label,
  shape,
  condition,
  onChange,
}: {
  label: string;
  shape: Shape;
  condition: Condition;
  onChange: (value: unknown) => void;
}) {
  const [text, setText] = useState(shape === "list" ? listText(condition.value) : String(condition.value ?? ""));

  return (
    <Input
      aria-label={label}
      size="small"
      className="w-44"
      placeholder={shape === "list" ? "pro, team" : shape === "date" ? "2026-12-31" : "Value"}
      value={text}
      onChange={(next) => {
        setText(next);
        onChange(shape === "list" ? listOf(next) : next);
      }}
    />
  );
}

export function ValueInput({
  field,
  condition,
  choices,
  onChange,
}: {
  field: AudienceField | undefined;
  condition: Condition;
  choices: Choice[] | undefined;
  onChange: (value: unknown) => void;
}) {
  const shape = shapeOf(condition.op, field?.type);
  const label = "Value";

  if (shape === "none") {
    return null;
  }
  if (shape === "days") {
    return (
      <span className="flex items-center gap-2">
        <NumberInput label="Days" value={condition.value} onChange={onChange} />
        <span className="text-body-2-regular text-text-secondary">{daysSuffix(condition.op)}</span>
      </span>
    );
  }
  if (shape === "number") {
    return <NumberInput label={label} value={condition.value} onChange={onChange} />;
  }
  if (shape === "range") {
    const [lo, hi] = Array.isArray(condition.value) ? condition.value : [0, 0];
    return (
      <span className="flex items-center gap-2">
        <NumberInput label="From" value={lo} onChange={(n) => onChange([n, hi])} />
        <span className="text-body-2-regular text-text-secondary">and</span>
        <NumberInput label="To" value={hi} onChange={(n) => onChange([lo, n])} />
      </span>
    );
  }
  if (choices && shape === "list") {
    const picked = Array.isArray(condition.value) ? condition.value.map(String) : [];
    return <MultiPick label={label} choices={choices} value={picked} onChange={onChange} />;
  }
  if (choices) {
    return <OnePick label={label} choices={choices} value={String(condition.value ?? "")} onChange={onChange} />;
  }

  return <TextValue key={condition.op} label={label} shape={shape} condition={condition} onChange={onChange} />;
}
