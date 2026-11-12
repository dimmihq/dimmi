/*
 * The parts every step's settings editor shares: the props the inspector
 * hands it, a box for words such as a message, and a line of note.
 */
import { Textarea } from "@/components/base/textarea/textarea";

import type { Lists } from "./step-fields";
import { unsetSummary } from "./step-settings";
import type { TriggerKind } from "./triggers";
import type { Step } from "./workflows";

export type Edit = (edit: (step: Step) => Step) => void;

export interface EditorProps {
  step: Step;
  trigger: TriggerKind;
  lists: Lists;
  onEdit: Edit;
}

export function Words({
  label,
  value,
  placeholder = unsetSummary,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="py-2.5 pe-2.5">
      <Textarea
        aria-label={label}
        defaultValue={value}
        placeholder={placeholder}
        onChange={onChange}
        rows={3}
        autoResize
        maxRows={8}
      />
    </div>
  );
}

/*
 * Note is a line of explanation inside a card, where no row fits.
 */
export function Note({ children }: { children: string }) {
  return <p className="py-2.5 pe-2.5 text-body-2-regular text-text-secondary">{children}</p>;
}
