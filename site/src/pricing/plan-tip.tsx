/*
 * The small "i" beside a competitor's name in a comparison. The bars use
 * each competitor's cheapest plan, which leaves features out; the tip says
 * what the same month costs on the plan that has them, whenever dimmi is
 * the cheaper of the two. It opens on hover, focus or a tap.
 */
import { RiInformation2Line } from "@remixicon/react";
import { useId, useState } from "react";

import { cx } from "@/utils/cx";

import { type Competitor, fullBill } from "./competitors";
import { dollars, type Usage } from "./prices";

export function PlanTip({
  competitor,
  usage,
  cheapest,
  ours,
}: {
  competitor: Competitor;
  usage: Usage & { seats: number };
  /* The competitor's bill on its cheapest plan, the one the bar draws. */
  cheapest: number;
  /* dimmi's bill for the same month; null when it is a custom price. */
  ours: number | null;
}) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const full = fullBill(competitor, usage);
  if (ours === null || full <= ours) {
    return null;
  }
  // The cheapest plan undercuts dimmi here, and the full one does not.
  const flips = cheapest < ours;

  return (
    <span
      className="relative inline-flex shrink-0"
      onPointerEnter={(event) => event.pointerType === "mouse" && setOpen(true)}
      onPointerLeave={(event) => event.pointerType === "mouse" && setOpen(false)}
    >
      <button
        type="button"
        aria-label={`${competitor.name} with every feature`}
        aria-describedby={open ? id : undefined}
        onClick={() => setOpen((was) => !was)}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={(event) => event.key === "Escape" && setOpen(false)}
        className={cx(
          "inline-flex size-5 cursor-pointer items-center justify-center rounded-full outline-none transition-colors focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          flips
            ? "text-accent-600 hover:text-accent-700 dark:text-accent-400 dark:hover:text-accent-300"
            : "text-foreground-icon-tertiary hover:text-text-secondary",
        )}
      >
        <RiInformation2Line className="size-3.5" aria-hidden />
      </button>
      <span
        id={id}
        role="tooltip"
        className={cx(
          "absolute bottom-full -left-2 z-20 mb-2 flex w-[264px] flex-col max-sm:w-[232px] gap-1.5 rounded-xl border border-border-button-default bg-background-full p-3.5 text-start text-[13px] leading-5 font-normal text-text-secondary shadow-[0_16px_40px_-16px_rgb(0_0_0/0.3)] transition-[opacity,translate,visibility] duration-150",
          open ? "visible translate-y-0 opacity-100" : "invisible translate-y-1 opacity-0",
        )}
      >
        <span>
          {dollars(Math.round(cheapest))} is their cheapest plan. It has no {competitor.full.adds}.
        </span>
        <span>
          With those, {competitor.name} is {competitor.full.plan} at {dollars(competitor.full.seat)} a seat:{" "}
          <span className="font-medium text-text-primary tabular-nums">{dollars(Math.round(full))}</span> a month.
        </span>
        <span>
          dimmi is{" "}
          <span className="font-medium text-accent-700 tabular-nums dark:text-accent-300">
            {dollars(Math.round(ours))}
          </span>
          , with all of it included.
        </span>
      </span>
    </span>
  );
}
