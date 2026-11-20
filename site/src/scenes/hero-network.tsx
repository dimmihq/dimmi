/*
 * The hero visual. The prerendered HTML carries an SVG still of the
 * network, so the first paint already shows it; after load the three.js
 * scene takes over in the same spot and starts moving. With reduced
 * motion the still stays.
 */
import { useEffect, useRef, useState } from "react";

import { DimmiIcon } from "@/features/brand/logo";
import { cx } from "@/utils/cx";

import { buildNetwork, project } from "./network";

const poster = buildNetwork().map((node, i) => ({ id: `n${i}`, node, p: project(node, 0.6) }));
const SIZE = 1000;
const SPREAD = 330;

function Poster() {
  return (
    <svg viewBox={`0 0 ${SIZE} ${SIZE}`} className="absolute inset-0 size-full" aria-hidden>
      <g transform={`translate(${SIZE / 2} ${SIZE / 2})`}>
        {poster.map(({ id, node, p }) => (
          <line
            key={`l${id}`}
            x1={0}
            y1={0}
            x2={p.x * SPREAD}
            y2={-p.y * SPREAD}
            className="stroke-text-tertiary"
            strokeOpacity={0.22 * Math.max(0.3, 1 - (1 - p.z) * 0.4)}
            strokeWidth={1.2}
            data-kind={node.kind}
          />
        ))}
        {poster.map(({ id, node, p }) => (
          <circle
            key={`c${id}`}
            cx={p.x * SPREAD}
            cy={-p.y * SPREAD}
            r={(node.kind === "teammate" ? 5 : 3) * p.scale}
            className={node.kind === "teammate" ? "fill-accent-500" : "fill-text-tertiary"}
            fillOpacity={node.kind === "teammate" ? 0.95 : 0.55}
          />
        ))}
      </g>
    </svg>
  );
}

export function HeroNetwork({ className }: { className?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const markRef = useRef<HTMLDivElement>(null);
  const [live, setLive] = useState(false);
  const [counts, setCounts] = useState({ answered: 0, routed: 0 });

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      return;
    }
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    let unmount: (() => void) | undefined;
    let cancelled = false;

    const pulse = () => {
      markRef.current?.animate(
        [
          { boxShadow: "0 0 0 0 color-mix(in oklab, var(--color-accent-500) 45%, transparent)" },
          { boxShadow: "0 0 0 22px color-mix(in oklab, var(--color-accent-500) 0%, transparent)" },
        ],
        { duration: 900, easing: "cubic-bezier(0.22, 1, 0.36, 1)" },
      );
    };

    import("./network-scene").then(({ mountNetwork }) => {
      if (cancelled) {
        return;
      }
      unmount = mountNetwork(canvas, {
        onResolved: () => {
          pulse();
          setCounts((c) => ({ ...c, answered: c.answered + 1 }));
        },
        onRouted: () => setCounts((c) => ({ ...c, routed: c.routed + 1 })),
      });
      requestAnimationFrame(() => setLive(true));
    });

    return () => {
      cancelled = true;
      unmount?.();
    };
  }, []);

  return (
    <div className={cx("relative aspect-square w-full select-none", className)}>
      <div className={cx("absolute inset-0 transition-opacity duration-700", live ? "opacity-0" : "opacity-100")}>
        <Poster />
      </div>
      <canvas
        ref={canvasRef}
        className={cx(
          "absolute inset-0 size-full cursor-grab touch-pan-y transition-opacity duration-700 active:cursor-grabbing",
          live ? "opacity-100" : "opacity-0",
        )}
        aria-hidden
      />
      <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
        <div
          ref={markRef}
          className="flex size-[76px] items-center justify-center rounded-full border border-border-button-default bg-background-primary-default shadow-md"
        >
          <DimmiIcon className="size-9 text-text-primary" title="dimmi" />
        </div>
      </div>
      <div className="pointer-events-none absolute inset-x-0 bottom-2 flex justify-center gap-4 text-caption-1-medium text-text-tertiary tabular-nums">
        <span className="inline-flex items-center gap-1.5">
          <span className="size-1.5 rounded-full bg-text-primary" />
          {counts.answered} answered by dimmi
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="size-1.5 rounded-full bg-accent-500" />
          {counts.routed} routed to your team
        </span>
      </div>
    </div>
  );
}
