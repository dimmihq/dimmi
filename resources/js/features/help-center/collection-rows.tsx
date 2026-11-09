/*
 * The rows of the collections tree, as easy to work as pages in a
 * sidebar. Each row shows how many articles it holds; hovering it trades
 * the count for a plus that adds a collection inside and a menu to
 * rename or delete. New and renamed collections are named right in the
 * row. The plus and the menu's "inside" choice only appear where one
 * more level still fits.
 */
import {
  RiAddLine,
  RiDeleteBinLine,
  RiEditLine,
  RiFolder3Line,
  RiFolderAddLine,
  RiFolderOpenLine,
} from "@remixicon/react";
import { Button as AriaButton, type TreeItemRenderProps } from "react-aria-components";

import { TreeViewItem } from "@/components/base/tree-view/tree-view";
import { type RowAction, RowMenu } from "@/features/shell/row-menu";
import type { Article, Collection } from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { articlesIn, childrenOf, depthOf, maxDepth } from "./collection-tree";
import { InlineName } from "./inline-name";
import type { Editing } from "./use-collection-edits";

export type RowAct = "rename" | "child" | "delete";

const draftKey = "new-collection";

export interface RowProps {
  collections: Collection[];
  articles: Article[];
  editing: Editing | null;
  onAct: (act: RowAct, collection: Collection) => void;
  onName: (name: string) => Promise<unknown>;
  onCancel: () => void;
}

/*
 * rowClass draws a row picked, so the collection being listed is plain
 * to see, and outlines one a drag would drop into.
 */
function rowClass({ isSelected, isDropTarget }: TreeItemRenderProps): string {
  return cx(
    "group/row",
    isSelected &&
      "bg-background-primary-default text-text-primary shadow-xs ring-1 ring-inset ring-border-button-default",
    isDropTarget && "ring-2 ring-inset ring-accent-500",
  );
}

function actionsFor(roomInside: boolean): RowAction<RowAct>[] {
  return [
    { kind: "rename", label: "Rename", icon: RiEditLine },
    ...(roomInside ? [{ kind: "child" as const, label: "Add a collection inside", icon: RiFolderAddLine }] : []),
    { kind: "delete", label: "Delete", icon: RiDeleteBinLine, danger: true },
  ];
}

/*
 * DraftRow is the row a new collection is named in, under its parent or
 * at the end of the top level.
 */
export function DraftRow({ onName, onCancel }: Pick<RowProps, "onName" | "onCancel">) {
  return (
    <TreeViewItem
      id={draftKey}
      label="New collection"
      icon={RiFolder3Line}
      labelContent={<InlineName initial="" label="New collection name" onCommit={onName} onCancel={onCancel} />}
    />
  );
}

/*
 * RowTools shows a row's article count, traded on hover or focus for its
 * plus and its menu, or while the keyboard is on the row. The menu stays
 * shown while it is open.
 */
function RowTools({
  collection,
  count,
  roomInside,
  onAct,
}: {
  collection: Collection;
  count: number;
  roomInside: boolean;
  onAct: RowProps["onAct"];
}) {
  return (
    <span className="grid items-center *:col-start-1 *:row-start-1">
      <span
        className={cx(
          "justify-self-end tabular-nums transition-opacity group-hover/row:opacity-0",
          "group-data-[focus-visible]/row:opacity-0 group-has-[[aria-expanded=true]]/row:opacity-0 pointer-coarse:opacity-0",
        )}
      >
        {count > 0 ? count : null}
      </span>
      <span
        className={cx(
          "flex items-center justify-end gap-0.5 opacity-0 transition-opacity has-[[aria-expanded=true]]:opacity-100",
          "group-hover/row:opacity-100 group-data-[focus-visible]/row:opacity-100 pointer-coarse:opacity-100",
        )}
      >
        {roomInside ? (
          <AriaButton
            aria-label={`Add a collection inside ${collection.name}`}
            onPress={() => onAct("child", collection)}
            className={cx(
              "inline-flex size-6 items-center justify-center rounded-md text-foreground-icon-secondary outline-none",
              "transition-colors hover:bg-background-tertiary-default hover:text-foreground-icon-primary",
              "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
            )}
          >
            <RiAddLine aria-hidden className="size-4" />
          </AriaButton>
        ) : null}
        <RowMenu
          quiet
          label={collection.name}
          actions={actionsFor(roomInside)}
          onPick={(act) => onAct(act, collection)}
        />
      </span>
    </span>
  );
}

export function CollectionRow({ collection, ...props }: RowProps & { collection: Collection }) {
  const { collections, articles, editing, onAct, onName, onCancel } = props;
  const renaming = editing?.kind === "rename" && editing.id === collection.id;
  const adding = editing?.kind === "new" && editing.parent === collection.id;
  const roomInside = depthOf(collections, collection.id) < maxDepth;

  return (
    <TreeViewItem
      id={collection.id}
      label={collection.name}
      icon={RiFolder3Line}
      expandedIcon={RiFolderOpenLine}
      className={rowClass}
      labelContent={
        renaming ? (
          <InlineName initial={collection.name} label="Collection name" onCommit={onName} onCancel={onCancel} />
        ) : undefined
      }
      trailingContent={
        renaming ? undefined : (
          <RowTools
            collection={collection}
            count={articlesIn(collection.id, articles, collections).length}
            roomInside={roomInside}
            onAct={onAct}
          />
        )
      }
    >
      {childrenOf(collections, collection.id).map((child) => (
        <CollectionRow key={child.id} collection={child} {...props} />
      ))}
      {adding ? <DraftRow onName={onName} onCancel={onCancel} /> : null}
    </TreeViewItem>
  );
}
