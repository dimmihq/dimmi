/*
 * A tag as it shows anywhere: its icon and name on a tint of the color
 * the workspace chose, shaped like BoardUI's tags. The color is anything
 * a person picks, so the text color is worked out from it for light and
 * dark mode and always reads; a tag without a color keeps the neutral
 * chip.
 */
import { RiCloseLine } from "@remixicon/react";
import type { CSSProperties } from "react";

import type { Tag } from "@/lib/api/tags";
import { cx } from "@/utils/cx";
import { tagColors } from "./tag-colors";
import { tagIcon } from "./tag-icons";

type Tinted = Pick<Tag, "color" | "icon">;

function tintStyle(color: string | null): CSSProperties | undefined {
  const colors = tagColors(color);
  if (!colors) {
    return undefined;
  }

  return {
    "--tag-light-background": colors.light.background,
    "--tag-light-text": colors.light.text,
    "--tag-dark-background": colors.dark.background,
    "--tag-dark-text": colors.dark.text,
  } as CSSProperties;
}

const tinted =
  "bg-(--tag-light-background) text-(--tag-light-text) dark:bg-(--tag-dark-background) dark:text-(--tag-dark-text)";
const neutral = "bg-background-tertiary-default text-text-primary";

/*
 * TagIcon is the tag's icon in its readable color, for places that list
 * tags beside other text, such as the picker and the settings list.
 */
export function TagIcon({ tag, className }: { tag: Tinted; className?: string }) {
  const Icon = tagIcon(tag.icon);

  return (
    <span
      aria-hidden
      style={tintStyle(tag.color)}
      className={cx(
        "flex size-5 shrink-0 items-center justify-center rounded-md",
        tag.color ? tinted : neutral,
        className,
      )}
    >
      <Icon className="size-3.5" />
    </span>
  );
}

export function TagChip({ tag, onRemove }: { tag: Tag; onRemove?: () => void }) {
  const Icon = tagIcon(tag.icon);

  return (
    <span
      style={tintStyle(tag.color)}
      className={cx(
        "inline-flex h-6 max-w-full items-center gap-1 rounded-lg px-1.5 text-caption-1-medium",
        tag.color ? tinted : neutral,
        onRemove && "pe-0.5",
      )}
    >
      <Icon aria-hidden className="size-3.5 shrink-0" />
      <span className="truncate">{tag.name}</span>
      {onRemove ? (
        <button
          type="button"
          aria-label={`Remove ${tag.name}`}
          onClick={onRemove}
          className="flex size-4 shrink-0 cursor-pointer items-center justify-center rounded-md opacity-60 outline-none hover:bg-current/15 hover:opacity-100 focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiCloseLine aria-hidden className="size-3" />
        </button>
      ) : null}
    </span>
  );
}
