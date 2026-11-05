/*
 * A tag as it shows anywhere: its icon and name on a tint of the color
 * the workspace chose, shaped like BoardUI's tags. The color is anything
 * a person picks, so the text color is worked out from it for light and
 * dark mode and always reads; a tag without a color keeps the neutral
 * chip.
 */
import { RiCloseLine } from "@remixicon/react";

import type { Tag } from "@/lib/api/tags";
import { cx } from "@/utils/cx";
import { tagTint } from "./tag-colors";
import { tagIcon } from "./tag-icons";

type Tinted = Pick<Tag, "color" | "icon">;

/*
 * TagIcon is the tag's icon in its readable color, for places that list
 * tags beside other text, such as the picker and the settings list.
 */
export function TagIcon({ tag, className }: { tag: Tinted; className?: string }) {
  const Icon = tagIcon(tag.icon);

  return (
    <span
      aria-hidden
      style={tagTint(tag.color).style}
      className={cx(
        "flex size-5 shrink-0 items-center justify-center rounded-md",
        tagTint(tag.color).className,
        className,
      )}
    >
      <Icon className="size-3.5" />
    </span>
  );
}

export type TagChipSize = "medium" | "small";

const chipSizes = {
  medium: { chip: "h-7 px-2 text-body-medium", removable: "pe-1", icon: "size-4", remove: "size-5", x: "size-3.5" },
  small: {
    chip: "h-6 px-1.5 text-caption-1-medium",
    removable: "pe-0.5",
    icon: "size-3.5",
    remove: "size-4",
    x: "size-3",
  },
};

/*
 * TagChip shows a tag at BoardUI's regular tag size, or small in dense
 * lists such as table rows.
 */
export function TagChip({ tag, onRemove, size = "medium" }: { tag: Tag; onRemove?: () => void; size?: TagChipSize }) {
  const Icon = tagIcon(tag.icon);
  const sized = chipSizes[size];
  const tint = tagTint(tag.color);

  return (
    <span
      style={tint.style}
      className={cx(
        "inline-flex max-w-full items-center gap-1 rounded-lg",
        sized.chip,
        tint.className,
        onRemove && sized.removable,
      )}
    >
      <Icon aria-hidden className={cx("shrink-0", sized.icon)} />
      <span className="truncate">{tag.name}</span>
      {onRemove ? (
        <button
          type="button"
          aria-label={`Remove ${tag.name}`}
          onClick={onRemove}
          className={cx(
            "flex shrink-0 cursor-pointer items-center justify-center rounded-md opacity-60 outline-none",
            "hover:bg-current/15 hover:opacity-100 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
            sized.remove,
          )}
        >
          <RiCloseLine aria-hidden className={sized.x} />
        </button>
      ) : null}
    </span>
  );
}
