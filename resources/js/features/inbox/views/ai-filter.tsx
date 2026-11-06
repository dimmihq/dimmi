/*
 * Narrowing a list by the agent's part in each conversation: every
 * conversation, those the agent touched, those it still answers, or
 * those it handed to the team. The choice lives on the address, so the
 * agent's overview can link straight to a narrowed list.
 */
import { RiCheckLine, RiSparkling2Line } from "@remixicon/react";
import { useState } from "react";

import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import type { AiFilter } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { aiFilters } from "./ai-filters";

export function AiFilterMenu({
  value,
  onChange,
}: {
  value: AiFilter | undefined;
  onChange: (next?: AiFilter) => void;
}) {
  const [open, setOpen] = useState(false);
  const label = value ? `Agent filter: ${aiFilters.find((f) => f.value === value)?.label}` : "Filter by agent";

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label={label}
        className={cx(
          "flex size-7 items-center justify-center rounded-lg transition-colors hover:bg-background-secondary-hover",
          value && "bg-background-tertiary-default",
        )}
      >
        <RiSparkling2Line
          aria-hidden
          className={cx("size-4", value ? "text-foreground-icon-primary" : "text-foreground-icon-secondary")}
        />
      </DropdownTrigger>
      <DropdownPopover aria-label="Filter by agent" placement="bottom end" className="w-56">
        {aiFilters.map((filter) => (
          <DropdownItem
            key={filter.label}
            selected={filter.value === value}
            onSelect={() => {
              setOpen(false);
              onChange(filter.value);
            }}
          >
            <span className="flex-1 text-body-2-regular text-text-primary">{filter.label}</span>
            {filter.value === value ? (
              <RiCheckLine aria-hidden className="size-4 text-foreground-icon-secondary" />
            ) : null}
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
  );
}
