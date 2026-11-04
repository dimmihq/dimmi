/*
 * The other half of the split Send button: send and close, or send and
 * snooze by the conversation's first snooze choice, each with its
 * shortcut. Either moves on to the next conversation.
 */
import { RiArrowDownSLine } from "@remixicon/react";
import { useState } from "react";

import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Kbd } from "@/components/base/kbd/kbd";

export function SendMenu({ disabled, onSend }: { disabled: boolean; onSend: (then: "close" | "snooze") => void }) {
  const [open, setOpen] = useState(false);
  const pick = (then: "close" | "snooze") => {
    setOpen(false);
    onSend(then);
  };

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label="More ways to send"
        isDisabled={disabled}
        className="flex size-9 shrink-0 items-center justify-center rounded-full text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover disabled:cursor-not-allowed disabled:opacity-40"
      >
        <RiArrowDownSLine aria-hidden className="size-5" />
      </DropdownTrigger>
      <DropdownPopover aria-label="Send options" placement="top end" className="w-64">
        <DropdownItem onSelect={() => pick("close")}>
          <span className="min-w-0 flex-1 text-start">Send and close</span>
          <span className="flex gap-1">
            <Kbd>⌘</Kbd>
            <Kbd>Shift</Kbd>
            <Kbd>↵</Kbd>
          </span>
        </DropdownItem>
        <DropdownItem onSelect={() => pick("snooze")}>
          <span className="min-w-0 flex-1 text-start">Send and snooze</span>
          <span className="flex gap-1">
            <Kbd>⌘</Kbd>
            <Kbd>Alt</Kbd>
            <Kbd>↵</Kbd>
          </span>
        </DropdownItem>
      </DropdownPopover>
    </Dropdown>
  );
}
