/*
 * What the conversation attribute pages share: each type's mark and
 * one-line hint, the types that take options, what each option means,
 * and a row the team drags into a new place.
 */
import { RiDraggable } from "@remixicon/react";
import type { DragEvent, ReactNode } from "react";

import { Input } from "@/components/base/input/input";
import { cx } from "@/utils/cx";

/*
 * OptionDescriptions says what each option means, one line each, for
 * the AI to read when it picks one.
 */
export function OptionDescriptions({
  options,
  value,
  onChange,
}: {
  options: string[];
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
}) {
  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="pb-0.5 text-body-2-medium text-text-primary">What each option means</legend>
      <p className="pb-1 text-caption-1-regular text-text-tertiary">
        Optional. The AI reads these to pick the right one.
      </p>
      {options.map((option) => (
        <div key={option} className="grid grid-cols-1 gap-1 sm:grid-cols-[8rem_minmax(0,1fr)] sm:items-center sm:gap-3">
          <span className="truncate text-body-2-regular text-text-secondary" title={option}>
            {option}
          </span>
          <Input
            aria-label={`What ${option} means`}
            size="small"
            className="min-w-0"
            value={value[option] ?? ""}
            placeholder={`When to pick ${option}`}
            maxLength={300}
            onChange={(text) => onChange({ ...value, [option]: text })}
          />
        </div>
      ))}
    </fieldset>
  );
}

/*
 * rowHover is how a list row lights up under the pointer: the whole
 * row, from its drag handle to its chevron.
 */
const rowHover = "-ms-1.5 ps-1.5 pe-1 rounded-lg transition-colors duration-150 hover:bg-background-secondary-hover";

/*
 * Movable is a row the team drags into a new place by its handle, or
 * moves with the arrow keys on the handle. The whole row follows the
 * pointer, not just the handle. While a row is dragged, a
 * line follows the pointer to show where it lands: above the row under
 * it when moving up, below it when moving down.
 */
export function Movable({
  label,
  movable,
  dragging,
  line,
  onStart,
  onEnd,
  onOver,
  onDrop,
  onStep,
  children,
}: {
  label: string;
  movable: boolean;
  dragging: boolean;
  line: "before" | "after" | null;
  onStart: () => void;
  onEnd: () => void;
  onOver: () => void;
  onDrop: () => void;
  onStep: (by: number) => void;
  children: ReactNode;
}) {
  if (!movable) {
    return <li className={rowHover}>{children}</li>;
  }

  return (
    <li
      className={cx(
        "relative flex items-center gap-1 bg-background-secondary-default",
        rowHover,
        dragging && "opacity-50",
      )}
      onDragOver={(event: DragEvent) => {
        event.preventDefault();
        event.dataTransfer.dropEffect = "move";
        onOver();
      }}
      onDrop={(event: DragEvent) => {
        event.preventDefault();
        onDrop();
      }}
    >
      {line ? (
        <span
          aria-hidden
          className={cx(
            "pointer-events-none absolute inset-x-0 z-10 h-0.5 rounded-full bg-accent-500",
            line === "before" ? "-top-px" : "-bottom-px",
          )}
        />
      ) : null}
      <button
        type="button"
        draggable
        aria-label={`Move ${label}`}
        onDragStart={(event) => {
          event.dataTransfer.effectAllowed = "move";
          event.dataTransfer.setData("text/plain", label);
          const row = event.currentTarget.closest("li");
          if (row) {
            const box = row.getBoundingClientRect();
            event.dataTransfer.setDragImage(row, event.clientX - box.left, event.clientY - box.top);
          }
          onStart();
        }}
        onDragEnd={onEnd}
        onKeyDown={(event) => {
          if (event.key === "ArrowUp" || event.key === "ArrowDown") {
            event.preventDefault();
            onStep(event.key === "ArrowUp" ? -1 : 1);
          }
        }}
        className="flex h-8 w-5 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring active:cursor-grabbing"
      >
        <RiDraggable aria-hidden className="size-4" />
      </button>
      <div className="min-w-0 flex-1">{children}</div>
    </li>
  );
}
