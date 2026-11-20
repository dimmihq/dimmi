/*
 * The price book drawn as steps: one column per volume band, with an AI
 * bar and a human bar whose heights are the band's prices, so the way
 * prices fall as a workspace grows reads at a glance. The first 100
 * conversations stand at the start as the free step.
 */
import { cx } from "@/utils/cx";

import { BANDS, cents, FREE_CONVERSATIONS } from "./prices";

const TALLEST = Math.max(...BANDS.map((b) => b.ai));

export function PriceSteps() {
  let from = FREE_CONVERSATIONS + 1;
  const steps = BANDS.map((band) => {
    const step = { from, ...band };
    from = band.upTo + 1;
    return step;
  });

  return (
    <figure className="flex flex-col gap-5">
      <div className="grid grid-cols-6 items-end gap-1.5 border-b border-separator-border sm:gap-3">
        <div className="flex flex-col items-center gap-2 pb-px">
          <span className="rounded-full bg-accent-500/12 px-2 py-0.5 text-[12px] leading-4 font-medium text-accent-700 dark:text-accent-300">
            Free
          </span>
          <div className="h-1.5 w-full rounded-t-md bg-accent-500/40" />
        </div>
        {steps.map((step) => (
          <div key={step.from} className="flex items-end justify-center gap-1 sm:gap-1.5">
            <Bar value={step.ai} label={cents(step.ai)} className="bg-accent-500" />
            <Bar value={step.human} label={cents(step.human)} className="bg-foreground-icon-tertiary/40" />
          </div>
        ))}
      </div>
      <div className="grid grid-cols-6 gap-1.5 text-center sm:gap-3">
        <span className="text-[12px] leading-4 text-text-secondary tabular-nums sm:text-[13px]">
          First {FREE_CONVERSATIONS}
        </span>
        {steps.map((step) => (
          <span key={step.from} className="text-[12px] leading-4 text-text-secondary tabular-nums sm:text-[13px]">
            {step.from.toLocaleString("en-US")}+
          </span>
        ))}
      </div>
      <figcaption className="flex flex-wrap gap-x-5 gap-y-2 text-[13px] leading-5 text-text-secondary">
        <Key className="bg-accent-500">AI conversation</Key>
        <Key className="bg-foreground-icon-tertiary/40">Human, or AI on your own key</Key>
        <span>Conversations a month, per band</span>
      </figcaption>
    </figure>
  );
}

function Bar({ value, label, className }: { value: number; label: string; className: string }) {
  return (
    <div className="flex w-full max-w-9 flex-col items-center gap-1.5">
      <span className="text-[11px] leading-4 font-medium text-text-primary tabular-nums sm:text-[13px]">{label}</span>
      <div className={cx("w-full rounded-t-md", className)} style={{ height: `${(value / TALLEST) * 180}px` }} />
    </div>
  );
}

function Key({ className, children }: { className: string; children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className={cx("size-2.5 rounded-sm", className)} aria-hidden />
      {children}
    </span>
  );
}
