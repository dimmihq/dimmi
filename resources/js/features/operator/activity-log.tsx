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
  RiRobot2Line,
  RiSearchLine,
  RiUserLine,
} from "@remixicon/react";
import { AnimatePresence, motion } from "motion/react";
import { type ComponentType, useState } from "react";

import { LogRow, ShimmerText, useLogMotion, WorkingRow } from "@/components/application/agent-log/agent-log";
import type { OperatorStep } from "@/lib/api/operator";
import { cx } from "@/utils/cx";

import { grouped, type LogItem, type SubagentGroup, stepCount } from "./grouped-steps";

type Icon = ComponentType<{ className?: string }>;

/*
 * toolIcons pairs each kind of tool with the glyph a row shows, matched
 * by the start of the tool's name.
 */
const toolIcons: [string, Icon][] = [
  ["search_", RiSearchLine],
  ["workspace_report", RiBarChartLine],
  ["get_workspace_report", RiBarChartLine],
  ["agent_runs", RiBarChartLine],
  ["get_ai_agent_runs", RiBarChartLine],
  ["get_conversation", RiChat3Line],
  ["get_contact", RiUserLine],
  ["get_customer", RiUserLine],
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

/*
 * Note is something Operator said while it worked, shown whole where it
 * was said so the log reads in the order things happened.
 */
function Note({ step, first, last, reduce }: { step: OperatorStep; first: boolean; last: boolean; reduce: boolean }) {
  return (
    <LogRow first={first} last={last} reduce={reduce}>
      <span className="flex min-w-0 items-start gap-1.5 py-1 text-body-2-regular text-text-tertiary">
        <RiChat3Line aria-hidden className="mt-0.5 size-3.5 shrink-0 text-foreground-icon-tertiary" />
        <span className="min-w-0 italic">{step.summary}</span>
      </span>
    </LogRow>
  );
}

function Row({ step, first, last, reduce }: { step: OperatorStep; first: boolean; last: boolean; reduce: boolean }) {
  const Glyph = iconFor(step);
  if (step.tool === "note") {
    return <Note step={step} first={first} last={last} reduce={reduce} />;
  }

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
 * Subagent is one subagent's part of a run: a line that names the part
 * and says how far it is, opening to the steps it took and its report.
 */
function Subagent({
  group,
  working,
  first,
  last,
  reduce,
}: {
  group: SubagentGroup;
  working: boolean;
  first: boolean;
  last: boolean;
  reduce: boolean;
}) {
  const [open, setOpen] = useState(false);
  const busy = working && group.report === null;
  const count = group.steps.length === 1 ? "1 step" : `${group.steps.length} steps`;
  const progress = busy ? `working, ${count}` : count;
  const rows = keyed(group.steps.map((step) => ({ kind: "step", step }) as LogItem));

  return (
    <LogRow first={first} last={last} reduce={reduce}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="group/sub flex w-full min-w-0 cursor-pointer items-center gap-1.5 rounded-md py-1 text-start text-body-2-regular text-text-secondary outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiRobot2Line
          aria-hidden
          className={cx("size-3.5 shrink-0", group.ok ? "text-foreground-icon-tertiary" : "text-text-error-primary")}
        />
        <span className="min-w-0 truncate text-body-2-medium">
          {busy ? <ShimmerText>{group.title}</ShimmerText> : group.title}
        </span>
        <span className="shrink-0 text-text-tertiary">· {progress}</span>
        <RiArrowDownSLine
          aria-hidden
          className={cx(
            "size-3.5 shrink-0 text-foreground-icon-tertiary transition-transform duration-300 ease group-hover/sub:text-foreground-icon-secondary",
            open && "rotate-180",
          )}
        />
      </button>
      {open ? (
        <ul className="ms-1 flex flex-col pb-1">
          {rows.map(({ key, item }, index) =>
            item.kind === "step" ? (
              <Row
                key={key}
                step={item.step}
                first={index === 0}
                last={index === rows.length - 1 && group.report === null}
                reduce={reduce}
              />
            ) : null,
          )}
          {group.report !== null ? (
            <Note
              step={{ tool: "note", summary: group.report, ok: group.ok }}
              first={rows.length === 0}
              last
              reduce={reduce}
            />
          ) : null}
        </ul>
      ) : null}
    </LogRow>
  );
}

/*
 * keyed gives each line a key that stays put as lines are added: its tool
 * and summary, counted when the same pair repeats, or its subagent.
 */
function keyed(items: LogItem[]) {
  const seen = new Map<string, number>();

  return items.map((item) => {
    if (item.kind === "subagent") {
      return { key: `subagent:${item.group.subagent}`, item };
    }
    const base = `${item.step.tool}:${item.step.summary}`;
    const count = (seen.get(base) ?? 0) + 1;
    seen.set(base, count);

    return { key: `${base}:${count}`, item };
  });
}

function headerLabel(steps: number, working: boolean) {
  if (working) {
    return steps === 0 ? "Reading your question" : "Working";
  }

  return steps === 1 ? "Worked through 1 step" : `Worked through ${steps} steps`;
}

export function ActivityLog({ steps: all, working }: { steps: OperatorStep[]; working: boolean }) {
  const items = grouped(all);
  const steps = stepCount(all);
  const reduce = useLogMotion();
  const [opened, setOpened] = useState<boolean | null>(null);
  const open = opened ?? working;
  if (items.length === 0 && !working) {
    return null;
  }
  if (items.length === 0) {
    return <WorkingRow label="Thinking" reduce={reduce} />;
  }
  const label = headerLabel(steps, working);

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
              {keyed(items).map(({ key, item }, index) =>
                item.kind === "step" ? (
                  <Row
                    key={key}
                    step={item.step}
                    first={index === 0}
                    last={index === items.length - 1}
                    reduce={reduce}
                  />
                ) : (
                  <Subagent
                    key={key}
                    group={item.group}
                    working={working}
                    first={index === 0}
                    last={index === items.length - 1}
                    reduce={reduce}
                  />
                ),
              )}
            </ul>
            {working ? <WorkingRow label="Thinking" reduce={reduce} className="ms-2" /> : null}
          </motion.div>
        ) : null}
      </AnimatePresence>
    </section>
  );
}
