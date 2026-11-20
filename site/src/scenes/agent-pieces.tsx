/*
 * The pieces the agent stories are drawn with, from the app's own parts:
 * message bubbles, the agent-thinking indicator, a procedure card.
 */
import { type RemixiconComponentType, RiCheckLine } from "@remixicon/react";
import { AnimatePresence, motion } from "motion/react";
import type { ReactNode } from "react";

import { AgentThinking } from "@/components/application/agent-thinking/agent-thinking";
import { cx } from "@/utils/cx";

import { enter } from "./motion";

/*
 * Bubbles drawn the way the messenger widget draws them
 * (resources/widget/styles/bubbles.css): 22px corners all round, 13 by 18
 * padding, 14/20 type, at most 85% wide, and one quiet byline under the
 * bubble. The customer sits on the right in the brand gradient; the agent
 * sits on the left in grey.
 */
const bubble =
  "max-w-full rounded-[22px] px-[18px] py-[13px] text-[14px] leading-5 whitespace-pre-wrap [overflow-wrap:anywhere]";
const byline =
  "flex flex-wrap gap-[5px] px-1 text-[12px] leading-4 text-text-tertiary [&>*+*]:before:me-[5px] [&>*+*]:before:content-['·']";

export function Customer({ name, children }: { name: string; initials?: string; children: ReactNode }) {
  return (
    <motion.div {...enter} className="ms-auto grid max-w-[85%] justify-items-end gap-0.5">
      <div className={cx(bubble, "bg-linear-to-b from-accent-500 to-accent-600 text-white")}>{children}</div>
      <p className={byline}>
        <span>{name}</span>
      </p>
    </motion.div>
  );
}

export function Agent({ children, footer }: { children: ReactNode; footer?: ReactNode }) {
  return (
    <motion.div {...enter} className="me-auto grid max-w-[85%] justify-items-start gap-0.5">
      <div className={cx(bubble, "bg-background-tertiary-default text-text-primary")}>
        {children}
        {footer ? <span className="mt-2.5 flex flex-wrap gap-1.5 whitespace-normal">{footer}</span> : null}
      </div>
      <p className={byline}>
        <span>dimmi</span>
        <span>AI</span>
      </p>
    </motion.div>
  );
}

/* A source under an answer, as the widget's .source chip. */
export function Source({ icon: Icon, children }: { icon: RemixiconComponentType; children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-border-button-default bg-background-full py-1 ps-2 pe-2.5 text-[12px] leading-4 font-medium text-text-primary">
      <Icon className="size-3.5 text-accent-600 dark:text-accent-400" aria-hidden />
      {children}
    </span>
  );
}

export function Thinking({ label }: { label: string }) {
  return (
    <motion.div {...enter} className="flex justify-start">
      <AgentThinking variant="wave" label={label} shimmer tone="subtle" />
    </motion.div>
  );
}

export type ProcedureStep = { label: string; detail?: string; state: "done" | "waiting" | "approved" };

export function Procedure({ name, steps }: { name: string; steps: ProcedureStep[] }) {
  return (
    <motion.div
      {...enter}
      className="me-auto flex w-[86%] flex-col gap-0 overflow-hidden rounded-xl border border-separator-border bg-background-primary-default shadow-xs"
    >
      <div className="flex items-center gap-2 border-b border-separator-border px-3.5 py-2.5">
        <span className="text-caption-1-medium text-text-tertiary">Procedure</span>
        <span className="text-body-medium text-text-primary">{name}</span>
      </div>
      <ol className="flex flex-col gap-2.5 px-3.5 py-3">
        <AnimatePresence initial={false}>
          {steps.map((step) => (
            <motion.li key={step.label} {...enter} className="flex items-start gap-2.5">
              <span
                className={cx(
                  "mt-0.5 flex size-[18px] shrink-0 items-center justify-center rounded-full",
                  step.state === "waiting"
                    ? "border border-dashed border-border-button-hover"
                    : "bg-accent-500 text-text-white",
                )}
              >
                {step.state === "waiting" ? null : <RiCheckLine className="size-3" aria-hidden />}
              </span>
              <span className="flex min-w-0 flex-col">
                <span className="text-body-medium text-text-primary">{step.label}</span>
                {step.detail ? <span className="text-caption-1-regular text-text-secondary">{step.detail}</span> : null}
              </span>
            </motion.li>
          ))}
        </AnimatePresence>
      </ol>
    </motion.div>
  );
}
