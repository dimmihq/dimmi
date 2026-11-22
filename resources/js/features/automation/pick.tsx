/*
 * The pickers the inspector's rows share. Pick chooses one of a list of
 * named things by its id, the way step settings store it, so two
 * teammates sharing a name are still told apart. MultiPick chooses any
 * number of them and always reads as one line, the way Select's quiet
 * trigger does: the first two names and how many more, opening a
 * searchable Dropdown of checkbox rows.
 */
import { type ComponentType, useState } from "react";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { CheckboxGlyph } from "@/components/base/checkbox/checkbox-glyph";
import {
  Dropdown,
  DropdownDivider,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem, type SelectVariant } from "@/components/base/select/select";
import { ChevronDownSmall } from "@/components/foundations/icons/chevrons";
import { cx } from "@/utils/cx";

import type { Choices } from "./choices";

export function Pick({
  label,
  choices,
  value,
  placeholder = "Pick one",
  variant = "default",
  className,
  onChange,
}: {
  label: string;
  choices: Choices;
  value: string;
  placeholder?: string;
  variant?: SelectVariant;
  /* The trigger's own classes, such as a width that hugs the choice. */
  className?: string;
  onChange: (id: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      variant={variant}
      className={className}
      placeholder={placeholder}
      defaultSelectedKey={value || null}
      onSelectionChange={(key) => onChange(String(key))}
      popoverClassName="w-56"
    >
      {choices.map(([id, name]) => (
        <SelectItem key={id} id={id} textValue={name}>
          {name}
        </SelectItem>
      ))}
    </Select>
  );
}

export function MultiPick({
  label,
  choices,
  value,
  allLabel = "All",
  empty = "None available",
  icons,
  shown = 2,
  className,
  onChange,
}: {
  label: string;
  choices: Choices;
  value: string[];
  allLabel?: string;
  empty?: string;
  icons?: Record<string, ComponentType<{ className?: string }>>;
  /* How many picked names the trigger spells out before it counts the rest. */
  shown?: number;
  /* The trigger's own classes, such as a width that fits more names. */
  className?: string;
  onChange: (ids: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  if (choices.length === 0) {
    return <span className="text-body-2-regular text-text-tertiary">{empty}</span>;
  }

  const byId = new Map(choices);
  /* In the list's order, so the summary does not shuffle as boxes are ticked. */
  const selected = choices.map(([id]) => id).filter((id) => value.includes(id));
  const needle = query.trim().toLowerCase();
  const filtered = needle ? choices.filter(([, name]) => name.toLowerCase().includes(needle)) : choices;

  const toggle = (id: string) =>
    onChange(value.includes(id) ? value.filter((current) => current !== id) : [...value, id]);

  const popover = (
    <DropdownPopover aria-label={label} placement="bottom end" className="w-60">
      {choices.length > 5 ? (
        <div className="pb-1">
          <Input
            aria-label={`Search ${label.toLowerCase()}`}
            size="small"
            fieldClassName="bg-background-secondary-default"
            value={query}
            onChange={setQuery}
            placeholder={`Search ${label.toLowerCase()}…`}
          />
        </div>
      ) : null}
      <div className="flex max-h-56 flex-col gap-0.5 overflow-y-auto">
        {filtered.length === 0 ? (
          <p className="px-2 py-2 text-body-2-regular text-text-tertiary">No matches.</p>
        ) : (
          filtered.map(([id, name]) => {
            const checked = value.includes(id);
            const Icon = icons?.[id];
            return (
              <DropdownItem key={id} onSelect={() => toggle(id)} className="gap-2 px-2 py-1.5">
                <CheckboxGlyph
                  size="sm"
                  state={{
                    isSelected: checked,
                    isIndeterminate: false,
                    isFocusVisible: false,
                    isDisabled: false,
                    isHovered: false,
                  }}
                />
                {Icon ? <Icon aria-hidden className="size-3.5 shrink-0 text-foreground-icon-secondary" /> : null}
                <span className="min-w-0 flex-1 truncate text-start text-body-2-regular">{name}</span>
              </DropdownItem>
            );
          })
        )}
      </div>
      {selected.length > 0 ? (
        <>
          <DropdownDivider />
          <DropdownItem
            onSelect={() => {
              onChange([]);
              setOpen(false);
            }}
            className="px-2 py-1.5 text-body-2-regular text-text-secondary"
          >
            Reset to {allLabel.toLowerCase()}
          </DropdownItem>
        </>
      ) : null}
    </DropdownPopover>
  );

  const names = selected.map((id) => byId.get(id) ?? id);
  const summary = names.length === 0 ? allLabel : names.slice(0, shown).join(", ");
  const more = names.length - shown;
  const Lead = selected.length === 1 ? icons?.[selected[0] ?? ""] : undefined;

  return (
    <Dropdown
      isOpen={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) {
          setQuery("");
        }
      }}
    >
      <DropdownTrigger
        aria-label={`${label}: ${names.length === 0 ? allLabel : names.join(", ")}`}
        className={cx(
          "group/pick -ms-1.5 flex h-7 max-w-full min-w-0 items-center gap-1 rounded-lg px-1.5 text-body-2-regular text-text-primary",
          "transition-colors duration-150 hover:bg-background-secondary-hover aria-expanded:bg-background-secondary-hover",
          "disabled:cursor-not-allowed disabled:text-text-tertiary",
          className,
        )}
      >
        {Lead ? <Lead aria-hidden className="size-3.5 shrink-0 text-foreground-icon-secondary" /> : null}
        <span className="min-w-0 truncate">{summary}</span>
        {more > 0 ? <span className="shrink-0 text-text-tertiary tabular-nums">+{more}</span> : null}
        <ChevronDownSmall
          aria-hidden
          className={cx(
            "size-3.5 shrink-0 text-text-tertiary opacity-0 transition-[opacity,transform] duration-150",
            "group-hover/pick:opacity-100 group-focus-visible/pick:opacity-100",
            open && "rotate-180 opacity-100",
          )}
        />
      </DropdownTrigger>
      {popover}
    </Dropdown>
  );
}

export function Picks({
  choices,
  value,
  empty,
  onChange,
}: {
  choices: Choices;
  value: string[];
  empty: string;
  onChange: (ids: string[]) => void;
}) {
  if (choices.length === 0) {
    return <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">{empty}</p>;
  }

  return (
    <div className="flex flex-col gap-2 px-3 py-2.5">
      {choices.map(([id, name]) => (
        <Checkbox
          key={id}
          size="sm"
          isSelected={value.includes(id)}
          onChange={(on) => onChange(on ? [...value, id] : value.filter((current) => current !== id))}
        >
          {name}
        </Checkbox>
      ))}
    </div>
  );
}
