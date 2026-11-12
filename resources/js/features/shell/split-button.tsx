/*
 * A primary button with a second half that opens more ways to finish:
 * the main action on the left, the others behind the chevron, fused into
 * one control the way BoardUI fuses a button group. The menu's rows say
 * what each does, and a row that cannot run yet says why.
 */
import { RiArrowDownSLine } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { cx } from "@/utils/cx";

/*
 * SplitChoice is one row of the menu: what it is called, a line on what
 * it does or why it is not available, and what it runs.
 */
export interface SplitChoice {
  label: string;
  detail?: string;
  disabled?: boolean;
  onSelect: () => void;
}

export function SplitButton({
  label,
  menuLabel,
  choices,
  disabled = false,
  loading = false,
  onPress,
}: {
  label: string;
  menuLabel: string;
  choices: SplitChoice[];
  disabled?: boolean;
  loading?: boolean;
  onPress: () => void;
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="flex items-stretch">
      <Button onClick={onPress} disabled={disabled} loading={loading} className="rounded-e-none">
        {label}
      </Button>
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label={menuLabel}
          className={cx(
            "flex h-9 items-center rounded-e-2lg border-s border-border-button-white bg-button-primary px-2 text-text-white shadow-xs",
            "button-press-motion focus-visible:ring-offset-2",
          )}
        >
          <RiArrowDownSLine aria-hidden className="size-5" />
        </DropdownTrigger>
        <DropdownPopover aria-label={menuLabel} placement="top end" className="w-64 p-1">
          {choices.map((choice) => (
            <DropdownItem
              key={choice.label}
              onSelect={() => {
                if (choice.disabled) {
                  return;
                }
                setOpen(false);
                choice.onSelect();
              }}
              className={cx("flex-col items-start gap-0.5 px-2 py-1.5", choice.disabled && "cursor-not-allowed")}
            >
              <span className={cx("text-body-medium", choice.disabled ? "text-text-tertiary" : "text-text-primary")}>
                {choice.label}
              </span>
              {choice.detail ? <span className="text-body-2-regular text-text-tertiary">{choice.detail}</span> : null}
            </DropdownItem>
          ))}
        </DropdownPopover>
      </Dropdown>
    </div>
  );
}
