/*
 * The workflow canvas, on React Flow: the trigger card on top and the
 * steps below it, joined by thin lines. A step with paths splits into
 * columns, each line labelled with its path's name, and every path ends
 * with the add-step plus. The canvas pans by dragging or scrolling and
 * zooms by pinching or from the controls, so a tree of any width stays
 * reachable on a phone; the minimap gives the overview on larger
 * screens. Cards cannot be dragged: their places come from the layout,
 * so the tree always reads top to bottom. Whatever is picked, added or
 * focused with the keyboard is brought into view. Cards sit on the
 * ground surface as BoardUI secondary cards; the selected one wears the
 * accent ring.
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
  type NodeChange,
  type NodeProps,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import { useReducedMotion } from "motion/react";
import { createContext, type RefObject, useContext, useEffect, useMemo, useRef, useState } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { useWide } from "@/features/shell/use-wide";
import { cx } from "@/utils/cx";

import { AddStep } from "./add-step";
import { addSize, cardHeight, cardWidth, type FlowItem, layoutWorkflow } from "./flow-layout";
import { KindMark } from "./parts";
import { type KindLook, stepLook, triggerLook } from "./step-kinds";
import { type Step, type StepKind, type TriggerKind, triggerLabel } from "./workflows";

export interface CanvasMoves {
  selected: string;
  onSelect: (id: string) => void;
  onAdd: (path: string | null, kind: StepKind) => void;
}

type CardNode = Node<{ look: KindLook; summary: string }, "card">;
type AddNode = Node<{ path: string | null }, "add">;
type PathEdge = Edge<{ label?: string }, "path">;
type Sizes = ReadonlyMap<string, { width: number; height: number }>;

const Moves = createContext<(CanvasMoves & { reveal: (id: string) => void }) | null>(null);

function useMoves() {
  const moves = useContext(Moves);
  if (!moves) {
    throw new Error("canvas nodes render only inside the canvas");
  }

  return moves;
}

/*
 * Card takes the keyboard focus when pressed, which the canvas's panning
 * would otherwise swallow, so Tab carries on from the card just picked.
 * React Flow turns pointer events off on nodes nobody can drag or
 * select, so cards and pluses turn them back on for themselves.
 */
function Card({ id, data }: NodeProps<CardNode>) {
  const moves = useMoves();
  const selected = moves.selected === id;

  return (
    <>
      <Handle type="target" position={Position.Top} isConnectable={false} className="invisible" />
      <button
        type="button"
        aria-pressed={selected}
        onClick={(event) => {
          event.currentTarget.focus({ preventScroll: true });
          moves.onSelect(id);
        }}
        onFocus={() => moves.reveal(id)}
        className={cx(
          "pointer-events-auto flex w-[264px] cursor-pointer flex-col gap-2 rounded-2xl bg-background-secondary-default p-3 text-start outline-none",
          "transition-[box-shadow,background-color] duration-150 hover:bg-background-secondary-hover",
          "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          selected && "ring-2 ring-accent-500",
        )}
      >
        <span className="flex items-center gap-2">
          <KindMark icon={data.look.icon} tint={data.look.tint} size="sm" />
          <span className="text-body-2-medium text-text-secondary">{data.look.name}</span>
        </span>
        <span className="wrap-break-word text-body-regular text-text-primary">{data.summary}</span>
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
      <span className="pointer-events-auto flex">
        <AddStep onAdd={(kind) => moves.onAdd(data.path, kind)} />
      </span>
    </>
  );
}

/*
 * Line runs from a card's foot to the next card's head. A line into a
 * new path turns sideways just under the split, so a split's lines read
 * as one bar, and carries the path's name just above the card it leads
 * to.
 */
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
            style={{ transform: `translate(-50%, -50%) translate(${targetX}px, ${targetY - 32}px)` }}
            className="nodrag nopan pointer-events-none absolute max-w-[240px] truncate rounded-full bg-background-tertiary-default px-2.5 py-1 text-body-2-medium text-text-secondary"
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

function lookOf(item: FlowItem, trigger: TriggerKind): { look: KindLook; summary: string } {
  if (item.kind === "step") {
    return { look: stepLook[item.step.kind], summary: item.step.summary };
  }

  return { look: triggerLook[trigger], summary: triggerLabel[trigger] };
}

