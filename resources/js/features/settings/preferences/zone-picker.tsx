/*
 * A searchable list of time zones. Typing narrows the list; picking calls
 * back at once, so the row that holds it needs no save button.
 */
import { RiArrowDownSLine } from "@remixicon/react";
import { Button as AriaButton, ComboBox, ListBox, Popover, useFilter } from "react-aria-components";

import { InputBase } from "@/components/base/input/input";
import { SelectItem } from "@/components/base/select/select";
import { cx } from "@/utils/cx";

export interface ZoneChoice {
  id: string;
  label: string;
}

export function ZonePicker({
  label,
  choices,
  selected,
  onPick,
}: {
  label: string;
  choices: ZoneChoice[];
  selected: string;
  onPick: (id: string) => void;
}) {
  const { contains } = useFilter({ sensitivity: "base" });

  return (
    <ComboBox
      aria-label={label}
      className="w-64"
      key={selected}
      defaultItems={choices}
      defaultSelectedKey={selected}
      defaultFilter={contains}
      menuTrigger="focus"
      onSelectionChange={(key) => {
        if (key !== null && key !== selected) {
          onPick(String(key));
        }
      }}
    >
      <InputBase
        size="small"
        trailingAddon={
          <AriaButton
            aria-label={`Show ${label.toLowerCase()}s`}
            className="flex items-center text-foreground-icon-tertiary"
          >
            <RiArrowDownSLine className="size-4" />
          </AriaButton>
        }
      />
      <Popover
        offset={4}
        className={cx(
          "w-64 overflow-hidden rounded-xl border border-border-button-default",
          "bg-background-primary-default p-2 shadow-lg",
        )}
      >
        <ListBox<ZoneChoice> className="flex max-h-60 flex-col gap-1 overflow-auto outline-none">
          {(choice) => (
            <SelectItem id={choice.id} textValue={choice.label}>
              {choice.label}
            </SelectItem>
          )}
        </ListBox>
      </Popover>
    </ComboBox>
  );
}
