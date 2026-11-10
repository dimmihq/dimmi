/*
 * The series canvas, on React Flow, drawn the way the workflow canvas
 * is: who enters on top, each step under the one before it, and a check
 * splitting into its two paths, each line carrying the path's name.
 * Cards are the same secondary cards with a tinted mark; the selected
 * one wears the accent ring. With results on, every card adds what its
 * step did: how many it was sent to and what they did, or how many are
 * waiting on it now, opening below the card as results turn on. Places come from d3's tidy tree, so the series
 * always reads top to bottom and a path never crosses another.
 */
import { RiAddLine, RiFocus3Line, RiSubtractLine } from "@remixicon/react";
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
  MiniMap,
  type Node,
  type NodeProps,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import { hierarchy, tree } from "d3-hierarchy";
import { createContext, useContext, useMemo, useRef } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { KindMark } from "@/features/automation/parts";
import { useWide } from "@/features/shell/use-wide";
import { formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { nodeLook } from "./looks";
import { Unfold } from "./motion";
import { percent, type SeriesNode } from "./sample";

const cardWidth = 264;
const rowHeight = 160;
const columnGap = 48;

type CardNode = Node<{ node: SeriesNode }, "card">;
type AddNode = Node<{ node?: undefined }, "add">;
type PathEdge = Edge<{ label?: string }, "path">;

interface Moves {
  selected: string;
  results: boolean;
  onSelect: (id: string) => void;
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
function resultLine(node: SeriesNode): string {
  const stats = node.stats;
  if (!stats) {
    return "No one here yet";
  }
  if (node.kind === "entry") {
    return `${formatNumber(stats.reached)} entered`;
  }
  if (node.kind === "wait") {
    return `${formatNumber(stats.waiting ?? 0)} waiting now`;
  }
  if (node.kind === "branch") {
    const [yes, no] = node.next ?? [];
    return `${formatNumber(yes?.stats?.reached ?? 0)} yes · ${formatNumber(no?.stats?.reached ?? 0)} no`;
  }
  if (stats.sent === undefined) {
    return `${formatNumber(stats.reached)} people`;
  }
  const action =
    stats.replied === undefined
      ? `${percent(stats.clicked ?? 0, stats.sent)} clicked`
      : `${percent(stats.replied, stats.sent)} replied`;

  return `${formatNumber(stats.sent)} sent · ${percent(stats.opened ?? 0, stats.sent)} opened · ${action}`;
}

function Card({ id, data }: NodeProps<CardNode>) {
  const moves = useMoves();
  const look = nodeLook[data.node.kind];
  const selected = moves.selected === id;

  return (
    <>
      <Handle type="target" position={Position.Top} isConnectable={false} className="invisible" />
      <button
        type="button"
        aria-pressed={selected}
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
        </span>
        <span className="line-clamp-2 text-body-regular text-text-primary">{data.node.summary}</span>
        <Unfold shown={moves.results} className="-mt-2">
          <span className="mt-2 block border-t border-separator-border pt-2 text-caption-1-medium text-text-secondary tabular-nums">
            {resultLine(data.node)}
          </span>
        </Unfold>
      </button>
      <Handle type="source" position={Position.Bottom} isConnectable={false} className="invisible" />
    </>
  );
}

function Plus() {
  return (
    <>
      <Handle type="target" position={Position.Top} isConnectable={false} className="invisible" />
      <span
        aria-hidden
        className="pointer-events-auto flex size-7 items-center justify-center rounded-full border border-border-button-default bg-background-primary-default text-foreground-icon-secondary shadow-xs"
      >
        <RiAddLine className="size-4" />
      </span>
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

const nodeTypes = { card: Card, add: Plus };
const edgeTypes = { path: Line };

type Item = { node?: SeriesNode; id: string; children: Item[] };

/*
 * itemOf turns the series into the tree d3 lays out, ending every path
 * that does not leave the series with the add-step plus.
 */
function itemOf(node: SeriesNode): Item {
  const next = node.next ?? [];
  const children =
    next.length > 0 ? next.map(itemOf) : node.kind === "exit" ? [] : [{ id: `add:${node.id}`, children: [] }];

  return { node, id: node.id, children };
}

function useGraph(root: SeriesNode) {
  return useMemo(() => {
    const laid = tree<Item>().nodeSize([cardWidth + columnGap, rowHeight])(hierarchy(itemOf(root)));
    const nodes: (CardNode | AddNode)[] = laid.descendants().map((point) =>
      point.data.node
        ? {
            id: point.data.id,
            type: "card",
            position: { x: point.x - cardWidth / 2, y: point.y },
            data: { node: point.data.node },
          }
        : { id: point.data.id, type: "add", position: { x: point.x - 14, y: point.y }, data: {} },
    );
    const edges: PathEdge[] = laid.links().map((link) => ({
      id: `${link.source.data.id}->${link.target.data.id}`,
      source: link.source.data.id,
      target: link.target.data.id,
      type: "path",
      data: { label: link.target.data.node?.label },
    }));

    return { nodes, edges };
  }, [root]);
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

function Flow({ root, moves }: { root: SeriesNode; moves: Moves }) {
  const { nodes, edges } = useGraph(root);
  const overview = useWide(768);
  const frame = useRef<HTMLDivElement>(null);

  return (
    <MovesContext.Provider value={moves}>
      <div ref={frame} className="relative min-h-0 flex-1">
        <ReactFlow
          aria-label="Series canvas"
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          onInit={(flow) => flow.setViewport({ x: (frame.current?.clientWidth ?? 0) / 2, y: 32, zoom: 0.9 })}
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
          {overview ? <MiniMap pannable zoomable ariaLabel="Series overview" className="rounded-2xl" /> : null}
        </ReactFlow>
      </div>
    </MovesContext.Provider>
  );
}

export function SeriesCanvas({ root, moves }: { root: SeriesNode; moves: Moves }) {
  return (
    <ReactFlowProvider>
      <Flow root={root} moves={moves} />
    </ReactFlowProvider>
  );
}
