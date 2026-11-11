/*
 * The series canvas, on React Flow, drawn the way the workflow canvas
 * is: who enters on top, each step under the one before it, and a step
 * with several paths splitting into them, each line carrying the path's
 * name. Where two paths meet again, the second shows a small card that
 * says where it continues. Every path that leads nowhere yet ends in a
 * plus that adds a step there. Cards are secondary cards with a tinted
 * mark; the selected one wears the accent ring and one with something
 * missing a warning line. With results on, every card adds what its
 * step did. Places come from d3's tidy tree, so the series always reads
 * top to bottom.
 */
import { RiAddLine, RiCornerDownRightLine, RiErrorWarningLine, RiFocus3Line, RiSubtractLine } from "@remixicon/react";
import {
  Background,
  BackgroundVariant,
  BaseEdge,
  type Edge,
  EdgeLabelRenderer,
  type EdgeProps,
  Panel as FlowPanel,
  getSmoothStepPath,
  Handle,
  type Node,
  type NodeProps,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import { hierarchy, tree } from "d3-hierarchy";
import { createContext, type RefObject, useContext, useEffect, useMemo, useRef } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { KindMark } from "@/features/automation/parts";
import type { SeriesNode, SeriesProblem, SeriesReport, SeriesSpec } from "@/lib/api/series";
import { formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { nodeLook, percent } from "./looks";
import { isMessage, optionalPaths, pathsOf, summary, targetOf } from "./model";
import { Unfold } from "./motion";

const cardWidth = 264;
const rowHeight = 160;
const columnGap = 48;

type CardNode = Node<{ node: SeriesNode }, "card">;
type AddNode = Node<{ from: string; path: string }, "add">;
type JoinNode = Node<{ to: string }, "join">;
type PathEdge = Edge<{ label?: string }, "path">;

export interface Moves {
  selected: string;
  results: boolean;
  editable: boolean;
  spec: SeriesSpec;
  report?: SeriesReport;
  problems: SeriesProblem[];
  onSelect: (id: string) => void;
  onAdd: (from: string, path: string) => void;
}

const MovesContext = createContext<Moves | null>(null);

function useMoves(): Moves {
  const moves = useContext(MovesContext);
  if (!moves) {
    throw new Error("series cards render only inside the canvas");
  }

  return moves;
}

/*
 * resultLine says in one line what a step did while the series ran.
 */
function resultLine(node: SeriesNode, report?: SeriesReport): string {
  const step = report?.steps[node.id];
  if (!step) {
    return "No one here yet";
  }
  if (node.kind === "entry") {
    return `${formatNumber(step.reached)} entered`;
  }
  if (step.waiting > 0) {
    return `${formatNumber(step.reached)} reached · ${formatNumber(step.waiting)} here now`;
  }
  if (isMessage(node.kind) && step.message) {
    const sent = step.message.sent ?? 0;
    return `${formatNumber(sent)} sent · ${percent(step.message.opened ?? 0, sent)} opened · ${percent(step.message.replied ?? 0, sent)} replied`;
  }
  const paths = Object.entries(step.paths);
  if (paths.length > 1) {
    const labels = new Map(pathsOf(node).map((p) => [p.id, p.label]));
    return paths.map(([id, n]) => `${formatNumber(n)} ${labels.get(id) ?? id}`).join(" · ");
  }

  return `${formatNumber(step.reached)} reached`;
}

function Card({ id, data }: NodeProps<CardNode>) {
  const moves = useMoves();
  const look = nodeLook[data.node.kind];
  const selected = moves.selected === id;
  const missing = moves.problems.some((p) => p.node_id === id);

  return (
    <>
      <Handle type="target" position={Position.Top} isConnectable={false} className="invisible" />
      <button
        type="button"
        aria-pressed={selected}
        aria-label={`${look.name}: ${summary(data.node, moves.spec)}`}
        onClick={() => moves.onSelect(id)}
        className={cx(
          "pointer-events-auto flex w-[264px] cursor-pointer flex-col gap-2 rounded-2xl bg-background-secondary-default p-3 text-start outline-none",
          "transition-[box-shadow,background-color] duration-150 hover:bg-background-secondary-hover",
          "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          selected && "ring-2 ring-accent-500",
        )}
      >
        <span className="flex items-center gap-2">
          <KindMark icon={look.icon} tint={look.tint} size="sm" />
          <span className="flex-1 text-body-2-medium text-text-secondary">{look.name}</span>
          {missing ? (
            <RiErrorWarningLine aria-label="Something is missing" className="size-4 text-text-error-primary" />
          ) : null}
        </span>
        <span className="line-clamp-2 text-body-regular text-text-primary">{summary(data.node, moves.spec)}</span>
        <Unfold shown={moves.results} className="-mt-2">
          <span className="mt-2 block border-t border-separator-border pt-2 text-caption-1-medium text-text-secondary tabular-nums">
            {resultLine(data.node, moves.report)}
          </span>
        </Unfold>
      </button>
      <Handle type="source" position={Position.Bottom} isConnectable={false} className="invisible" />
    </>
  );
}

function Plus({ data }: NodeProps<AddNode>) {
  const moves = useMoves();

  return (
    <>
      <Handle type="target" position={Position.Top} isConnectable={false} className="invisible" />
      <button
        type="button"
        aria-label="Add a step"
        disabled={!moves.editable}
        onClick={() => moves.onAdd(data.from, data.path)}
        className="pointer-events-auto flex size-7 cursor-pointer items-center justify-center rounded-full border border-border-button-default bg-background-primary-default text-foreground-icon-secondary shadow-xs outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-default disabled:opacity-60"
      >
        <RiAddLine aria-hidden className="size-4" />
      </button>
    </>
  );
}

function Join({ data }: NodeProps<JoinNode>) {
  const moves = useMoves();
  const target = moves.spec.nodes.find((n) => n.id === data.to);

  return (
    <>
      <Handle type="target" position={Position.Top} isConnectable={false} className="invisible" />
      <button
        type="button"
        onClick={() => moves.onSelect(data.to)}
        className="pointer-events-auto flex w-[264px] cursor-pointer items-center gap-2 rounded-xl border border-border-button-default bg-background-primary-default px-3 py-2 text-start text-body-2-regular text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiCornerDownRightLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
        <span className="truncate">Continues at {target ? summary(target, moves.spec) : data.to}</span>
      </button>
    </>
  );
}

function Line({ sourceX, sourceY, targetX, targetY, data }: EdgeProps<PathEdge>) {
  const [path] = getSmoothStepPath({
    sourceX,
    sourceY,
    sourcePosition: Position.Bottom,
    targetX,
    targetY,
    targetPosition: Position.Top,
    borderRadius: 8,
    centerY: sourceY + 20,
  });

  return (
    <>
      <BaseEdge path={path} />
      {data?.label ? (
        <EdgeLabelRenderer>
          <span
            style={{ transform: `translate(-50%, -50%) translate(${targetX}px, ${targetY - 26}px)` }}
            className="nodrag nopan pointer-events-none absolute rounded-full bg-background-tertiary-default px-2.5 py-1 text-body-2-medium text-text-secondary"
          >
            {data.label}
          </span>
        </EdgeLabelRenderer>
      ) : null}
    </>
  );
}

const nodeTypes = { card: Card, add: Plus, join: Join };
const edgeTypes = { path: Line };

type Item =
  | { kind: "card"; id: string; node: SeriesNode; label?: string; children: Item[] }
  | { kind: "add"; id: string; from: string; path: string; label?: string; children: Item[] }
  | { kind: "join"; id: string; to: string; label?: string; children: Item[] };

/*
 * itemOf turns the series' graph into the tree d3 lays out, from the
 * step down: each path to a step not drawn yet becomes a child, a path
 * to one drawn already a join card, and a path to nowhere a plus.
 */
function itemOf(spec: SeriesSpec, node: SeriesNode, placed: Set<string>, label?: string): Item {
  placed.add(node.id);
  const children: Item[] = [];
  const optional = optionalPaths(node).filter((p) => targetOf(spec, node.id, p.id));
  for (const p of [...pathsOf(node), ...optional]) {
    const to = targetOf(spec, node.id, p.id);
    const next = spec.nodes.find((n) => n.id === to);
    if (!next) {
      children.push({
        kind: "add",
        id: `add:${node.id}:${p.id}`,
        from: node.id,
        path: p.id,
        label: p.label,
        children: [],
      });
    } else if (placed.has(next.id)) {
      children.push({ kind: "join", id: `join:${node.id}:${p.id}`, to: next.id, label: p.label, children: [] });
    } else {
      children.push(itemOf(spec, next, placed, p.label));
    }
  }

  return { kind: "card", id: node.id, node, label, children };
}

function useGraph(spec: SeriesSpec) {
  return useMemo(() => {
    const entry = spec.nodes.find((n) => n.kind === "entry") ?? { id: "entry", kind: "entry" as const };
    const laid = tree<Item>().nodeSize([cardWidth + columnGap, rowHeight])(hierarchy(itemOf(spec, entry, new Set())));
    const nodes: (CardNode | AddNode | JoinNode)[] = laid.descendants().map((point) => {
      const item = point.data;
      if (item.kind === "card") {
        return {
          id: item.id,
          type: "card",
          position: { x: point.x - cardWidth / 2, y: point.y },
          data: { node: item.node },
        };
      }
      if (item.kind === "join") {
        return {
          id: item.id,
          type: "join",
          position: { x: point.x - cardWidth / 2, y: point.y },
          data: { to: item.to },
        };
      }

      return {
        id: item.id,
        type: "add",
        position: { x: point.x - 14, y: point.y },
        data: { from: item.from, path: item.path },
      };
    });
    const edges: PathEdge[] = laid.links().map((link) => ({
      id: `${link.source.data.id}->${link.target.data.id}`,
      source: link.source.data.id,
      target: link.target.data.id,
      type: "path",
      data: { label: link.target.data.label },
    }));

    return { nodes, edges };
  }, [spec]);
}

function Zoom() {
  const flow = useReactFlow();

  return (
    <div className="flex flex-col gap-1">
      <IconButton size="small" icon={RiAddLine} aria-label="Zoom in" onClick={() => flow.zoomIn({ duration: 200 })} />
      <IconButton
        size="small"
        icon={RiSubtractLine}
        aria-label="Zoom out"
        onClick={() => flow.zoomOut({ duration: 200 })}
      />
      <IconButton
        size="small"
        icon={RiFocus3Line}
        aria-label="Fit to screen"
        onClick={() => flow.fitView({ padding: 0.1, maxZoom: 1, duration: 200 })}
      />
    </div>
  );
}

/*
 * useReveal pans the canvas, at its zoom, until the picked card is fully
 * on screen once the settings pane has finished pushing the canvas
 * aside, and leaves it alone when it already is.
 */
function useReveal(frame: RefObject<HTMLDivElement | null>, selected: string) {
  const flow = useReactFlow();

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const node = flow.getNode(selected);
      const box = frame.current?.getBoundingClientRect();
      if (!node || !box) {
        return;
      }
      const { x, y, zoom } = flow.getViewport();
      const width = node.measured?.width ?? cardWidth;
      const height = node.measured?.height ?? 120;
      const left = node.position.x * zoom + x;
      const top = node.position.y * zoom + y;
      if (left >= 0 && top >= 0 && left + width * zoom <= box.width && top + height * zoom <= box.height) {
        return;
      }
      void flow.setCenter(node.position.x + width / 2, node.position.y + height / 2, { zoom, duration: 300 });
    }, 360);

    return () => window.clearTimeout(timer);
  }, [flow, frame, selected]);
}

