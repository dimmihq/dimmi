/*
 * The priority of a conversation: a chip that says it, and a menu that
 * changes it. P opens the menu from anywhere in the open conversation and
 * the digits 1 to 5 pick a level while it is open, none first. A level of
 * none shows no chip on a list row, since most conversations have none.
 */
import { RiFlag2Line } from "@remixicon/react";
import { useEffect, useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Kbd } from "@/components/base/kbd/kbd";
import { type ConversationPriority, conversationPriorities } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { isTyping } from "../keyboard/typing";
import { priorityLabel } from "./labels";

const priorityColor: Record<Exclude<ConversationPriority, "none">, ChipColor> = {
  low: "soft",
  medium: "yellow",
  high: "orange",
  urgent: "rose",
};

/*
 * PriorityChip names the level, and shows nothing for none.
 */
export function PriorityChip({ priority }: { priority: ConversationPriority }) {
  if (priority === "none") {
    return null;
  }

  return (
    <Chip variant="caption" color={priorityColor[priority]} className="py-0">
      {priorityLabel[priority]}
    </Chip>
  );
}

/*
 * usePriorityKeys opens the menu on P, and picks a level by its digit
 * while the menu is open.
 */
function usePriorityKeys(open: boolean, setOpen: (open: boolean) => void, pick: (p: ConversationPriority) => void) {
  useEffect(() => {
    const press = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) {
        return;
      }
      if (open) {
        const level = conversationPriorities[Number(event.key) - 1];
        if (level) {
          event.preventDefault();
          pick(level);
          setOpen(false);
        }

        return;
      }
      if (event.key.toLowerCase() === "p" && !isTyping(event.target)) {
        event.preventDefault();
        setOpen(true);
      }
    };
    document.addEventListener("keydown", press);

    return () => document.removeEventListener("keydown", press);
  }, [open, setOpen, pick]);
}

export function PriorityMenu({
  priority,
  onChange,
}: {
  priority: ConversationPriority;
  onChange: (priority: ConversationPriority) => void;
}) {
  const [open, setOpen] = useState(false);
  usePriorityKeys(open, setOpen, onChange);

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label={`Priority: ${priorityLabel[priority]}`}
        className={cx(
          "flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-2 text-body-2-medium text-text-secondary",
          "transition-colors duration-150 hover:bg-background-secondary-hover",
        )}
      >
        <RiFlag2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
        {priority === "none" ? null : <PriorityChip priority={priority} />}
      </DropdownTrigger>
      <DropdownPopover aria-label="Set priority" placement="bottom end" className="w-52">
        {conversationPriorities.map((level, index) => (
          <DropdownItem
            key={level}
            selected={level === priority}
            onSelect={() => {
              onChange(level);
              setOpen(false);
            }}
          >
            <span className="min-w-0 flex-1 truncate text-start">{priorityLabel[level]}</span>
            <Kbd>{index + 1}</Kbd>
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
  );
}
