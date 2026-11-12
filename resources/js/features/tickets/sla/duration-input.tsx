/*
 * A duration typed as an amount and picked unit, read from and written
 * back as seconds. The unit follows what the seconds hold whole, so
 * "2 hours" stays in hours; changing the unit keeps the amount.
 */
import { useState } from "react";

import { NumberField } from "@/components/base/number-field/number-field";
import { Select, SelectItem } from "@/components/base/select/select";

import { type DurationUnit, durationUnits, toDuration, toSeconds } from "./form";

export function DurationInput({
  label,
  unitLabel,
  unitNames,
  seconds,
  min = 60,
  max,
  disabled = false,
  onChange,
}: {
  label: string;
  unitLabel: string;
  unitNames: Record<DurationUnit, string>;
  seconds: number;
  min?: number;
  max: number;
  disabled?: boolean;
  onChange: (seconds: number) => void;
}) {
  const [unit, setUnit] = useState<DurationUnit>(() => toDuration(seconds).unit);
  const amount = seconds / toSeconds({ amount: 1, unit });
  const clamp = (next: number) => Math.min(max, Math.max(min, next));

  return (
    <div className="flex items-center gap-2">
      <NumberField
        aria-label={label}
        size="small"
        minValue={1}
        value={Number.isInteger(amount) ? amount : Number(amount.toFixed(2))}
        isDisabled={disabled}
        onChange={(next) => onChange(clamp(toSeconds({ amount: Number.isNaN(next) ? 1 : next, unit })))}
        className="w-28"
      />
      <Select
        aria-label={unitLabel}
        size="sm"
        isDisabled={disabled}
        selectedKey={unit}
        onSelectionChange={(key) => {
          if (key === null) {
            return;
          }
          const next = key as DurationUnit;
          setUnit(next);
          onChange(clamp(toSeconds({ amount: Math.max(1, Math.round(amount)), unit: next })));
        }}
        className="w-28"
        popoverClassName="w-36"
      >
        {durationUnits.map((u) => (
          <SelectItem key={u} id={u} textValue={unitNames[u]}>
            {unitNames[u]}
          </SelectItem>
        ))}
      </Select>
    </div>
  );
}
