/*
 * The snooze picker: the choices the conversation's team or workspace
 * offers, each with the moment it ends in the reader's time zone, and a
 * date and time of the reader's own. Z opens it from anywhere in the
 * open conversation.
 */
import { type CalendarDate, getLocalTimeZone, Time, today } from "@internationalized/date";
import { RiAlarmLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { DatePicker } from "@/components/base/date-picker/date-picker";
import {
  Dropdown,
  DropdownDivider,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { TimeField } from "@/components/base/time-field/time-field";
import { snoozeChoicesQuery } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { untilLabel, untilOf } from "./until";

/*
 * clockOf writes a time of day as HH:MM.
 */
function clockOf(time: Time): string {
  return `${String(time.hour).padStart(2, "0")}:${String(time.minute).padStart(2, "0")}`;
}

export function SnoozePicker({
  workspace,
  conversation,
  isOpen,
  onOpenChange,
  onOption,
  onUntil,
}: {
  workspace: string;
  conversation: string;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onOption: (key: string) => void;
  onUntil: (iso: string) => void;
}) {
  const { data: choices = [] } = useQuery({ ...snoozeChoicesQuery(workspace, conversation), enabled: isOpen });
  const [day, setDay] = useState<CalendarDate | null>(today(getLocalTimeZone()).add({ days: 1 }));
  const [time, setTime] = useState<Time | null>(new Time(9, 0));
  const custom = day && time ? untilOf(day, clockOf(time)) : null;
  const pick = (run: () => void) => {
    run();
    onOpenChange(false);
  };

  return (
    <Dropdown isOpen={isOpen} onOpenChange={onOpenChange}>
      <DropdownTrigger
        aria-label="Snooze"
        className={cx(
          "flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-2 text-body-2-medium text-text-secondary",
          "transition-colors duration-150 hover:bg-background-secondary-hover",
        )}
      >
        <RiAlarmLine aria-hidden className="size-4 text-foreground-icon-secondary" />
      </DropdownTrigger>
      <DropdownPopover aria-label="Snooze until" placement="bottom end" className="w-72">
        {choices.map((choice) => (
          <DropdownItem key={choice.key} onSelect={() => pick(() => onOption(choice.key))}>
            <span className="min-w-0 flex-1 truncate text-start">{choice.label}</span>
            <span className="shrink-0 text-caption-1-medium text-text-tertiary">{untilLabel(choice.until)}</span>
          </DropdownItem>
        ))}
        <DropdownDivider />
        <div className="flex flex-col gap-2 p-1">
          <span className="text-caption-1-medium text-text-tertiary">Pick date and time</span>
          <div className="flex items-center gap-2">
            <DatePicker aria-label="Snooze date" value={day} onChange={setDay} />
            <TimeField aria-label="Snooze time" size="small" value={time} onChange={setTime} className="w-32" />
          </div>
          <Button size="small" disabled={custom === null} onClick={() => custom && pick(() => onUntil(custom))}>
            Snooze
          </Button>
        </div>
      </DropdownPopover>
    </Dropdown>
  );
}
