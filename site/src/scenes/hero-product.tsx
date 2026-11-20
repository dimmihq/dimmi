/*
 * The home hero's product moments, on a brand panel. Procedures: the list,
 * a pointer that clicks "New procedure", then the editor filling in until
 * the procedure is live. Then the AI agent follows that procedure with a
 * customer, and Operator briefs the teammate who picks it up. The
 * prerendered HTML shows the last scene, finished.
 */
import {
  RiAddLine,
  RiArrowRightLine,
  RiArrowRightSLine,
  RiBankCardLine,
  RiCheckLine,
  RiExchangeLine,
  RiGitBranchLine,
  RiMailLine,
  RiRefund2Line,
  RiSparkling2Fill,
  RiSparkling2Line,
} from "@remixicon/react";
import { motion } from "motion/react";
import type { ReactNode } from "react";

import { DimmiIcon } from "@/features/brand/logo";

import { cx } from "@/utils/cx";

import { Customer, Thinking } from "./agent-pieces";
import { AgentStory } from "./agent-scene";
import { STORIES } from "./agent-stories";

import { enter } from "./motion";
import { useScene } from "./use-scene";

const procedures = [
  { name: "Change plan", icon: RiExchangeLine, tone: "bg-blue-100 text-blue-600 dark:bg-blue-950 dark:text-blue-300" },
  {
    name: "Update billing card",
    icon: RiBankCardLine,
    tone: "bg-orange-200/60 text-orange-600 dark:bg-orange-950 dark:text-orange-300",
  },
  {
    name: "Cancel subscription",
    icon: RiRefund2Line,
    tone: "bg-pink-100 text-pink-500 dark:bg-pink-950 dark:text-pink-300",
  },
  {
    name: "Change account email",
    icon: RiMailLine,
    tone: "bg-purple-100 text-purple-600 dark:bg-purple-950 dark:text-purple-300",
  },
];

const steps = [
  { kind: "Instruction", text: "Look up the customer's account and plan." },
  { kind: "Instruction", text: "Find the charges for the month they name." },
  { kind: "Condition", text: "If there is only one charge, explain it and stop." },
  { kind: "Action", text: "Refund the duplicate once Billing approves it." },
];

/*
 * The hero plays three features in turn, each a scene of its own steps:
 * writing a procedure, the AI agent following it with a customer, and
 * Operator briefing a teammate. Each entry is how long a step stays.
 */
const SCENES = [
  {
    name: "Procedures",
    sparkle: false,
    // 0 list, 1 pointer arrives, 2 click, 3 editor, 4–7 one step each, 8 live.
    waits: [1300, 700, 500, 900, 800, 800, 800, 900, 2600],
  },
  { name: "dimmi", sparkle: true, waits: [1100, 1100, 1300, 1200, 2800] },
  { name: "Operator", sparkle: false, waits: [1100, 1300, 1400, 3000] },
];

const TIMELINE = SCENES.flatMap((scene, index) => scene.waits.map((wait, local) => ({ index, local, wait })));

export function HeroProduct({ className }: { className?: string }) {
  const { ref, step, goTo } = useScene(TIMELINE.length, {
    interval: TIMELINE.map((t) => t.wait),
    hold: TIMELINE[TIMELINE.length - 1].wait,
  });
  const { index: active, local } = TIMELINE[step];
  // A picked scene plays from its start; with reduced motion it shows finished.
  const pick = (index: number) => {
    const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const first = TIMELINE.findIndex((t) => t.index === index);
    goTo(still ? first + SCENES[index].waits.length - 1 : first);
  };

  return (
    <div
      ref={ref}
      className={cx(
        "relative flex flex-col items-center justify-center gap-6 overflow-hidden bg-accent-600 px-5 py-12 sm:px-10",
        className,
      )}
    >
      <Pattern />
      {/* Every scene shares one grid cell of a fixed size, so switching only fades and nothing around it moves. */}
      <div className="relative grid h-[560px] w-full max-w-[420px] overflow-hidden rounded-2xl border border-black/5 bg-background-primary-default shadow-[0_24px_60px_-20px_rgb(0_0_0/0.45)] [&>*]:[grid-area:1/1] [&>*]:min-h-0">
        <Layer shown={active === 0}>
          <ProcedureScene step={active === 0 ? local : SCENES[0].waits.length - 1} />
        </Layer>
        <Layer shown={active === 1}>
          <Frame title="Omar Haddad" detail="Email · Northwind Labs">
            <AgentStory story={STORIES[1]} frame={active === 1 ? local : 0} />
          </Frame>
        </Layer>
        <Layer shown={active === 2}>
          <Frame title="Operator" detail="Only your team sees this">
            <OperatorScene step={active === 2 ? local : 0} />
          </Frame>
        </Layer>
      </div>
      <fieldset
        aria-label="Product tour"
        className="relative m-0 flex min-w-0 items-center gap-1 rounded-full border-0 bg-black/15 p-1"
      >
        {SCENES.map((scene, i) => (
          <button
            key={scene.name}
            type="button"
            aria-pressed={i === active}
            onClick={() => pick(i)}
            className={cx(
              "inline-flex cursor-pointer items-center gap-1 rounded-full px-3 py-1 text-caption-1-medium outline-none transition-colors duration-300 focus-visible:ring-2 focus-visible:ring-white",
              i === active ? "bg-white text-accent-700" : "text-white/80 hover:bg-white/10 hover:text-white",
            )}
          >
            {scene.sparkle ? <RiSparkling2Fill className="size-3.5" aria-hidden /> : null}
            {scene.name}
          </button>
        ))}
      </fieldset>
    </div>
  );
}

