/*
 * The pickers the inspector's rows share. Pick chooses one of a list of
 * named things by its id, the way step settings store it, so two
 * teammates sharing a name are still told apart. Picks chooses any
 * number of them with a checkbox each.
 */
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Select, SelectItem } from "@/components/base/select/select";

import type { Choices } from "./choices";

export function Pick({
  label,
  choices,
  value,
  placeholder = "Pick one",
  onChange,
}: {
  label: string;
  choices: Choices;
  value: string;
  placeholder?: string;
  onChange: (id: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      placeholder={placeholder}
      defaultSelectedKey={value || null}
      onSelectionChange={(key) => onChange(String(key))}
      popoverClassName="w-56"
    >
      {choices.map(([id, name]) => (
        <SelectItem key={id} id={id} textValue={name}>
          {name}
        </SelectItem>
      ))}
    </Select>
  );
}

export function Picks({
  choices,
  value,
  empty,
  onChange,
}: {
  choices: Choices;
  value: string[];
  empty: string;
  onChange: (ids: string[]) => void;
}) {
  if (choices.length === 0) {
    return <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">{empty}</p>;
  }

  return (
    <div className="flex flex-col gap-2 px-3 py-2.5">
      {choices.map(([id, name]) => (
        <Checkbox
          key={id}
          size="sm"
          isSelected={value.includes(id)}
          onChange={(on) => onChange(on ? [...value, id] : value.filter((current) => current !== id))}
        >
          {name}
        </Checkbox>
      ))}
    </div>
  );
}
