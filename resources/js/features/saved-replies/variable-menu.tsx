/*
 * The Variable menu of an editor's toolbar: the customer details a
 * message fills in, each inserted in Liquid's form with its fallback.
 * An editor with details of its own, such as a WhatsApp template's
 * custom values, lists them under the customer's and can offer a way to
 * add another.
 */
import { RiAddLine, RiBracesLine } from "@remixicon/react";
import { useState } from "react";

import {
  Dropdown,
  DropdownDivider,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { variableMarkdown, variables } from "@/features/broadcasts/writer/variables";

import { contextValues } from "./render";

const known = Object.keys(contextValues({}));
const offered = variables.filter((variable) => known.includes(variable.key));

export function VariableMenu({
  onPick,
  extra = [],
  onAdd,
}: {
  onPick: (markdown: string) => void;
  extra?: { key: string; label: string }[];
  onAdd?: () => void;
}) {
  const [open, setOpen] = useState(false);
  const pick = (markdown: string) => {
    onPick(markdown);
    setOpen(false);
  };

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger className="flex h-[30px] shrink-0 cursor-pointer items-center gap-1 rounded-full ps-2 pe-2.5 text-body-2-medium text-text-secondary outline-none transition-colors duration-150 ease hover:bg-background-secondary-hover hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring">
        <RiBracesLine aria-hidden className="size-4" />
        Variable
      </DropdownTrigger>
      <DropdownPopover aria-label="Insert a variable" placement="bottom end">
        {offered.map((variable) => (
          <DropdownItem key={variable.key} onSelect={() => pick(variableMarkdown(variable.key, variable.fallback))}>
            {variable.label}
          </DropdownItem>
        ))}
        {extra.map((variable) => (
          <DropdownItem key={variable.key} onSelect={() => pick(variableMarkdown(variable.key, ""))}>
            {variable.label}
          </DropdownItem>
        ))}
        {onAdd ? (
          <>
            <DropdownDivider />
            <DropdownItem
              onSelect={() => {
                setOpen(false);
                onAdd();
              }}
            >
              <RiAddLine aria-hidden className="size-4 text-foreground-icon-secondary" />
              Custom value…
            </DropdownItem>
          </>
        ) : null}
      </DropdownPopover>
    </Dropdown>
  );
}
