/*
 * Picks a tag's colour: the palette as one row of swatches spread across
 * the width, arrows moving between them, and a last rainbow swatch with a
 * plus that opens the full picker for any other colour, showing that
 * colour once picked. The chosen swatch carries a ring in the focus
 * colour.
 */
import { RiAddLine } from "@remixicon/react";
import { parseColor, Radio, RadioGroup } from "react-aria-components";

import { ColorWell } from "@/components/base/color-picker/color-picker";
import { Label } from "@/components/base/input/label";
import { cx } from "@/utils/cx";

import { tagPalette } from "./tag-colors";

const rainbow = "conic-gradient(from 0deg, #ef4444, #eab308, #22c55e, #06b6d4, #6366f1, #d946ef, #ef4444)";

export function TagColorPicker({ value, onChange }: { value: string; onChange: (hex: string) => void }) {
  const custom = !tagPalette.includes(value);

  return (
    <RadioGroup
      value={custom ? "" : value}
      onChange={onChange}
      orientation="horizontal"
      className="flex flex-col gap-1.5"
    >
      <Label>Colour</Label>
      <div className="flex items-center justify-between">
        {tagPalette.map((hex) => (
          <Radio
            key={hex}
            value={hex}
            aria-label={hex}
            className={({ isSelected, isFocusVisible }) =>
              cx(
                "size-6 shrink-0 cursor-pointer rounded-full outline-none ring-offset-2 ring-offset-background-primary-default transition-[scale,box-shadow] duration-150 ease-out hover:scale-110 active:scale-95",
                (isSelected || isFocusVisible) && "ring-2 ring-border-focus-ring",
              )
            }
            style={{ backgroundColor: hex }}
          />
        ))}
        <span
          className={cx(
            "relative size-6 shrink-0 rounded-full transition-[box-shadow] duration-150 ease-out",
            custom && "ring-2 ring-border-focus-ring ring-offset-2 ring-offset-background-primary-default",
          )}
        >
          <ColorWell
            aria-label="Custom colour"
            value={parseColor(value)}
            onChange={(next) => onChange(next.toString("hex").toLowerCase())}
            swatches={tagPalette}
            placement="bottom end"
            wellClassName="size-6"
          />
          <span
            aria-hidden
            className="pointer-events-none absolute inset-0 flex items-center justify-center rounded-full"
            style={custom ? undefined : { backgroundImage: rainbow }}
          >
            <RiAddLine className="size-3.5 text-text-white drop-shadow" />
          </span>
        </span>
      </div>
    </RadioGroup>
  );
}
