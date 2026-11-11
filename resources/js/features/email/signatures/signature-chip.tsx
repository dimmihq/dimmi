/*
 * A signature detail as the writer draws it: a quiet chip with the
 * detail's name. Pressing it says what fills it in, and that a line
 * whose detail is empty is left out of the reply rather than sent with
 * a gap, which is how a signature stands in for a fallback.
 */
import { RiBracesLine } from "@remixicon/react";
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";

import { Popover, PopoverHeader, PopoverTrigger } from "@/components/base/popover/popover";
import { cx } from "@/utils/cx";

import { signatureVariableOf } from "./signature-variables";

export function SignatureChip({ node, editor, selected }: NodeViewProps) {
  const key = node.attrs.key as string;
  const variable = signatureVariableOf(key);
  const label = variable?.label ?? key;
  const chip = cx(
    "mx-px inline-flex items-center gap-0.5 rounded-md bg-status-blue-background px-1 align-baseline text-status-blue-text",
    "outline-none",
    selected && "ring-2 ring-border-focus-ring",
  );

  return (
    <NodeViewWrapper as="span" data-signature-variable={key} className="inline">
      {editor.isEditable ? (
        <PopoverTrigger>
          <button type="button" contentEditable={false} className={cx(chip, "cursor-pointer")}>
            <RiBracesLine aria-hidden className="size-3 shrink-0" />
            {label}
          </button>
          <Popover placement="bottom start" aria-label={`${label} variable`} className="w-72">
            <PopoverHeader title={label} description={variable?.description ?? key} />
            <p className="text-body-2-regular text-text-secondary">
              When a reply has nothing to fill it with, the line it sits on is left out.
            </p>
          </Popover>
        </PopoverTrigger>
      ) : (
        <span className={chip}>{label}</span>
      )}
    </NodeViewWrapper>
  );
}
