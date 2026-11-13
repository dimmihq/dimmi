/*
 * The text controls of the agent's settings. What the person types goes
 * into the page's draft as they type, and waits for the save bar like
 * every other setting on the page; nothing saves when a field loses
 * focus.
 */
import { type ReactNode, useEffect, useState } from "react";

import { Input } from "@/components/base/input/input";
import { Textarea } from "@/components/base/textarea/textarea";

/*
 * DraftText is a one-line text setting, showing the draft's value.
 */
export function DraftText({
  label,
  value,
  placeholder,
  disabled,
  maxLength,
  width = "w-full sm:w-64",
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  disabled: boolean;
  maxLength?: number;
  width?: string;
  onChange: (value: string) => void;
}) {
  return (
    <Input
      aria-label={label}
      size="small"
      value={value}
      placeholder={placeholder}
      maxLength={maxLength}
      isDisabled={disabled}
      onChange={onChange}
      className={width}
    />
  );
}

/*
 * same leaves text as it is, for a draft that keeps it as typed.
 */
const same = (text: string) => text;

/*
 * DraftTextarea is a longer text setting with a count of what is left.
 * When the draft keeps a tidied form of the text, such as a list read
 * from its lines, normalize gives that form: the field keeps what was
 * typed while it still reads the same, and takes the draft's value once
 * it differs, as after Discard.
 */
export function DraftTextarea({
  label,
  value,
  placeholder,
  disabled,
  maxLength,
  hint,
  normalize = same,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  disabled: boolean;
  maxLength: number;
  hint?: string;
  normalize?: (text: string) => string;
  onChange: (value: string) => void;
}) {
  const [text, setText] = useState(value);
  useEffect(() => {
    setText((typed) => (normalize(typed) === value ? typed : value));
  }, [value, normalize]);

  return (
    <Textarea
      aria-label={label}
      value={text}
      placeholder={placeholder}
      isDisabled={disabled}
      maxLength={maxLength}
      showCount
      hint={hint}
      rows={4}
      autoResize
      maxRows={12}
      onChange={(next) => {
        setText(next);
        onChange(next);
      }}
    />
  );
}

/*
 * StackedRow is a setting whose control needs the card's full width,
 * such as a long text or a set of choices, placed under its label.
 */
export function StackedRow({
  label,
  description,
  children,
}: {
  label: string;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 border-separator-border border-b py-3 pe-3 last:border-b-0">
      <div className="flex min-w-0 flex-col">
        <span className="text-body-regular text-text-primary">{label}</span>
        {description ? <span className="text-body-2-regular text-text-secondary">{description}</span> : null}
      </div>
      {children}
    </div>
  );
}
