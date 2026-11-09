/*
 * A table row's actions, behind one outlined icon button at the row's
 * end. Everyday actions come first; one that destroys something sits
 * apart below a divider, in the error tone, so it is never picked by
 * accident.
 */
import { type RemixiconComponentType, RiMore2Fill } from "@remixicon/react";
import { useState } from "react";

import {
  Dropdown,
  DropdownDivider,
  DropdownGroup,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { cx } from "@/utils/cx";

export interface RowAction<K extends string> {
  kind: K;
  label: string;
  icon: RemixiconComponentType;
  danger?: boolean;
}

function Item<K extends string>({ action, onPick }: { action: RowAction<K>; onPick: (kind: K) => void }) {
  const Icon = action.icon;
  const tone = action.danger ? "text-text-error-primary" : "text-foreground-icon-secondary";

  return (
    <DropdownItem onSelect={() => onPick(action.kind)} className="px-2 py-1.5">
      <Icon className={cx("size-[18px] shrink-0", tone)} aria-hidden />
      <span
        className={cx(
          "truncate text-body-medium whitespace-nowrap",
          action.danger ? "text-text-error-primary" : "text-text-primary",
        )}
      >
        {action.label}
      </span>
    </DropdownItem>
  );
}

function outlinedTrigger(open: boolean): string {
  return cx(
    "relative inline-flex size-8 shrink-0 items-center justify-center rounded-2lg",
    "border border-border-button-default bg-background-primary-default text-foreground-icon-primary shadow-xs",
    "transition-[background-color,border-color,box-shadow,color] duration-150 ease",
    "hover:border-border-button-hover hover:bg-background-primary-hover",
    open && "border-border-button-active bg-background-primary-active",
  );
}

/*
 * quietTrigger is the borderless trigger for rows that show their actions
 * only on hover, such as the rows of a tree.
 */
function quietTrigger(open: boolean): string {
  return cx(
    "relative inline-flex size-6 shrink-0 items-center justify-center rounded-md text-foreground-icon-secondary",
    "outline-none transition-colors duration-150 hover:bg-background-tertiary-default hover:text-foreground-icon-primary",
    "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
    open && "bg-background-tertiary-default text-foreground-icon-primary",
  );
}

/*
 * RowMenu draws the actions of the row named label; onPick gets the kind
 * of the action chosen, after the menu has closed. Quiet draws a
 * small borderless trigger for dense rows.
 */
export function RowMenu<K extends string>({
  label,
  actions,
  onPick,
  quiet = false,
}: {
  label: string;
  actions: RowAction<K>[];
  onPick: (kind: K) => void;
  quiet?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const pick = (kind: K) => {
    setOpen(false);
    onPick(kind);
  };
  const everyday = actions.filter((a) => !a.danger);
  const danger = actions.filter((a) => a.danger);

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label={`Actions for ${label}`}
        className={cx(quiet ? quietTrigger(open) : outlinedTrigger(open))}
      >
        <RiMore2Fill className="size-4 shrink-0" aria-hidden />
      </DropdownTrigger>
      <DropdownPopover aria-label={`Actions for ${label}`} placement="bottom end" className="w-[220px] p-2">
        <DropdownGroup>
          {everyday.map((action) => (
            <Item key={action.kind} action={action} onPick={pick} />
          ))}
        </DropdownGroup>
        {danger.length > 0 ? (
          <>
            {everyday.length > 0 ? <DropdownDivider /> : null}
            <DropdownGroup>
              {danger.map((action) => (
                <Item key={action.kind} action={action} onPick={pick} />
              ))}
            </DropdownGroup>
          </>
        ) : null}
      </DropdownPopover>
    </Dropdown>
  );
}
