/*
 * The controls settings rows use, BoardUI's own at the compact sizes the
 * rows call for. They keep their own state and tell onChange, so a page
 * saves what it is told. A disabled control shows a setting the person
 * may read but not change.
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
  disabled = false,
  onChange,
}: {
  label: string;
  on?: boolean;
  disabled?: boolean;
  onChange?: (on: boolean) => void;
}) {
  return <Switch aria-label={label} size="sm" defaultSelected={on} isDisabled={disabled} onChange={onChange} />;
}

/*
 * Choice picks one of a few options, telling onChange which was picked.
 */
export function Choice({
  label,
  options,
  value,
  disabled = false,
  onChange,
}: {
  label: string;
  options: string[];
  value?: string;
  disabled?: boolean;
  onChange?: (option: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      isDisabled={disabled}
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
 * value it holds beside it, telling onChange each hex picked. The well
 * is 20px, the size of the other marks in a settings row, rather than
 * BoardUI's 32px default. Disabled, it shows the colour without the
 * well.
 */
export function Colour({
  label,
  value,
  disabled = false,
  onChange,
}: {
  label: string;
  value: string;
  disabled?: boolean;
  onChange?: (hex: string) => void;
}) {
  const [color, setColor] = useState<Color>(() => parseColor(value));
  const hex = (
    <span className="font-mono text-body-2-regular text-text-secondary uppercase">{color.toString("hex")}</span>
  );

  if (disabled) {
    return hex;
  }

  return (
    <span className="flex items-center gap-2">
      {hex}
      <ColorWell
        aria-label={label}
        value={color}
        onChange={(picked) => {
          setColor(picked);
          onChange?.(picked.toString("hex").toLowerCase());
        }}
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
  disabled = false,
  onChange,
}: {
  label: string;
  value?: string;
  placeholder?: string;
  width?: string;
  disabled?: boolean;
  onChange?: (value: string) => void;
}) {
  return (
    <Input
      aria-label={label}
      size="small"
      isDisabled={disabled}
      defaultValue={value}
      placeholder={placeholder}
      onChange={onChange}
      className={width}
    />
  );
}

/*
 * Action is a secondary button on a row, such as Change or Connect. Rows
 * in dense lists, such as templates, take the extra-small size. Loading
 * shows the work it started is still running.
 */
export function Action({
  children,
  onPress,
  size = "small",
  loading = false,
}: {
  children: string;
  onPress?: () => void;
  size?: "small" | "xs";
  loading?: boolean;
}) {
  return (
    <Button variant="secondary" size={size} loading={loading} onClick={onPress}>
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
