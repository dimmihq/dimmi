/*
 * Picks a tag's icon from the ones the server accepts, as one radio group:
 * arrows move between icons, a press dips the icon the way BoardUI's
 * buttons do, and the chosen one eases into the accent fill.
 */
import { Radio, RadioGroup } from "react-aria-components";

import { Label } from "@/components/base/input/label";
import { cx } from "@/utils/cx";
import { tagIcons } from "./tag-icons";

function iconLabel(name: string): string {
  return name.replace(/-\d+$/, "").replaceAll("-", " ");
}

export function TagIconPicker({ value, onChange }: { value: string; onChange: (icon: string) => void }) {
  return (
    <RadioGroup value={value} onChange={onChange} orientation="horizontal" className="flex flex-col gap-1.5">
      <Label>Icon</Label>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(2.25rem,1fr))] gap-1">
        {Object.entries(tagIcons).map(([name, Icon]) => (
          <Radio
            key={name}
            value={name}
            aria-label={iconLabel(name)}
            className={({ isSelected, isFocusVisible, isHovered }) =>
              cx(
                "button-press-motion flex h-9 w-full cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none",
                isHovered && !isSelected && "bg-background-secondary-hover",
                isSelected && "bg-accent-500 text-white",
                isFocusVisible && "ring-2 ring-border-focus-ring",
              )
            }
          >
            <Icon aria-hidden className="size-4" />
          </Radio>
        ))}
      </div>
    </RadioGroup>
  );
}
