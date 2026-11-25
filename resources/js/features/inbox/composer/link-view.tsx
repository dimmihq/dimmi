/*
 * A help center link as the writer draws it: a quiet chip with what it
 * points at and its title, its address in the tooltip.
 */
import { RiArticleLine, RiHistoryLine, RiLightbulbLine } from "@remixicon/react";
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";

import { cx } from "@/utils/cx";

import { kindOf } from "./link-token";

const icons = { article: RiArticleLine, request: RiLightbulbLine, changelog: RiHistoryLine } as const;

export function LinkView({ node, selected }: NodeViewProps) {
  const url = node.attrs.url as string;
  const title = node.attrs.title as string;
  const Icon = icons[kindOf(url)];

  return (
    <NodeViewWrapper as="span" data-link-token={url} className="inline">
      <span
        contentEditable={false}
        title={url}
        className={cx(
          "mx-px inline-flex max-w-full items-center gap-1 rounded-md bg-background-tertiary-default px-1.5 align-baseline text-text-primary",
          selected && "ring-2 ring-border-focus-ring",
        )}
      >
        <Icon aria-hidden className="size-3.5 shrink-0 text-foreground-icon-secondary" />
        <span className="truncate">{title}</span>
      </span>
    </NodeViewWrapper>
  );
}
