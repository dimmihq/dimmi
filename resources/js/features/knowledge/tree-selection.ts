/*
 * What ticking an item in a source's tree means. A ticked item reads
 * everything under it, so its descendants show as ticked without being
 * stored; storing only the outermost ticks keeps a later item added
 * under a ticked one in the reading.
 */
import type { TreeNode } from "@/lib/api/knowledge-sources";

/*
 * inheritedOf lists the items ticked only because an item above them is.
 */
export function inheritedOf(nodes: TreeNode[], selected: string[]): Set<string> {
  const out = new Set<string>();
  const walk = (list: TreeNode[], covered: boolean) => {
    for (const node of list) {
      if (covered) {
        out.add(node.id);
      }
      walk(node.children, covered || selected.includes(node.id));
    }
  };
  walk(nodes, false);

  return out;
}

/*
 * nextSelection turns the ticks the tree now shows into the ticks to
 * store: the items ticked by hand, without those an item above covers.
 */
export function nextSelection(nodes: TreeNode[], selected: string[], shown: Set<string>): string[] {
  const inherited = inheritedOf(nodes, selected);
  const kept = selected.filter((id) => shown.has(id));
  const added = [...shown].filter((id) => !selected.includes(id) && !inherited.has(id));
  const next = [...kept, ...added];
  const covered = inheritedOf(nodes, next);

  return next.filter((id) => !covered.has(id));
}
