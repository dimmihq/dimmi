/*
 * A tag as it shows anywhere: its name in the theme's text colour beside
 * a dot of the colour the workspace chose. The colour is the workspace's
 * own data, so it only ever paints the dot and text stays readable in
 * light and dark.
 */
import { RiCloseLine } from "@remixicon/react";

import type { Tag } from "@/lib/api/tags";

export function TagDot({ color }: { color: string | null }) {
  if (!color) {
    return <span aria-hidden className="size-2 shrink-0 rounded-full bg-foreground-icon-quaternary" />;
  }

  return <span aria-hidden className="size-2 shrink-0 rounded-full" style={{ backgroundColor: color }} />;
}

export function TagChip({ tag, onRemove }: { tag: Tag; onRemove?: () => void }) {
  return (
    <span className="flex max-w-full items-center gap-1.5 rounded-full border border-border-button-default bg-background-primary-default py-0.5 ps-2 pe-2 text-body-2-regular text-text-primary">
      <TagDot color={tag.color} />
      <span className="truncate">{tag.name}</span>
      {onRemove ? (
        <button
          type="button"
          aria-label={`Remove ${tag.name}`}
          onClick={onRemove}
          className="-me-1 flex size-4 shrink-0 items-center justify-center rounded-full text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiCloseLine aria-hidden className="size-3" />
        </button>
      ) : null}
    </span>
  );
}
