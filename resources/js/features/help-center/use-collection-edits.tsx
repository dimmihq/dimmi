/*
 * Everything that changes a help center's collections from the tree:
 * dragging to reorder or nest, naming a new collection or renaming one
 * in its row, and deleting one. Each change saves at once and the tree
 * reloads, so what a teammate sees is always what is stored. Drops that
 * would nest too deep are refused before they land.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { DropIndicator, type Key, useDragAndDrop } from "react-aria-components";

import { toast } from "@/components/base/toast/toast";
import {
  type Collection,
  collectionsQuery,
  createCollection,
  deleteCollection,
  updateCollection,
} from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { canDrop, type DropTarget, planMove } from "./collection-tree";

/*
 * Editing is the row being named: an existing collection renamed, or a
 * new one under parent, or at the top for null.
 */
export type Editing = { kind: "rename"; id: string } | { kind: "new"; parent: string | null };

export interface CollectionEdits {
  editing: Editing | null;
  setEditing: (editing: Editing | null) => void;
  expanded: Set<Key>;
  setExpanded: (keys: Set<Key>) => void;
  onName: (name: string) => Promise<unknown>;
  startNew: (parent: string | null) => void;
  remove: (collection: Collection) => Promise<unknown>;
  dragAndDropHooks: ReturnType<typeof useDragAndDrop>["dragAndDropHooks"];
}

function useRefresh(workspace: string, helpCenter: string) {
  const client = useQueryClient();

  return () => client.invalidateQueries({ queryKey: collectionsQuery(workspace, helpCenter).queryKey });
}

/*
 * useMoves lets the tree's rows be dragged, refuses drops that would
 * nest too deep, and saves the order a drop leaves.
 */
function useMoves(workspace: string, collections: Collection[], refresh: () => Promise<unknown>) {
  const dragging = useRef<string | null>(null);
  const move = useMutation({
    mutationFn: async (target: DropTarget & { dragged: string }) => {
      for (const { id, ...change } of planMove(collections, target.dragged, target)) {
        await updateCollection(workspace, id, change);
      }
    },
    onError: (error) => toast.error(error.message),
    onSettled: refresh,
  });

  return useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ "text/plain": String(key) })),
    onDragStart: (event) => {
      dragging.current = String([...event.keys][0] ?? "");
    },
    onDragEnd: () => {
      dragging.current = null;
    },
    getDropOperation: (target) => {
      if (target.type !== "item" || !dragging.current) {
        return "cancel";
      }
      const drop = { key: String(target.key), position: target.dropPosition };

      return canDrop(collections, dragging.current, drop) ? "move" : "cancel";
    },
    onMove: (event) => {
      const dragged = String([...event.keys][0] ?? "");
      if (event.target.type === "item" && dragged) {
        move.mutate({ dragged, key: String(event.target.key), position: event.target.dropPosition });
      }
    },
    renderDropIndicator: (target) => (
      <DropIndicator
        target={target}
        className={({ isDropTarget }) =>
          cx("mx-2 h-0.5 rounded-full", isDropTarget ? "bg-accent-500" : "bg-transparent")
        }
      />
    ),
  });
}

/*
 * useCollectionEdits keeps the tree's open rows and the row being named,
 * and saves every change. onCreated hears of each new collection, so the
 * page can open it.
 */
export function useCollectionEdits({
  workspace,
  helpCenter,
  collections,
  onCreated,
}: {
  workspace: string;
  helpCenter: string;
  collections: Collection[];
  onCreated: (collection: Collection) => void;
}): CollectionEdits {
  const refresh = useRefresh(workspace, helpCenter);
  const [editing, setEditing] = useState<Editing | null>(null);
  const [collapsed, setCollapsed] = useState<Set<Key>>(new Set());
  const { dragAndDropHooks } = useMoves(workspace, collections, refresh);
  const onName = async (name: string) => {
    if (!editing) {
      return;
    }
    try {
      if (editing.kind === "rename") {
        await updateCollection(workspace, editing.id, { name });
        await refresh();
      } else {
        const made = await createCollection(workspace, helpCenter, name, editing.parent ?? undefined);
        await refresh();
        onCreated(made);
      }
      setEditing(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Not saved.");
    }
  };

  return {
    editing,
    setEditing,
    expanded: new Set(collections.filter((c) => !collapsed.has(c.id)).map((c) => c.id)),
    setExpanded: (keys) => setCollapsed(new Set(collections.filter((c) => !keys.has(c.id)).map((c) => c.id))),
    onName,
    startNew: (parent) => {
      if (parent) {
        setCollapsed((before) => new Set([...before].filter((key) => key !== parent)));
      }
      setEditing({ kind: "new", parent });
    },
    remove: async (collection) => {
      try {
        await deleteCollection(workspace, collection.id);
        await refresh();
      } catch (error) {
        toast.error(error instanceof Error ? error.message : "Not deleted.");
        throw error;
      }
    },
    dragAndDropHooks,
  };
}
