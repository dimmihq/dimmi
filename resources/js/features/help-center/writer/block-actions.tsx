/*
 * The small bar on a selected embed or button: change it, or remove it.
 */
import { RiDeleteBinLine, RiPencilLine } from "@remixicon/react";

import { cx } from "@/utils/cx";

const actionClass = cx(
  "flex size-8 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none",
  "transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
);

export function BlockActions({
  label,
  editLabel,
  onEdit,
  onRemove,
}: {
  label: string;
  editLabel: string;
  onEdit: () => void;
  onRemove: () => void;
}) {
  return (
    <div
      role="toolbar"
      aria-label={label}
      className={cx(
        "absolute -top-11 end-0 z-10 flex items-center gap-0.5 rounded-xl border border-border-button-default",
        "bg-background-primary-default p-1 shadow-dropdown motion-safe:animate-menu-in",
      )}
    >
      <button
        type="button"
        aria-label={editLabel}
        onMouseDown={(event) => event.preventDefault()}
        onClick={onEdit}
        className={actionClass}
      >
        <RiPencilLine aria-hidden className="size-4" />
      </button>
      <button
        type="button"
        aria-label={`Remove ${label.toLowerCase()}`}
        onMouseDown={(event) => event.preventDefault()}
        onClick={onRemove}
        className={actionClass}
      >
        <RiDeleteBinLine aria-hidden className="size-4" />
      </button>
    </div>
  );
}