/*
 * useFit keeps the whole series in view while the canvas changes size,
 * as it does while the settings pane pushes it aside, until the
 * teammate first pans or zooms it themselves.
 */
function useFit(frame: RefObject<HTMLDivElement | null>) {
  const flow = useReactFlow();
  const moved = useRef(false);

  useEffect(() => {
    const element = frame.current;
    if (!element) {
      return;
    }
    const observer = new ResizeObserver(() => {
      if (!moved.current) {
        void flow.fitView({ padding: 0.15, maxZoom: 0.9 });
      }
    });
    observer.observe(element);

    return () => observer.disconnect();
  }, [flow, frame]);

  return (event: MouseEvent | TouchEvent | null) => {
    if (event) {
      moved.current = true;
    }
  };
}

function Flow({ moves }: { moves: Moves }) {
  const { nodes, edges } = useGraph(moves.spec);
  const frame = useRef<HTMLDivElement>(null);
  const onMoveStart = useFit(frame);
  useReveal(frame, moves.selected);

  return (
    <MovesContext.Provider value={moves}>
      <div ref={frame} className="relative min-h-0 flex-1">
        <ReactFlow
          aria-label="Series canvas"
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          onInit={(flow) => void flow.fitView({ padding: 0.15, maxZoom: 0.9 })}
          onMoveStart={onMoveStart}
          nodesDraggable={false}
          nodesConnectable={false}
          nodesFocusable={false}
          edgesFocusable={false}
          elementsSelectable={false}
          panOnScroll
          zoomOnScroll={false}
          zoomOnDoubleClick={false}
          minZoom={0.2}
          maxZoom={1.5}
        >
          <Background variant={BackgroundVariant.Dots} gap={20} size={1} />
          <FlowPanel position="bottom-left">
            <Zoom />
          </FlowPanel>
        </ReactFlow>
      </div>
    </MovesContext.Provider>
  );
}

export function SeriesCanvas({ moves }: { moves: Moves }) {
  return (
    <ReactFlowProvider>
      <Flow moves={moves} />
    </ReactFlowProvider>
  );
}
