/*
 * The people list's column chooser: which columns follow the person,
 * saved with the teammate's inbox preferences.
 */
import { RiLayoutColumnLine } from "@remixicon/react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";

import { choices, usePeopleColumns } from "./people-columns";

export function ColumnsChooser() {
  const { chosen, toggle } = usePeopleColumns();

  return (
    <PopoverTrigger>
      <IconButton icon={RiLayoutColumnLine} aria-label="Columns" />
      <Popover aria-label="Columns" placement="bottom end" className="w-56">
        <div className="flex flex-col gap-2 p-1">
          {choices.map((column) => (
            <Checkbox
              key={column.id}
              size="sm"
              isSelected={chosen.includes(column.id)}
              onChange={(on) => toggle(column.id, on)}
            >
              {column.label}
            </Checkbox>
          ))}
        </div>
      </Popover>
    </PopoverTrigger>
  );
}
