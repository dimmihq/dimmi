/*
 * The controls settings rows use, BoardUI's own at the compact sizes the
 * rows call for. Until settings are saved on the server they keep their
 * state on the page, so every page can be tried as it will work.
 */
import { useState } from "react";
import { type Color, parseColor } from "react-aria-components";

import { Button } from "@/components/base/buttons/button";
import { ColorWell } from "@/components/base/color-picker/color-picker";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";

/*
 * Toggle switches a setting on or off, telling onChange when it flips.
 */
export function Toggle({
  label,
  on = false,
  onChange,
}: {
  label: string;
  on?: boolean;
  onChange?: (on: boolean) => void;
}) {
  return <Switch aria-label={label} size="sm" defaultSelected={on} onChange={onChange} />;
}

/*
 * Choice picks one of a few options, telling onChange which was picked.
 */
export function Choice({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: string[];
  value?: string;
  onChange?: (option: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      defaultSelectedKey={value ?? options[0]}
      onSelectionChange={(key) => onChange?.(String(key))}
      popoverClassName="w-56"
    >
      {options.map((option) => (
        <SelectItem key={option} id={option} textValue={option}>
          {option}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * brand holds the swatches offered beside any colour: the accent the app
 * itself uses, and a spread of common brand hues.
 */
const brand = ["#10b981", "#3b82f6", "#8b5cf6", "#ec4899", "#ef4444", "#f59e0b", "#171717"];

/*
 * Colour picks any colour with BoardUI's colour well, and shows the hex
 * value it holds beside it. The well is 20px, the size of the other marks
 * in a settings row, rather than BoardUI's 32px default.
 */
export function Colour({ label, value }: { label: string; value: string }) {
  const [color, setColor] = useState<Color>(() => parseColor(value));

  return (
    <span className="flex items-center gap-2">
      <span className="font-mono text-body-2-regular text-text-secondary uppercase">{color.toString("hex")}</span>
      <ColorWell
        aria-label={label}
        value={color}
        onChange={setColor}
        swatches={brand}
        placement="bottom end"
        wellClassName="size-5"
      />
    </span>
  );
}

/*
 * Field is a short text setting, such as a name, telling onChange as it
 * is typed.
 */
export function Field({
  label,
  value,
  placeholder,
  width = "w-56",
  onChange,
}: {
  label: string;
  value?: string;
  placeholder?: string;
  width?: string;
  onChange?: (value: string) => void;
}) {
  return (
    <Input
      aria-label={label}
      size="small"
      defaultValue={value}
      placeholder={placeholder}
      onChange={onChange}
      className={width}
    />
  );
}

/*
 * Action is a secondary button on a row, such as Change or Connect. Rows
 * in dense lists, such as templates, take the extra-small size.
 */
export function Action({
  children,
  onPress,
  size = "small",
}: {
  children: string;
  onPress?: () => void;
  size?: "small" | "xs";
}) {
  return (
    <Button variant="secondary" size={size} onClick={onPress}>
      {children}
    </Button>
  );
}

/*
 * Danger is the button of an action that cannot be undone, such as
 * deleting the workspace: the quiet destructive button, so a danger zone
 * reads as a warning without shouting over the page. The confirmation
 * that follows uses the solid one.
 */
export function Danger({ children, onPress }: { children: string; onPress?: () => void }) {
  return (
    <Button variant="danger-secondary" size="small" onClick={onPress}>
      {children}
    </Button>
  );
}
