/*
 * The options of a choice, edited in place: one row per option with a
 * handle to drag it into a new spot, its text, and a button to remove
 * it, then a button to add one. Enter in an option adds the next one
 * below it; Backspace in an empty option removes it. The handle also
 * moves its option with the arrow keys. An option typed twice is marked;
 * blank options are dropped when the form saves.
 */
import { RiAddLine, RiCloseLine, RiDraggable } from "@remixicon/react";
import { type DragEvent, type KeyboardEvent, useEffect, useRef, useState } from "react";

import { LinkButton } from "@/components/base/buttons/link-button";
import { Input } from "@/components/base/input/input";
import { cx } from "@/utils/cx";

interface Row {
  id: number;
  text: string;
}

/*
 * moveRow moves the row at one place to another.
 */
function moveRow(rows: Row[], from: number, to: number): Row[] {
  if (from === to || to < 0 || to >= rows.length) {
    return rows;
  }
  const next = [...rows];
  const [row] = next.splice(from, 1);
  next.splice(to, 0, row);

  return next;
}

export function OptionsEditor({
  label = "Options",
  value,
  onChange,
  isDisabled = false,
}: {
  label?: string;
  value: string[];
  onChange: (options: string[]) => void;
  isDisabled?: boolean;
}) {
  const nextId = useRef(0);
  const make = (text: string): Row => {
    nextId.current += 1;
    return { id: nextId.current, text };
  };
  const [rows, setRows] = useState<Row[]>(() => (value.length > 0 ? value : [""]).map(make));
  const [focus, setFocus] = useState<number | null>(null);
  const [dragging, setDragging] = useState<number | null>(null);
  const [over, setOver] = useState<number | null>(null);
  const inputs = useRef(new Map<number, HTMLInputElement>());

  useEffect(() => {
    if (focus !== null) {
      inputs.current.get(focus)?.focus();
      setFocus(null);
    }
  }, [focus]);

  const update = (next: Row[]) => {
    setRows(next);
    onChange(next.map((r) => r.text));
  };
  const addAfter = (index: number) => {
    const row = make("");
    update([...rows.slice(0, index + 1), row, ...rows.slice(index + 1)]);
    setFocus(row.id);
  };
  const remove = (index: number) => {
    const next = rows.filter((_, i) => i !== index);
    update(next.length > 0 ? next : [make("")]);
    const before = next[Math.max(0, index - 1)];
    if (before) {
      setFocus(before.id);
    }
  };
  const keyed = (index: number) => (event: KeyboardEvent) => {
    if (event.key === "Enter") {
      event.preventDefault();
      addAfter(index);
    } else if (event.key === "Backspace" && rows[index].text === "" && rows.length > 1) {
      event.preventDefault();
      remove(index);
    }
  };
  const seen = (text: string, index: number) =>
    text.trim() !== "" && rows.some((r, i) => i < index && r.text.trim().toLowerCase() === text.trim().toLowerCase());
  const drop = (index: number) => (event: DragEvent) => {
    event.preventDefault();
    if (dragging !== null) {
      update(moveRow(rows, dragging, index));
    }
    setDragging(null);
    setOver(null);
  };

  return (
    <fieldset className="flex flex-col gap-1.5" disabled={isDisabled}>
      <legend className="pb-1.5 text-body-2-medium text-text-primary">{label}</legend>
      <ul className="flex flex-col gap-1.5">
        {rows.map((row, index) => (
          <li
            key={row.id}
            onDragOver={(event) => {
              event.preventDefault();
              setOver(index);
            }}
            onDrop={drop(index)}
            className={cx(
              "flex items-start gap-1.5 rounded-lg",
              dragging === index && "opacity-50",
              over === index && dragging !== null && dragging !== index && "ring-2 ring-accent-500",
            )}
          >
            <button
              type="button"
              draggable={!isDisabled}
              aria-label={`Move ${row.text || "option"}`}
              onDragStart={(event) => {
                event.dataTransfer.effectAllowed = "move";
                event.dataTransfer.setData("text/plain", String(index));
                setDragging(index);
              }}
              onDragEnd={() => {
                setDragging(null);
                setOver(null);
              }}
              onKeyDown={(event) => {
                if (event.key === "ArrowUp" || event.key === "ArrowDown") {
                  event.preventDefault();
                  update(moveRow(rows, index, index + (event.key === "ArrowUp" ? -1 : 1)));
                }
              }}
              className="mt-2 flex size-6 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring active:cursor-grabbing"
            >
              <RiDraggable aria-hidden className="size-4" />
            </button>
            <Input
              aria-label={`Option ${index + 1}`}
              size="small"
              className="min-w-0 flex-1"
              value={row.text}
              placeholder={`Option ${index + 1}`}
              maxLength={100}
              isDisabled={isDisabled}
              isInvalid={seen(row.text, index)}
              errorMessage="Already an option"
              onChange={(text) => update(rows.map((r) => (r.id === row.id ? { ...r, text } : r)))}
              onKeyDown={keyed(index)}
              ref={(el: HTMLInputElement | null) => {
                if (el) {
                  inputs.current.set(row.id, el);
                } else {
                  inputs.current.delete(row.id);
                }
              }}
            />
            <button
              type="button"
              aria-label={`Remove ${row.text || "option"}`}
              disabled={isDisabled || (rows.length === 1 && row.text === "")}
              onClick={() => remove(index)}
              className="mt-1.5 flex size-7 shrink-0 items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover hover:text-foreground-icon-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:opacity-40 disabled:hover:bg-transparent"
            >
              <RiCloseLine aria-hidden className="size-4" />
            </button>
          </li>
        ))}
      </ul>
      {isDisabled ? null : (
        <div>
          <LinkButton
            size="small"
            variant="secondary"
            leadingIcon={RiAddLine}
            onClick={() => addAfter(rows.length - 1)}
          >
            Add option
          </LinkButton>
        </div>
      )}
    </fieldset>
  );
}
