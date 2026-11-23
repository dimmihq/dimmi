/*
 * A tool chip as the routine writer draws it: the quiet lime pill a
 * procedure step draws for an action it calls.
 */
import { RiFlashlightLine } from "@remixicon/react";
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";

import { cx } from "@/utils/cx";

import { toolLabel } from "./tools";

export function ToolChip({ node, selected }: NodeViewProps) {
  const name = String(node.attrs.name ?? "");

  return (
    <NodeViewWrapper
      as="span"
      data-tool={name}
      title={name}
      className={cx(
        "mx-0.5 inline-flex items-center gap-1 rounded-md bg-status-lime-background px-1.5 align-baseline text-body-2-medium text-status-lime-text",
        selected && "ring-2 ring-border-focus-ring",
      )}
    >
      <RiFlashlightLine aria-hidden className="size-3.5 shrink-0" />
      {toolLabel(name)}
    </NodeViewWrapper>
  );
}
