/*
 * Fixed-height Agent Studio quality-loop card for HowItWorks:
 * one stage lit per step with zero height change as steps advance.
 * 0 Draft guidance with @ chips, 1 Multi-run simulations & flakiness,
 * 2 Draft vs. Live comparison, 3 Operator diagnosis & one-click fix.
 */
import {
  RiCheckLine,
  RiCloseCircleFill,
  RiFlashlightLine,
  RiGitPullRequestLine,
  RiListCheck2,
  RiSparkling2Line,
  RiTestTubeLine,
} from "@remixicon/react";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

function Stage({ label, lit, children }: { label: string; lit: boolean; children: ReactNode }) {
  return (
    <div
      className={cx(
        "flex flex-col gap-1.5 rounded-xl border bg-background-full p-2.5 transition-[border-color,opacity] duration-500",
        lit ? "border-accent-500 opacity-100" : "border-separator-border opacity-55",
      )}
    >
      <span className="text-caption-1-medium text-text-secondary">{label}</span>
      {children}
    </div>
  );
}

function RunDot({ pass }: { pass: boolean }) {
  return (
    <span
      className={cx("size-2 rounded-full", pass ? "bg-notification-success-foreground" : "bg-foreground-icon-error")}
      aria-hidden
    />
  );
}

export function AgentStudioLoopFragment({ step }: { step: number }) {
  const ready = step === 3;
  return (
    <div
      className="flex h-[510px] w-[440px] max-w-full flex-col gap-1.5 overflow-hidden rounded-2xl border border-border-button-default bg-background-secondary-default p-2 shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]"
      aria-hidden
    >
      <div
        className={cx(
          "flex items-center gap-2 rounded-xl border px-3 py-2 transition-[border-color,opacity] duration-500",
          ready ? "border-accent-500 bg-background-full" : "border-transparent",
        )}
      >
        <span className="flex size-5 items-center justify-center rounded-md bg-status-lime-bg text-caption-1-medium text-status-lime-text">
          S
        </span>
        <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">Sky</span>
        <span className="inline-flex items-center gap-1 rounded-md bg-status-lime-bg px-2 py-0.5 text-caption-1-medium text-status-lime-text tabular-nums">
          <RiTestTubeLine className="size-3" aria-hidden />
          {ready ? "8/8" : "7/8"}
        </span>
        <span
          className={cx(
            "inline-flex h-7 items-center rounded-lg px-2.5 text-body-2-medium transition-colors duration-500",
            ready
              ? "bg-linear-to-b from-accent-500 to-accent-600 text-white"
              : "bg-background-tertiary-default text-text-tertiary",
          )}
        >
          Publish
        </span>
      </div>

      <Stage label="Guidance" lit={step === 0}>
        <p className="text-body-2-regular leading-5 text-text-primary">
          Check parcels with{" "}
          <span className="inline-flex items-center gap-1 rounded-md bg-status-lime-bg px-1.5 py-0.5 text-caption-1-medium text-status-lime-text">
            <RiFlashlightLine className="size-3" />
            Look up a parcel
          </span>{" "}
          and follow{" "}
          <span className="inline-flex items-center gap-1 rounded-md bg-status-purple-bg px-1.5 py-0.5 text-caption-1-medium text-status-purple-text">
            <RiListCheck2 className="size-3" />
            Refund for a voided label
          </span>
          .
        </p>
      </Stage>

      <Stage label="Simulations · 5 recent runs" lit={step === 1}>
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 flex-col gap-1">
            <span className="truncate text-body-2-medium text-text-primary">Voided USPS label refund window</span>
            <span className="flex items-center gap-1">
              <RunDot pass />
              <RunDot pass={false} />
              <RunDot pass />
              <RunDot pass={false} />
              <RunDot pass />
              <span className="ms-1 text-caption-1-regular text-text-tertiary">3 of 5 passed</span>
            </span>
          </div>
          <Chip variant="caption" color="yellow" className="shrink-0 py-0">
            Flaky · 3 of 5
          </Chip>
        </div>
      </Stage>

      <Stage label="Draft vs. Live" lit={step === 2}>
        <div className="grid grid-cols-2 gap-2">
          <div className="flex flex-col gap-1 rounded-lg bg-background-secondary-default p-2">
            <div className="flex items-center justify-between gap-1">
              <span className="text-caption-1-medium text-text-primary">Draft</span>
              <Chip variant="caption" color="lime" className="py-0">
                5 of 5
              </Chip>
            </div>
            <span className="inline-flex items-center gap-1 text-caption-1-regular text-text-secondary">
              <RiCheckLine className="size-3.5 shrink-0 text-notification-success-foreground" />
              No refund guaranteed
            </span>
          </div>
          <div className="flex flex-col gap-1 rounded-lg bg-background-secondary-default p-2">
            <div className="flex items-center justify-between gap-1">
              <span className="text-caption-1-medium text-text-primary">Live · v2</span>
              <Chip variant="caption" color="rose" className="py-0">
                1 of 5
              </Chip>
            </div>
            <span className="inline-flex items-center gap-1 text-caption-1-regular text-text-error-primary">
              <RiCloseCircleFill className="size-3.5 shrink-0" />
              Promised $14.20 credit
            </span>
          </div>
        </div>
      </Stage>

      <Stage label="Operator" lit={step === 3}>
        <div className="flex items-center gap-2">
          <RiGitPullRequestLine className="size-3.5 shrink-0 text-foreground-icon-secondary" />
          <span className="min-w-0 flex-1 truncate text-body-2-medium text-text-primary">
            Tighten USPS refund window check
          </span>
          <Chip variant="caption" color="blue" className="shrink-0 py-0">
            Waiting
          </Chip>
        </div>
        <div className="flex items-center justify-between gap-2 pt-0.5">
          <span className="inline-flex items-center gap-1 text-caption-1-regular text-text-secondary">
            <RiSparkling2Line className="size-3.5 text-accent-600 dark:text-accent-400" />
            Draft fix ready to review
          </span>
          <span className="rounded-md bg-linear-to-b from-accent-500 to-accent-600 px-2 py-0.5 text-caption-1-medium text-white">
            Apply
          </span>
        </div>
      </Stage>
    </div>
  );
}
