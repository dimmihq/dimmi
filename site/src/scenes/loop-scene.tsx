/*
 * The feedback loop as one story across four products: a request lands
 * on the feedback board, votes come in, it ships, the changelog goes out
 * and everyone who asked hears back. Each station lights up in turn.
 */
import { RiArrowUpLine, RiCheckLine, RiMailSendLine, RiSparkling2Line } from "@remixicon/react";
import { motion } from "motion/react";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import { useScene } from "./use-scene";

function Station({
  index,
  step,
  label,
  title,
  children,
}: {
  index: number;
  step: number;
  label: string;
  title: string;
  children: ReactNode;
}) {
  const lit = step >= index;
  return (
    <motion.div
      animate={{ opacity: lit ? 1 : 0.45, scale: step === index ? 1 : 0.985 }}
      transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
      className={cx(
        "flex flex-col gap-3 rounded-2xl border bg-background-full p-5 shadow-xs transition-colors",
        step === index ? "border-border-button-hover" : "border-separator-border",
      )}
    >
      <div className="flex items-center justify-between">
        <span className="text-caption-1-semibold text-text-tertiary">{label}</span>
        <span
          className={cx(
            "flex size-5 items-center justify-center rounded-full transition-colors",
            lit ? "bg-accent-500 text-text-white" : "border border-dashed border-border-button-hover",
          )}
        >
          {lit ? <RiCheckLine className="size-3" aria-hidden /> : null}
        </span>
      </div>
      <span className="text-headline-semibold text-text-primary">{title}</span>
      {children}
    </motion.div>
  );
}

export function LoopScene() {
  const { ref, step } = useScene(4, { interval: 1600, hold: 3600 });
  const votes = [12, 48, 48, 48][step];

  return (
    <div ref={ref} className="grid gap-4 sm:grid-cols-2">
      <Station index={0} step={step} label="Feedback board" title="Export conversations to CSV">
        <div className="flex items-center gap-3">
          <span className="flex flex-col items-center rounded-lg border border-separator-border bg-background-secondary-default px-2.5 py-1.5">
            <RiArrowUpLine className="size-3.5 text-foreground-icon-secondary" aria-hidden />
            <span className="text-body-semibold text-text-primary tabular-nums">{votes}</span>
          </span>
          <span className="text-body-regular text-text-secondary">Requested from 9 conversations</span>
        </div>
      </Station>
      <Station index={1} step={step} label="Roadmap" title="Status changes as you build">
        <div className="flex flex-wrap gap-2">
          <Chip variant="caption" color={step >= 1 ? "gray" : "neutral"}>
            Planned
          </Chip>
          <Chip variant="caption" color={step >= 1 ? "blue" : "neutral"}>
            In progress
          </Chip>
          <Chip variant="caption" color={step >= 1 ? "lime" : "neutral"}>
            Shipped
          </Chip>
        </div>
      </Station>
      <Station index={2} step={step} label="Changelog" title="CSV export is here">
        <span className="inline-flex items-center gap-1.5 text-body-regular text-text-secondary">
          <RiSparkling2Line className="size-4 text-foreground-icon-secondary" aria-hidden />
          Published to your changelog and the messenger
        </span>
      </Station>
      <Station index={3} step={step} label="Email" title="Everyone who asked hears back">
        <span className="inline-flex items-center gap-1.5 text-body-regular text-text-secondary">
          <RiMailSendLine className="size-4 text-foreground-icon-secondary" aria-hidden />
          Sent to {votes} voters and subscribers
        </span>
      </Station>
    </div>
  );
}
