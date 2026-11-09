/*
 * A collection's name typed straight into its row, the way a sidebar
 * renames a page: the words start selected, Enter or leaving the field
 * keeps them, Escape puts things back. The field keeps its keys and
 * presses to itself, so typing never moves the tree's focus, expands a
 * row or starts a drag.
 */
import { type KeyboardEvent, type SyntheticEvent, useRef, useState } from "react";
import { Input as AriaInput } from "react-aria-components";

import { cx } from "@/utils/cx";

function keep(event: SyntheticEvent) {
  event.stopPropagation();
}

/*
 * InlineName edits a name in place. onCommit receives the trimmed name
 * when it is not empty and differs from initial; anything else cancels.
 * The field stays open, busy, until the commit settles.
 */
export function InlineName({
  initial,
  label,
  onCommit,
  onCancel,
}: {
  initial: string;
  label: string;
  onCommit: (name: string) => Promise<unknown>;
  onCancel: () => void;
}) {
  const [name, setName] = useState(initial);
  const [busy, setBusy] = useState(false);
  const closed = useRef(false);
  const cancel = () => {
    closed.current = true;
    onCancel();
  };
  const finish = () => {
    const trimmed = name.trim();
    if (busy || closed.current) {
      return;
    }
    if (trimmed === "" || trimmed === initial) {
      cancel();
      return;
    }
    setBusy(true);
    onCommit(trimmed).finally(() => setBusy(false));
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    event.stopPropagation();
    if (event.key === "Enter") {
      event.preventDefault();
      finish();
    }
    if (event.key === "Escape") {
      event.preventDefault();
      cancel();
    }
  };

  return (
    <AriaInput
      aria-label={label}
      value={name}
      maxLength={80}
      disabled={busy}
      placeholder="Untitled"
      autoFocus
      onFocus={(event) => event.currentTarget.select()}
      onChange={(event) => setName(event.target.value)}
      onKeyDown={onKeyDown}
      onKeyUp={keep}
      onPointerDown={keep}
      onMouseDown={keep}
      onClick={keep}
      onBlur={finish}
      className={cx(
        "h-7 min-w-0 flex-1 rounded-md border border-border-button-default bg-background-primary-default px-2",
        "text-body-regular text-text-primary outline-none placeholder:text-text-placeholder",
        "focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:opacity-60",
      )}
    />
  );
}
