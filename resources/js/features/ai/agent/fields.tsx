/*
 * The text controls of the agent's settings. Text saves once the person
 * is done with it, when the field loses focus or Enter is pressed, never
 * on every keystroke, and only when it changed. Escape puts the saved
 * value back.
 */
import { type KeyboardEvent, type ReactNode, useEffect, useState } from "react";

import { Input } from "@/components/base/input/input";
import { Textarea } from "@/components/base/textarea/textarea";

/*
 * useDraft keeps what the person types apart from the saved value, and
 * follows the saved value whenever it changes underneath.
 */
function useDraft(saved: string) {
  const [draft, setDraft] = useState(saved);
  useEffect(() => setDraft(saved), [saved]);

  return [draft, setDraft] as const;
}

/*
 * SavedText is a one-line text setting.
 */
export function SavedText({
  label,
  value,
  placeholder,
  disabled,
  maxLength,
  width = "w-full sm:w-64",
  onSave,
}: {
  label: string;
  value: string;
  placeholder?: string;
  disabled: boolean;
  maxLength?: number;
  width?: string;
  onSave: (value: string) => void;
}) {
  const [draft, setDraft] = useDraft(value);
  const commit = () => {
    if (draft.trim() !== value) {
      onSave(draft.trim());
    }
  };
  const keys = (event: KeyboardEvent) => {
    if (event.key === "Enter") {
      commit();
    }
    if (event.key === "Escape") {
      setDraft(value);
    }
  };

  return (
    <Input
      aria-label={label}
      size="small"
      value={draft}
      placeholder={placeholder}
      maxLength={maxLength}
      isDisabled={disabled}
      onChange={setDraft}
      onBlur={commit}
      onKeyDown={keys}
      className={width}
    />
  );
}

/*
 * SavedTextarea is a longer text setting with a count of what is left.
 */
export function SavedTextarea({
  label,
  value,
  placeholder,
  disabled,
  maxLength,
  hint,
  onSave,
}: {
  label: string;
  value: string;
  placeholder?: string;
  disabled: boolean;
  maxLength: number;
  hint?: string;
  onSave: (value: string) => void;
}) {
  const [draft, setDraft] = useDraft(value);

  return (
    <Textarea
      aria-label={label}
      value={draft}
      placeholder={placeholder}
      isDisabled={disabled}
      maxLength={maxLength}
      showCount
      hint={hint}
      rows={4}
      autoResize
      maxRows={12}
      onChange={setDraft}
      onBlur={() => {
        if (draft !== value) {
          onSave(draft);
        }
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
