/*
 * Where every card of a workflow sits on the canvas. A workflow is a
 * tree that only runs down: the trigger on top, each step under the one
 * before it, and a step with paths splitting into columns, one per path,
 * in the order the paths are listed. Every path ends with the add-step
 * plus. The columns come from d3's tidy tree, which keeps siblings in
 * order and packs subtrees as tight as their widths allow; the rows come
 * from the measured height of each card, so long summaries push what
 * follows down instead of overlapping it.
 */
import { type HierarchyPointNode, hierarchy, tree } from "d3-hierarchy";

import { type Step, triggerStep } from "./workflows";

export const cardWidth = 264;
export const cardHeight = 76;
export const addSize = 28;

/*
 * The vertical room between a card and the next one, and between a
 * split and the first card of each path, which also holds the path's
 * name. A label slot is wider than the plus so a path's name never
 * touches its neighbour's.
 */
export const stepGap = 20;
export const pathGap = 88;
const labelSlot = 160;
const columnGap = 32;

export type FlowItem =
  | { kind: "trigger"; id: string; label?: string; children: FlowItem[] }
  | { kind: "step"; id: string; step: Step; label?: string; children: FlowItem[] }
  | { kind: "add"; id: string; path: string | null; label?: string; children: FlowItem[] };

export interface Placed {
  item: FlowItem;
  x: number;
  y: number;
}

export interface Link {
  id: string;
  source: string;
  target: string;
  label?: string;
}

function chain(steps: Step[], index: number, path: string | null, label?: string): FlowItem {
  const step = steps[index];
  if (!step) {
    return { kind: "add", id: `add:${path ?? "root"}`, path, label, children: [] };
  }

  const next = step.paths
    ? step.paths.map((branch) => chain(branch.steps, 0, branch.id, branch.label))
    : [chain(steps, index + 1, path)];

  return { kind: "step", id: step.id, step, label, children: next };
}

function widthOf(item: FlowItem): number {
  if (item.kind !== "add") {
    return cardWidth;
  }

  return item.label ? labelSlot : addSize;
}

function heightOf(item: FlowItem, heights: ReadonlyMap<string, number>): number {
  return heights.get(item.id) ?? (item.kind === "add" ? addSize : cardHeight);
}

/*
 * stack sets each card's top from its parent's bottom, parents first, so
 * the rows follow the real card heights rather than d3's equal depths.
 */
function stack(root: HierarchyPointNode<FlowItem>, heights: ReadonlyMap<string, number>) {
  root.eachBefore((node) => {
    const parent = node.parent;
    node.y = parent ? parent.y + heightOf(parent.data, heights) + (node.data.label ? pathGap : stepGap) : 0;
  });
}

/*
 * layoutWorkflow places the trigger, the steps and the add-step pluses,
 * and lists the lines between them. Cards come out path by path, top to
 * bottom, which is also the order the keyboard walks them in. Positions
 * are the top-left corners React Flow expects, with the trigger centred
 * on x = 0. Heights not yet measured fall back to a one-line card.
 */
export function layoutWorkflow(
  steps: Step[],
  heights: ReadonlyMap<string, number>,
): { placed: Placed[]; links: Link[] } {
  const root = tree<FlowItem>()
    .nodeSize([1, 1])
    .separation((a, b) => (widthOf(a.data) + widthOf(b.data)) / 2 + columnGap)(
    hierarchy<FlowItem>({ kind: "trigger", id: triggerStep, children: [chain(steps, 0, null)] }),
  );
  stack(root, heights);

  const order: HierarchyPointNode<FlowItem>[] = [];
  root.eachBefore((node) => {
    order.push(node);
  });

  return {
    placed: order.map((node) => ({
      item: node.data,
      x: node.x - (node.data.kind === "add" ? addSize : cardWidth) / 2,
      y: node.y,
    })),
    links: root.links().map(({ source, target }) => ({
      id: `${source.data.id}>${target.data.id}`,
      source: source.data.id,
      target: target.data.id,
      label: target.data.label,
    })),
  };
}
