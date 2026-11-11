/*
 * A variable as the writer draws it: a quiet chip with the detail's
 * name. Pressing it opens a small form for the fallback, the words
 * people without that detail get instead, so a greeting never reads
 * "Hi ,".
 */
import { RiBracesLine } from "@remixicon/react";
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";

import { Input } from "@/components/base/input/input";
import { Popover, PopoverHeader, PopoverTrigger } from "@/components/base/popover/popover";
import { cx } from "@/utils/cx";

import { variableOf } from "./variables";

export function VariableView({ node, updateAttributes, editor, selected }: NodeViewProps) {
  const key = node.attrs.key as string;
  const fallback = node.attrs.fallback as string;
  const label = variableOf(key)?.label ?? key;
  const chip = cx(
    "mx-px inline-flex items-center gap-0.5 rounded-md bg-status-blue-background px-1 align-baseline text-status-blue-text",
    "outline-none",
    selected && "ring-2 ring-border-focus-ring",
  );

  return (
    <NodeViewWrapper as="span" data-variable={key} className="inline">
      {editor.isEditable ? (
        <PopoverTrigger>
          <button type="button" contentEditable={false} className={cx(chip, "cursor-pointer")}>
            <RiBracesLine aria-hidden className="size-3 shrink-0" />
            {label}
          </button>
          <Popover placement="bottom start" aria-label={`${label} variable`} className="w-72">
            <PopoverHeader title={label} description={`Filled in for each person from ${key}.`} />
            <Input
              label="If they don't have one"
              placeholder="Leave it out"
              fieldClassName="bg-background-secondary-default"
              defaultValue={fallback}
              onChange={(next) => updateAttributes({ fallback: next })}
            />
          </Popover>
        </PopoverTrigger>
      ) : (
        <span className={chip}>{label}</span>
      )}
    </NodeViewWrapper>
  );
}