function Layer({ shown, children }: { shown: boolean; children: ReactNode }) {
  return (
    <motion.div
      initial={false}
      animate={{ opacity: shown ? 1 : 0, y: shown ? 0 : 8, filter: shown ? "blur(0px)" : "blur(3px)" }}
      transition={{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }}
      aria-hidden={!shown}
      className={cx("flex flex-col", !shown && "pointer-events-none")}
    >
      {children}
    </motion.div>
  );
}

/* A conversation's frame: a header, then the messages. */
function Frame({ title, detail, children }: { title: string; detail: string; children: ReactNode }) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2.5 border-b border-separator-border px-5 py-4">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-text-primary text-background-full">
          <DimmiIcon className="size-4.5" title="dimmi" />
        </span>
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-headline-medium text-text-primary">{title}</span>
          <span className="truncate text-caption-1-regular text-text-tertiary">{detail}</span>
        </span>
      </div>
      {/* Messages stack from the top, so a new one never moves the ones before it. */}
      <div className="min-h-0 flex-1 overflow-hidden bg-background-secondary-default p-4">{children}</div>
    </div>
  );
}

function ProcedureScene({ step }: { step: number }) {
  const editing = step >= 3;
  return (
    <div className="grid h-full [&>*]:[grid-area:1/1]">
      <motion.div
        initial={false}
        animate={{ opacity: editing ? 0 : 1, filter: editing ? "blur(3px)" : "blur(0px)" }}
        transition={{ duration: 0.35 }}
        aria-hidden={editing}
      >
        <List pointer={step >= 1 && !editing} pressed={step === 2} />
      </motion.div>
      <motion.div
        initial={false}
        animate={{ opacity: editing ? 1 : 0, y: editing ? 0 : 8, filter: editing ? "blur(0px)" : "blur(3px)" }}
        transition={{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }}
        aria-hidden={!editing}
      >
        <Editor shown={editing ? step - 3 : 0} live={step >= 3 + steps.length + 1} />
      </motion.div>
    </div>
  );
}

const operatorFrames: ReactNode[] = [
  <Customer key="q" name="Priya Moreau · Billing" initials="PM">
    What has Omar asked us before? Brief me before I reply.
  </Customer>,
  <Thinking key="t" label="Reading Omar's history" />,
  <motion.div
    key="a"
    {...enter}
    className="me-auto flex w-[88%] flex-col gap-2 rounded-xl border border-separator-border bg-background-primary-default px-3.5 py-3 shadow-xs"
  >
    <span className="inline-flex items-center gap-1.5 text-caption-1-medium text-text-tertiary">
      <RiSparkling2Line className="size-3.5 text-accent-600" aria-hidden />
      Operator
    </span>
    <ul className="flex list-disc flex-col gap-1 ps-4 text-body-regular text-text-primary">
      <li>Northwind Labs, Growth plan since March.</li>
      <li>3 conversations this year, all billing.</li>
      <li>The AI agent refunded a duplicate $49.00 today.</li>
    </ul>
  </motion.div>,
  <motion.div key="d" {...enter} className="me-auto flex items-center gap-2">
    <span className="rounded-lg border border-border-button-default bg-background-full px-3 py-1.5 text-caption-1-medium text-text-primary shadow-xs">
      Draft a reply
    </span>
    <span className="rounded-lg bg-text-primary px-3 py-1.5 text-caption-1-medium text-background-full">
      Insert summary
    </span>
  </motion.div>,
];

function OperatorScene({ step }: { step: number }) {
  return <AgentStory story={{ channel: "Operator", frames: operatorFrames }} frame={step} />;
}

