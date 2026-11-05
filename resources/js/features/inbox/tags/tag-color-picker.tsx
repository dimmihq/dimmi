/*
 * Picks a tag's colour: the palette as one row of swatches, arrows moving
 * between them, and a last swatch that opens the full picker for any
 * other colour. The chosen swatch carries a ring in the focus colour.
 */
import { parseColor, Radio, RadioGroup } from "react-aria-components";

import { ColorWell } from "@/components/base/color-picker/color-picker";
import { Label } from "@/components/base/input/label";
import { cx } from "@/utils/cx";

import { tagPalette } from "./tag-colors";

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
      <div className="flex flex-wrap items-center gap-1.5">
        {tagPalette.map((hex) => (
          <Radio
            key={hex}
            value={hex}
            aria-label={hex}
            className={({ isSelected, isFocusVisible }) =>
              cx(
                "size-5 cursor-pointer rounded-full outline-none ring-offset-2 ring-offset-background-primary-default transition-[scale] duration-150 ease-out hover:scale-110",
                (isSelected || isFocusVisible) && "ring-2 ring-border-focus-ring",
              )
            }
            style={{ backgroundColor: hex }}
          />
        ))}
        <span
          className={cx(
            "rounded-full",
            custom && "ring-2 ring-border-focus-ring ring-offset-2 ring-offset-background-primary-default",
          )}
        >
          <ColorWell
            aria-label="Custom colour"
            value={parseColor(value)}
            onChange={(next) => onChange(next.toString("hex").toLowerCase())}
            swatches={tagPalette}
            placement="bottom end"
            wellClassName="size-5"
          />
        </span>
      </div>
    </RadioGroup>
  );
}
