/*
 * Replays a click-through captured from the real app (see the flows in
 * site/shots/capture.ts): the frames are real screenshots, and a drawn
 * pointer glides to whatever the next step acts on, clicks or hovers,
 * and the next frame takes over. It plays while in view and loops. The
 * prerendered page, and anyone who asked for reduced motion, sees the
 * first frame.
 */
import { motion } from "motion/react";
import { useEffect, useState } from "react";

import { cx } from "@/utils/cx";

import flows from "../flows.json";
import { useStartInView } from "./use-scene";

export type FlowName = keyof typeof flows;
type Step = { x: number; y: number; kind: string };

const READ_MS = 1100;
const MOVE_MS = 750;
const HOLD_MS = 2200;

function Pointer({ pressed }: { pressed: boolean }) {
  return (
    <motion.svg
      viewBox="0 0 24 24"
      className="size-6 drop-shadow-[0_2px_4px_rgb(0_0_0/0.35)]"
      animate={{ scale: pressed ? 0.85 : 1 }}
      transition={{ duration: 0.12 }}
      aria-hidden
    >
      <path d="M5 3l14 7.5-6.2 1.6L10.5 19z" fill="black" stroke="white" strokeWidth="1.5" strokeLinejoin="round" />
    </motion.svg>
  );
}

export function ShotFlow({
  name,
  alt,
  maxHeight = 410,
  maxWidth = 500,
  anchor = "bottom",
  className,
}: {
  name: FlowName;
  alt: string;
  /* The most room the capture may take, in CSS pixels; it scales down to fit both. */
  maxHeight?: number;
  maxWidth?: number;
  /* Which part of the capture stays in view when the panel is smaller than it. */
  anchor?: "bottom" | "top" | "top-right";
  className?: string;
}) {
  const flow = flows[name];
  const frames = flow.frames as { next?: Step }[];
  const { ref, started, inView } = useStartInView();
  const [frame, setFrame] = useState(0);
  const [pointer, setPointer] = useState<{ x: number; y: number } | null>(null);
  const [pressed, setPressed] = useState(false);
  const [ripple, setRipple] = useState(0);

  const scale = Math.min(1, maxHeight / flow.height, maxWidth / flow.width);
  const width = Math.round(flow.width * scale);

  // Fetch every frame as soon as the flow is first seen, so no step ever
  // lands on a frame the browser has not loaded yet.
  useEffect(() => {
    if (!started) {
      return;
    }
    const scheme = document.documentElement.classList.contains("dark") ? "dark" : "light";
    for (const i of frames.keys()) {
      new Image().src = `/shots/flow-${name}-${i}-${scheme}.webp`;
    }
  }, [started, frames, name]);

  useEffect(() => {
    if (!started || !inView) {
      return;
    }
    let cancelled = false;
    const timers: ReturnType<typeof setTimeout>[] = [];
    const wait = (ms: number) =>
      new Promise<void>((resolve) => {
        timers.push(setTimeout(resolve, ms));
      });

    // One step: glide to the target, press or pause, then show the next frame.
    const act = async (next: Step, i: number) => {
      setPointer({ x: next.x, y: next.y });
      await wait(MOVE_MS);
      if (next.kind === "click") {
        setPressed(true);
        setRipple((r) => r + 1);
        await wait(140);
        setPressed(false);
      }
      if (next.kind === "type") {
        await wait(300);
      }
      setFrame(i + 1);
      await wait(READ_MS);
    };

    const play = async () => {
      while (!cancelled) {
        setFrame(0);
        setPointer({ x: flow.width * 0.82, y: flow.height * 0.9 });
        await wait(READ_MS);
        for (const [i, f] of frames.entries()) {
          if (cancelled || !f.next) {
            break;
          }
          await act(f.next, i);
        }
        await wait(HOLD_MS);
      }
    };
    play();

    return () => {
      cancelled = true;
      for (const t of timers) {
        clearTimeout(t);
      }
    };
  }, [started, inView, flow.width, flow.height, frames]);

  return (
    <div
      ref={ref}
      className={cx(
        "relative flex h-full overflow-hidden",
        anchor === "bottom" && "items-end justify-center px-6",
        anchor === "top" && "items-start justify-center px-6",
        anchor === "top-right" && "items-start justify-end pe-8",
        className,
      )}
    >
      <div
        className="relative shrink-0 overflow-hidden rounded-t-xl border border-b-0 border-border-button-default bg-background-full p-1 pb-0 shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]"
        style={{ width, maxWidth: anchor === "top-right" ? undefined : "100%" }}
      >
        <div
          className="relative overflow-hidden rounded-t-lg border border-b-0 border-separator-border"
          style={{ aspectRatio: `${flow.width} / ${flow.height}` }}
        >
          {frames
            .map((_, i) => ({ i, id: `flow-${name}-${i}` }))
            .map(({ i, id }) => (
              <div
                key={id}
                className={cx(
                  "absolute inset-0 transition-opacity duration-200",
                  i === frame ? "opacity-100" : "opacity-0",
                )}
                aria-hidden={i !== 0}
              >
                <img
                  src={`/shots/flow-${name}-${i}-light.webp`}
                  alt={i === 0 ? alt : ""}
                  loading={i === 0 ? "eager" : undefined}
                  decoding="async"
                  className="block size-full dark:hidden"
                />
                <img
                  src={`/shots/flow-${name}-${i}-dark.webp`}
                  alt={i === 0 ? alt : ""}
                  loading={i === 0 ? "eager" : undefined}
                  decoding="async"
                  className="hidden size-full dark:block"
                />
              </div>
            ))}
          {pointer ? (
            <motion.div
              className="pointer-events-none absolute top-0 left-0 z-10"
              initial={false}
              animate={{ left: `${(pointer.x / flow.width) * 100}%`, top: `${(pointer.y / flow.height) * 100}%` }}
              transition={{ duration: MOVE_MS / 1000, ease: [0.45, 0, 0.2, 1] }}
            >
              <span className="absolute -top-0.5 -left-0.5 block">
                <Pointer pressed={pressed} />
              </span>
              {ripple > 0 ? (
                <motion.span
                  key={ripple}
                  className="absolute -top-4 -left-4 block size-8 rounded-full border-2 border-accent-500"
                  initial={{ scale: 0.3, opacity: 0.9 }}
                  animate={{ scale: 1.4, opacity: 0 }}
                  transition={{ duration: 0.5, ease: "easeOut" }}
                />
              ) : null}
            </motion.div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