function List({ pointer, pressed }: { pointer: boolean; pressed: boolean }) {
  return (
    <div className="flex h-full flex-col gap-4 p-5">
      <span className="px-1 text-headline-medium text-text-primary">Procedures</span>
      <ul className="flex flex-col">
        {procedures.map((p) => (
          <li key={p.name} className="flex items-center gap-3 rounded-lg px-1 py-3">
            <span className={cx("flex size-9 items-center justify-center rounded-lg", p.tone)}>
              <p.icon className="size-5" aria-hidden />
            </span>
            <span className="flex-1 text-body-regular text-text-primary">{p.name}</span>
            <RiArrowRightSLine className="size-4 text-foreground-icon-tertiary" aria-hidden />
          </li>
        ))}
      </ul>
      <div className="relative mt-auto">
        <motion.span
          animate={{ scale: pressed ? 0.97 : 1 }}
          className={cx(
            "flex items-center justify-center gap-1.5 rounded-lg border border-border-button-default px-3 py-2.5 text-body-medium text-text-primary shadow-xs transition-colors",
            pressed ? "bg-background-secondary-hover" : "bg-background-full",
          )}
        >
          <RiAddLine className="size-4" aria-hidden />
          New procedure
        </motion.span>
        <motion.svg
          viewBox="0 0 24 24"
          aria-hidden
          initial={false}
          animate={pointer ? { x: 0, y: 0, opacity: 1, scale: pressed ? 0.88 : 1 } : { x: 90, y: 70, opacity: 0 }}
          transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
          className="pointer-events-none absolute top-5 left-[58%] size-6 drop-shadow-[0_2px_3px_rgb(0_0_0/0.3)]"
        >
          <path d="M4 2l16 9.5-7 1.6-3.6 6.4z" fill="black" stroke="white" strokeWidth="1.5" strokeLinejoin="round" />
        </motion.svg>
      </div>
    </div>
  );
}

function Editor({ shown, live }: { shown: number; live: boolean }) {
  return (
    <div className="flex flex-col">
      <div className="flex items-center justify-between gap-3 border-b border-separator-border px-5 py-4">
        <span className="flex min-w-0 items-center gap-2.5">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-accent-100 text-accent-700 dark:bg-accent-900 dark:text-accent-300">
            <RiRefund2Line className="size-5" aria-hidden />
          </span>
          <span className="truncate text-headline-medium text-text-primary">Refund a duplicate charge</span>
        </span>
        <span
          className={cx(
            "inline-flex w-14 shrink-0 items-center justify-center gap-1 rounded-full px-2 py-0.5 text-caption-1-medium transition-colors duration-300",
            live
              ? "bg-accent-100 text-accent-700 dark:bg-accent-900 dark:text-accent-300"
              : "bg-background-tertiary-default text-text-secondary",
          )}
        >
          {live ? <RiCheckLine className="size-3" aria-hidden /> : null}
          {live ? "Live" : "Draft"}
        </span>
      </div>
      <div className="flex flex-col gap-2 px-5 pt-5">
        <span className="text-caption-1-medium text-text-tertiary">When dimmi uses this</span>
        <p className="rounded-lg bg-background-secondary-default px-3.5 py-2.5 text-body-regular text-text-primary">
          The customer was charged twice for the same period and asks for one charge back.
        </p>
      </div>
      <div className="flex flex-col gap-2 px-5 pt-5 pb-5">
        <span className="text-caption-1-medium text-text-tertiary">Steps</span>
        <ol className="flex flex-col gap-2.5">
          {/* Every step holds its place from the start and fades in, so the card never grows. */}
          {steps.map((s, i) => (
            <motion.li
              key={s.text}
              initial={false}
              animate={i < shown ? enter.animate : enter.initial}
              transition={enter.transition}
              className="flex items-start gap-2.5"
            >
              <span className="w-3 pt-2.5 text-caption-1-regular text-text-tertiary tabular-nums">{i + 1}</span>
              <span
                className={cx(
                  "mt-1 flex size-7 shrink-0 items-center justify-center rounded-md",
                  s.kind === "Condition"
                    ? "bg-purple-100 text-purple-600 dark:bg-purple-950 dark:text-purple-300"
                    : s.kind === "Action"
                      ? "bg-accent-100 text-accent-700 dark:bg-accent-900 dark:text-accent-300"
                      : "bg-background-tertiary-default text-foreground-icon-secondary",
                )}
              >
                {s.kind === "Condition" ? (
                  <RiGitBranchLine className="size-4" aria-hidden />
                ) : (
                  <RiArrowRightLine className="size-4" aria-hidden />
                )}
              </span>
              <span className="flex-1 rounded-lg bg-background-secondary-default px-3 py-2 text-body-regular text-text-primary">
                {s.text}
              </span>
            </motion.li>
          ))}
        </ol>
      </div>
    </div>
  );
}

/*
 * dimmi's own ground, the one under every hero: the 64px hairline grid,
 * here in white on the brand green, with a small node where lines cross,
 * fading out from the middle.
 */
function Pattern() {
  return (
    <svg
      aria-hidden
      className="pointer-events-none absolute inset-0 size-full text-white [mask-image:radial-gradient(70%_60%_at_50%_45%,black,transparent)]"
    >
      <defs>
        {/* Lines cross in the middle of each tile, so the node there is never clipped. */}
        <pattern id="hero-grid" width="64" height="64" patternUnits="userSpaceOnUse">
          <path d="M32.5 0V64M0 32.5H64" fill="none" stroke="currentColor" strokeOpacity="0.14" />
          <circle cx="32.5" cy="32.5" r="1.6" fill="currentColor" fillOpacity="0.45" />
        </pattern>
      </defs>
      <rect width="100%" height="100%" fill="url(#hero-grid)" />
    </svg>
  );
}
