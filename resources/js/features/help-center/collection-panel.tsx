/*
 * The help center's collections as a tree in their own pane beside the
 * articles, worked like the pages of a sidebar. Picking a collection
 * lists its articles, and those of every collection inside it; All
 * articles and the articles in no collection sit above the tree. The
 * tree is also where collections are arranged: drag a row to reorder it
 * or onto another to nest it, up to three levels; hover a row to add a
 * collection inside or to rename or delete it; the plus in the header
 * adds one at the top. Names are typed right in their row.
 */
import { type RemixiconComponentType, RiAddLine, RiArticleLine, RiFolderForbidLine } from "@remixicon/react";
import { useState } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { TreeView } from "@/components/base/tree-view/tree-view";
import { Panel, ScrollArea } from "@/features/shell/page";
import type { Article, Collection } from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { CollectionRow, DraftRow, type RowAct, type RowProps } from "./collection-rows";
import { allKey, articlesIn, childrenOf, descendantsOf, unfiledKey } from "./collection-tree";
import { useCollectionEdits } from "./use-collection-edits";

/*
 * PickRow is one of the picks above the tree, drawn like a tree row.
 */
function PickRow({
  label,
  icon: Icon,
  count,
  current,
  onPress,
}: {
  label: string;
  icon: RemixiconComponentType;
  count: number;
  current: boolean;
  onPress: () => void;
}) {
  return (
    <button
      type="button"
      aria-current={current ? "page" : undefined}
      onClick={onPress}
      className={cx(
        "flex min-h-8 w-full items-center gap-2 rounded-xl ps-3 pe-2 text-text-primary outline-none",
        "transition-[background-color,box-shadow] duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        current
          ? "bg-background-primary-default text-body-2-medium shadow-xs ring-1 ring-inset ring-border-button-default"
          : "text-body-2-regular hover:bg-background-secondary-hover",
      )}
    >
      <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary opacity-50" />
      <span className="min-w-0 flex-1 truncate text-start">{label}</span>
      {count > 0 ? <span className="text-caption-1-regular text-text-tertiary tabular-nums">{count}</span> : null}
    </button>
  );
}

export function CollectionPanel({
  workspace,
  helpCenter,
  title,
  collections,
  articles,
  pick,
  onPick,
}: {
  workspace: string;
  helpCenter: string;
  title: string;
  collections: Collection[];
  articles: Article[];
  pick: string;
  onPick: (pick: string) => void;
}) {
  const edits = useCollectionEdits({ workspace, helpCenter, collections, onCreated: (made) => onPick(made.id) });
  const [deleting, setDeleting] = useState<Collection | null>(null);
  const unfiled = articlesIn(unfiledKey, articles, collections).length;
  const onAct = (act: RowAct, collection: Collection) => {
    if (act === "delete") {
      setDeleting(collection);
      return;
    }
    if (act === "child") {
      edits.startNew(collection.id);
      return;
    }
    edits.setEditing({ kind: "rename", id: collection.id });
  };
  const rowProps: RowProps = {
    collections,
    articles,
    editing: edits.editing,
    onAct,
    onName: edits.onName,
    onCancel: () => edits.setEditing(null),
  };
  const addingTop = edits.editing?.kind === "new" && edits.editing.parent === null;
  const empty = collections.length === 0 && !addingTop;

  return (
    <Panel label="Collections" className="h-full w-full">
      <header className="flex h-14 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5">
        <h2 className="truncate text-body-medium text-text-primary">{title}</h2>
        <IconButton
          size="small"
          icon={RiAddLine}
          aria-label="New collection"
          onClick={() => edits.startNew(null)}
          disabled={addingTop}
        />
      </header>
      <ScrollArea>
        <div className="flex flex-col gap-0.5 px-2 pb-4">
          <PickRow
            label="All articles"
            icon={RiArticleLine}
            count={articles.length}
            current={pick === allKey}
            onPress={() => onPick(allKey)}
          />
          {unfiled > 0 ? (
            <PickRow
              label="No collection"
              icon={RiFolderForbidLine}
              count={unfiled}
              current={pick === unfiledKey}
              onPress={() => onPick(unfiledKey)}
            />
          ) : null}
          <div aria-hidden className="mx-3 my-2 h-px bg-separator-border" />
          {empty ? (
            <p className="px-3 py-2 text-body-2-regular text-text-tertiary">
              No collections yet. Add one with the plus above.
            </p>
          ) : (
            <TreeView
              aria-label="Collections"
              size="sm"
              selectionMode="single"
              selectedKeys={pick === allKey || pick === unfiledKey ? [] : [pick]}
              onSelectionChange={(keys) => {
                const [key] = keys === "all" ? [] : [...keys];
                if (key !== undefined && key !== "new-collection") {
                  onPick(String(key));
                }
              }}
              expandedKeys={edits.expanded}
              onExpandedChange={edits.setExpanded}
              dragAndDropHooks={edits.dragAndDropHooks}
              className="border-transparent bg-transparent p-0"
            >
              {childrenOf(collections, null).map((top) => (
                <CollectionRow key={top.id} collection={top} {...rowProps} />
              ))}
              {addingTop ? <DraftRow onName={rowProps.onName} onCancel={rowProps.onCancel} /> : null}
            </TreeView>
          )}
        </div>
      </ScrollArea>
      <AlertDialog
        isOpen={deleting !== null}
        onOpenChange={(open) => (open ? undefined : setDeleting(null))}
        title={deleting ? `Delete ${deleting.name}?` : "Delete collection?"}
        description="Its articles stay, without a collection. A collection with collections inside cannot be deleted until they are moved."
        confirmLabel="Delete collection"
        onConfirm={async () => {
          if (!deleting) {
            return;
          }
          const gone = [deleting.id, ...descendantsOf(collections, deleting.id)];
          await edits.remove(deleting);
          if (gone.includes(pick)) {
            onPick(allKey);
          }
        }}
      />
    </Panel>
  );
}
