/*
 * What Operator did to reach an answer, as a log that draws itself: each
 * step lands under the last on a curved guide while Operator works, with
 * a shimmering header and a thinking line at the tail. Once the answer is
 * in, the log folds behind one line that counts the steps, so the answer
 * stays the main event and the work is a click away.
 */
import {
  RiArrowDownSLine,
  RiBarChartLine,
  RiBookOpenLine,
  RiChat3Line,
  RiEditLine,
  RiErrorWarningLine,
  RiFlashlightLine,
  RiListCheck3,
  RiSearchLine,
  RiUserLine,
} from "@remixicon/react";
import { AnimatePresence, motion } from "motion/react";
import { type ComponentType, useState } from "react";

import { LogRow, ShimmerText, useLogMotion, WorkingRow } from "@/components/application/agent-log/agent-log";
import type { OperatorStep } from "@/lib/api/operator";
import { cx } from "@/utils/cx";

type Icon = ComponentType<{ className?: string }>;

/*
 * toolIcons pairs each kind of tool with the glyph a row shows, matched
 * by the start of the tool's name.
 */
const toolIcons: [string, Icon][] = [
  ["search_", RiSearchLine],
  ["workspace_report", RiBarChartLine],
  ["agent_runs", RiBarChartLine],
  ["get_conversation", RiChat3Line],
  ["get_contact", RiUserLine],
  ["list_teammates", RiUserLine],
  ["get_article", RiBookOpenLine],
  ["list_articles", RiBookOpenLine],
  ["propose_", RiEditLine],
  ["draft_", RiEditLine],
  ["remember", RiEditLine],
  ["action_", RiFlashlightLine],
];

function iconFor(step: OperatorStep): Icon {
  if (!step.ok) {
    return RiErrorWarningLine;
  }

  return toolIcons.find(([prefix]) => step.tool.startsWith(prefix))?.[1] ?? RiListCheck3;
}

function Row({ step, first, last, reduce }: { step: OperatorStep; first: boolean; last: boolean; reduce: boolean }) {
  const Glyph = iconFor(step);

  return (
    <LogRow first={first} last={last} reduce={reduce}>
      <span className="flex min-w-0 items-center gap-1.5 py-1 text-body-2-regular text-text-secondary">
        <Glyph
          aria-hidden
          className={cx("size-3.5 shrink-0", step.ok ? "text-foreground-icon-tertiary" : "text-text-error-primary")}
        />
        <span className="min-w-0 truncate">{step.summary}</span>
      </span>
    </LogRow>
  );
}

/*
 * keyed gives each step a key that stays put as steps are added: its tool
 * and summary, counted when the same pair repeats.
 */
function keyed(steps: OperatorStep[]) {
  const seen = new Map<string, number>();

  return steps.map((step) => {
    const base = `${step.tool}:${step.summary}`;
    const count = (seen.get(base) ?? 0) + 1;
    seen.set(base, count);

    return { key: `${base}:${count}`, step };
  });
}

function headerLabel(steps: number, working: boolean) {
  if (working) {
    return steps === 0 ? "Reading your question" : "Working";
  }

  return steps === 1 ? "Worked through 1 step" : `Worked through ${steps} steps`;
}

export function ActivityLog({ steps, working }: { steps: OperatorStep[]; working: boolean }) {
  const reduce = useLogMotion();
  const [opened, setOpened] = useState<boolean | null>(null);
  const open = opened ?? working;
  if (steps.length === 0 && !working) {
    return null;
  }
  if (steps.length === 0) {
    return <WorkingRow label="Thinking" reduce={reduce} />;
  }
  const label = headerLabel(steps.length, working);

  return (
    <section aria-label="Steps" className="flex w-full flex-col">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpened(!open)}
        className="group flex w-full cursor-pointer items-center gap-2 rounded-md py-0.5 text-start outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiListCheck3 aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="min-w-0 flex-1 text-body-medium text-text-secondary">
          {working ? <ShimmerText>{label}</ShimmerText> : <span>{label}</span>}
        </span>
        <RiArrowDownSLine
          aria-hidden
          className={cx(
            "size-4 shrink-0 text-foreground-icon-tertiary transition-transform duration-300 ease group-hover:text-foreground-icon-secondary",
            open && "rotate-180",
          )}
        />
      </button>
      <AnimatePresence initial={false}>
        {open ? (
          <motion.div
            key="steps"
            initial={reduce ? false : { height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={reduce ? { opacity: 0 } : { height: 0, opacity: 0 }}
            transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
            className="overflow-hidden"
          >
            <ul className="ms-2 mt-0.5 flex flex-col">
              {keyed(steps).map(({ key, step }, index) => (
                <Row key={key} step={step} first={index === 0} last={index === steps.length - 1} reduce={reduce} />
              ))}
            </ul>
            {working ? <WorkingRow label="Thinking" reduce={reduce} className="ms-2" /> : null}
          </motion.div>
        ) : null}
      </AnimatePresence>
    </section>
  );
}