function useGraph(trigger: TriggerKind, steps: Step[], sizes: Sizes) {
  return useMemo(() => {
    const heights = new Map([...sizes].map(([id, size]) => [id, size.height]));
    const { placed, links } = layoutWorkflow(steps, heights);
    const nodes: (CardNode | AddNode)[] = placed.map(({ item, x, y }) => {
      const measured = sizes.get(item.id);
      const base = { id: item.id, position: { x, y }, ...(measured ? { measured } : {}) };

      return item.kind === "add"
        ? { ...base, type: "add", data: { path: item.path } }
        : { ...base, type: "card", data: lookOf(item, trigger) };
    });
    const edges: PathEdge[] = links.map((link) => ({ ...link, type: "path", data: { label: link.label } }));

    return { nodes, edges };
  }, [trigger, steps, sizes]);
}

/*
 * measure keeps the sizes React Flow reports for each card, returning the
 * same map when nothing changed so the layout only runs again when a
 * card really grew or shrank.
 */
function measure(before: Sizes, changes: NodeChange[]): Sizes {
  let next: Map<string, { width: number; height: number }> | null = null;
  for (const change of changes) {
    if (change.type !== "dimensions" || !change.dimensions) {
      continue;
    }
    const size = { width: Math.round(change.dimensions.width), height: Math.round(change.dimensions.height) };
    const known = before.get(change.id);
    if (known?.width === size.width && known.height === size.height) {
      continue;
    }
    next ??= new Map(before);
    next.set(change.id, size);
  }

  return next ?? before;
}

/*
 * useReveal pans the canvas, at its current zoom, until the given card
 * is fully on screen, and leaves it alone when it already is.
 */
function useReveal(frame: RefObject<HTMLDivElement | null>) {
  const flow = useReactFlow();
  const still = useReducedMotion();

  return (id: string) => {
    const node = flow.getNode(id);
    const box = frame.current?.getBoundingClientRect();
    if (!node || !box) {
      return;
    }
    const { x, y, zoom } = flow.getViewport();
    const width = node.measured?.width ?? (node.type === "add" ? addSize : cardWidth);
    const height = node.measured?.height ?? (node.type === "add" ? addSize : cardHeight);
    const left = node.position.x * zoom + x;
    const top = node.position.y * zoom + y;
    if (left >= 0 && top >= 0 && left + width * zoom <= box.width && top + height * zoom <= box.height) {
      return;
    }
    flow.setCenter(node.position.x + width / 2, node.position.y + height / 2, {
      zoom,
      duration: still ? 0 : 300,
      interpolate: "linear",
    });
  };
}

function Zoom() {
  const flow = useReactFlow();
  const still = useReducedMotion();
  const duration = still ? 0 : 200;

  return (
    <div className="flex flex-col gap-1">
      <IconButton size="small" icon={RiAddLine} aria-label="Zoom in" onClick={() => flow.zoomIn({ duration })} />
      <IconButton size="small" icon={RiSubtractLine} aria-label="Zoom out" onClick={() => flow.zoomOut({ duration })} />
      <IconButton
        size="small"
        icon={RiFocus3Line}
        aria-label="Fit to screen"
        onClick={() => flow.fitView({ padding: 0.1, maxZoom: 1, duration })}
      />
    </div>
  );
}

function Flow({ trigger, steps, moves }: { trigger: TriggerKind; steps: Step[]; moves: CanvasMoves }) {
  const frame = useRef<HTMLDivElement>(null);
  const [sizes, setSizes] = useState<Sizes>(() => new Map());
  const { nodes, edges } = useGraph(trigger, steps, sizes);
  const reveal = useReveal(frame);
  const overview = useWide(768);
  const shown = useRef(moves.selected);

  useEffect(() => {
    if (shown.current !== moves.selected) {
      shown.current = moves.selected;
      reveal(moves.selected);
    }
  });

  return (
    <Moves.Provider value={{ ...moves, reveal }}>
      <div ref={frame} className="relative min-h-0 flex-1">
        <ReactFlow
          aria-label="Workflow canvas"
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          onNodesChange={(changes) => setSizes((before) => measure(before, changes))}
          onInit={(flow) => flow.setViewport({ x: (frame.current?.clientWidth ?? 0) / 2, y: 32, zoom: 1 })}
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
          {overview ? <MiniMap pannable zoomable ariaLabel="Workflow overview" className="rounded-2xl" /> : null}
        </ReactFlow>
      </div>
    </Moves.Provider>
  );
}

export function Canvas(props: { trigger: TriggerKind; steps: Step[]; moves: CanvasMoves }) {
  return (
    <ReactFlowProvider>
      <Flow {...props} />
    </ReactFlowProvider>
  );
}
