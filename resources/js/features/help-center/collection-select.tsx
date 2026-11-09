/*
 * Picking an article's collection from the collections as they nest:
 * each one indented to its level, the picked one named by its whole path
 * so a sub-collection is never mistaken for a namesake elsewhere, and a
 * choice of no collection at all. The menu opens as wide as its field.
 */
import { RiCornerDownRightLine, RiFolder3Line, RiFolderForbidLine } from "@remixicon/react";

import { Select, SelectItem } from "@/components/base/select/select";
import type { Collection } from "@/lib/api/help-centers";

import { collectionOptions } from "./docs-format";

const noCollection = "none";

/*
 * CollectionSelect picks value among collections; an empty value is no
 * collection, and onChange hears the same.
 */
export function CollectionSelect({
  collections,
  value,
  onChange,
}: {
  collections: Collection[];
  value: string;
  onChange: (collection: string) => void;
}) {
  const options = collectionOptions(collections);

  return (
    <Select
      label="Collection"
      className="w-full"
      popoverClassName="w-(--trigger-width)"
      selectedKey={value || noCollection}
      renderValue={options.find((c) => c.id === value)?.label ?? "No collection"}
      onSelectionChange={(key) => onChange(key === noCollection || key === null ? "" : String(key))}
    >
      {options.map((c) => {
        const nested = c.depth > 0;
        const Icon = c.id === "" ? RiFolderForbidLine : nested ? RiCornerDownRightLine : RiFolder3Line;

        return (
          <SelectItem key={c.id || noCollection} id={c.id || noCollection} textValue={c.label}>
            <span className="flex min-w-0 items-center gap-2" style={{ paddingInlineStart: `${c.depth * 1.25}rem` }}>
              <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
              <span className="truncate">{nested ? c.label.split(" / ").at(-1) : c.label}</span>
            </span>
          </SelectItem>
        );
      })}
    </Select>
  );
}
