/*
 * Help center collections as a tree: collections at the top, each with
 * sub-collections under it, three levels and no deeper, as the help
 * center allows. Moving one by dragging is planned here: whether a drop is
 * allowed, and the parent and positions it leaves behind. So is which
 * articles a pick of the tree lists.
 */
import type { Article, Collection, CollectionChange } from "@/lib/api/help-centers";

/*
 * positionStep spaces sibling positions, leaving room between them.
 */
const positionStep = 1024;

/*
 * maxDepth is how many levels of collections a help center holds.
 */
export const maxDepth = 3;

export type DropPosition = "before" | "after" | "on";

export interface DropTarget {
  key: string;
  position: DropPosition;
}

/*
 * childrenOf lists the collections under the parent, or at the top for
 * null, in their order.
 */
export function childrenOf(collections: Collection[], parent: string | null): Collection[] {
  return collections
    .filter((c) => c.parent_id === parent)
    .sort((a, b) => a.position - b.position || a.id.localeCompare(b.id));
}

function find(collections: Collection[], id: string): Collection | undefined {
  return collections.find((c) => c.id === id);
}

/*
 * newParentOf is the parent the dragged collection would have after the
 * drop.
 */
function newParentOf(collections: Collection[], target: DropTarget): string | null {
  if (target.position === "on") {
    return target.key;
  }

  return find(collections, target.key)?.parent_id ?? null;
}

/*
 * descendantsOf lists every collection somewhere under the one with id.
 */
export function descendantsOf(collections: Collection[], id: string): string[] {
  return childrenOf(collections, id).flatMap((c) => [c.id, ...descendantsOf(collections, c.id)]);
}

/*
 * depthOf is the level a collection sits at, one for the top.
 */
export function depthOf(collections: Collection[], id: string): number {
  const parent = find(collections, id)?.parent_id ?? null;

  return parent === null ? 1 : 1 + depthOf(collections, parent);
}

/*
 * heightOf is how many levels a collection spans with everything inside
 * it, one when it holds no sub-collections.
 */
function heightOf(collections: Collection[], id: string): number {
  return 1 + Math.max(0, ...childrenOf(collections, id).map((c) => heightOf(collections, c.id)));
}

/*
 * canDrop says whether the drop keeps the tree within maxDepth levels,
 * counting everything the dragged collection carries, and never puts a
 * collection under itself or anything inside it.
 */
export function canDrop(collections: Collection[], dragged: string, target: DropTarget): boolean {
  if (dragged === target.key || !find(collections, target.key) || !find(collections, dragged)) {
    return false;
  }
  const parent = newParentOf(collections, target);
  if (parent === null) {
    return true;
  }
  if (parent === dragged || descendantsOf(collections, dragged).includes(parent)) {
    return false;
  }

  return depthOf(collections, parent) + heightOf(collections, dragged) <= maxDepth;
}

/*
 * planMove lists the changes that make the drop: the dragged collection's
 * new parent, and the positions of every sibling whose place changed.
 */
export function planMove(
  collections: Collection[],
  dragged: string,
  target: DropTarget,
): Array<{ id: string } & CollectionChange> {
  const moving = find(collections, dragged);
  if (!moving || !canDrop(collections, dragged, target)) {
    return [];
  }
  const parent = newParentOf(collections, target);
  const siblings = childrenOf(collections, parent).filter((c) => c.id !== dragged);
  const at = target.position === "on" ? siblings.length : siblings.findIndex((c) => c.id === target.key);
  const order = [...siblings];
  order.splice(target.position === "after" ? at + 1 : at, 0, moving);

  return order.flatMap((c, index) => {
    const position = (index + 1) * positionStep;
    const change: { id: string } & CollectionChange = { id: c.id };
    const reparented = c.id === dragged && c.parent_id !== parent;
    if (reparented) {
      change.parent_id = parent ?? "";
    }
    if (reparented || c.position !== position) {
      change.position = position;
    }

    return Object.keys(change).length > 1 ? [change] : [];
  });
}

/*
 * treeOptions lists every collection in tree order with its depth, from
 * zero at the top, and its path of names, for pickers that show the
 * nesting.
 */
export function treeOptions(
  collections: Collection[],
  parent: string | null = null,
  above: string[] = [],
): Array<{ id: string; name: string; depth: number; path: string }> {
  return childrenOf(collections, parent).flatMap((c) => {
    const names = [...above, c.name];

    return [
      { id: c.id, name: c.name, depth: above.length, path: names.join(" / ") },
      ...treeOptions(collections, c.id, names),
    ];
  });
}

/*
 * allKey and unfiledKey are the picks above the tree: every article, and
 * the articles in no collection.
 */
export const allKey = "all";
export const unfiledKey = "unfiled";

/*
 * articlesIn lists the articles a pick shows: all of them, the unfiled
 * ones, or those in a collection and every collection inside it.
 */
export function articlesIn(pick: string, articles: Article[], collections: Collection[]): Article[] {
  if (pick === allKey) {
    return articles;
  }
  if (pick === unfiledKey) {
    return articles.filter((a) => a.collection_id === null);
  }
  const within = new Set([pick, ...descendantsOf(collections, pick)]);

  return articles.filter((a) => a.collection_id !== null && within.has(a.collection_id));
}
