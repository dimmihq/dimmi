/*
 * The workflow builder's cards stacked the way a run reads top to bottom:
 * a trigger, a condition, an action. The card in play is lit; the page's
 * numbered steps choose which.
 */
import { RiGitBranchLine, RiPlayLine, RiSparkling2Line } from "@remixicon/react";
import type { ReactNode } from "react";

import { cx } from "@/utils/cx";

function Node({ icon, name, lit, children }: { icon: ReactNode; name: string; lit: boolean; children: ReactNode }) {
  return (
    <div
      className={cx(
        "flex w-[280px] flex-col gap-2.5 rounded-xl border bg-background-full p-3 transition-[border-color,box-shadow,opacity] duration-500",
        lit
          ? "border-accent-500 shadow-[0_16px_40px_-24px_rgb(0_0_0/0.3)]"
          : "border-border-button-default opacity-60 shadow-xs",
      )}
    >
      <div className="flex items-center gap-2 text-body-medium text-text-primary">
        <span
          className={cx(
            "flex size-6 items-center justify-center rounded-md transition-colors duration-500",
            lit ? "bg-accent-600 text-text-white" : "bg-background-secondary-default text-foreground-icon-secondary",
          )}
        >
          {icon}
        </span>
        {name}
      </div>
      <div className="rounded-lg border border-separator-border bg-background-primary-default px-2.5 py-2 text-caption-1-regular text-text-secondary">
        {children}
      </div>
    </div>
  );
}

function Pill({ children }: { children: ReactNode }) {
  return (
    <span className="rounded border border-separator-border bg-background-full px-1.5 py-0.5 text-caption-1-medium text-text-primary">
      {children}
    </span>
  );
}

function Link() {
  return <span className="h-7 w-px bg-border-button-hover" />;
}

export function WorkflowStack({ step }: { step: number }) {
  return (
    <div className="flex flex-col items-center" aria-hidden>
      <Node icon={<RiPlayLine className="size-3.5" />} name="Trigger" lit={step === 0}>
        Customer sends their first message
      </Node>
      <Link />
      <Node icon={<RiGitBranchLine className="size-3.5" />} name="Branch" lit={step === 1}>
        <span className="flex flex-wrap items-center gap-1.5">
          If <Pill>Plan</Pill> is <Pill>Enterprise</Pill> and <Pill>Topic</Pill> is <Pill>Outage</Pill>
        </span>
      </Node>
      <Link />
      <Node icon={<RiSparkling2Line className="size-3.5" />} name="Let dimmi handle" lit={step === 2}>
        Check for a known incident. If there is one, reply with its status. If not, page the on-call team.
      </Node>
    </div>
  );
}
