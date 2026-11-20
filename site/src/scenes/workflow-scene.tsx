/*
 * A workflow drawn the way the builder draws one, and run: a customer's
 * first message arrives, the workflow sorts it by topic, and one of three
 * paths takes it. Each loop a different topic comes in, so every path
 * lights in turn. The step names are the builder's own.
 */
import {
  type RemixiconComponentType,
  RiCheckLine,
  RiMessage2Line,
  RiSignpostLine,
  RiSparkling2Line,
  RiTicket2Line,
  RiUserShared2Line,
} from "@remixicon/react";
import { useEffect, useState } from "react";

import { cx } from "@/utils/cx";

import { useScene } from "./use-scene";

const W = 700;
const H = 316;

const paths = [
  { topic: "Billing", name: "Assign", detail: "Billing team", icon: RiUserShared2Line, y: 14 },
  { topic: "Bug", name: "Create a ticket", detail: "Engineering tracker", icon: RiTicket2Line, y: 122 },
  {
    topic: "Anything else",
    name: "Let dimmi handle",
    detail: "Answers from your knowledge",
    icon: RiSparkling2Line,
    y: 230,
  },
];

const messages = ["We were charged twice", "Export fails with a 500", "How do I invite my team?"];

function Node({
  label,
  name,
  detail,
  icon: Icon,
  lit,
  done,
  className,
  style,
}: {
  label: string;
  name: string;
  detail?: string;
  icon: RemixiconComponentType;
  lit: boolean;
  done?: boolean;
  className?: string;
  style?: React.CSSProperties;
}) {
  return (
    <div
      style={style}
      className={cx(
        "absolute flex items-center gap-3 rounded-xl border bg-background-primary-default px-3 py-2.5 transition-[border-color,box-shadow,opacity] duration-500",
        lit
          ? "border-accent-500 shadow-[0_0_0_4px_color-mix(in_oklab,var(--color-accent-500)_14%,transparent)]"
          : "border-border-button-default shadow-xs",
        className,
      )}
    >
      <span
        className={cx(
          "flex size-8 shrink-0 items-center justify-center rounded-lg transition-colors duration-500",
          lit ? "bg-accent-600 text-text-white" : "bg-background-secondary-default text-foreground-icon-secondary",
        )}
      >
        {done ? <RiCheckLine className="size-4" /> : <Icon className="size-4" />}
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="font-mono text-[10px] tracking-[0.12em] text-text-tertiary uppercase">{label}</span>
        <span className="truncate text-body-medium text-text-primary">{name}</span>
        {detail ? <span className="truncate text-caption-1-regular text-text-secondary">{detail}</span> : null}
      </span>
    </div>
  );
}

function Wire({ d, lit }: { d: string; lit: boolean }) {
  return (
    <path
      d={d}
      fill="none"
      strokeWidth="1.5"
      strokeLinecap="round"
      className={cx(
        "transition-[stroke] duration-500",
        lit ? "site-flow stroke-accent-500" : "stroke-border-button-hover",
      )}
    />
  );
}

export function WorkflowScene() {
  const { ref, step } = useScene(4, { interval: [1300, 1300, 1500], hold: 2600 });
  const [round, setRound] = useState(0);

  // Each time the scene rewinds, the next topic comes in.
  useEffect(() => {
    if (step === 0) {
      setRound((r) => r + 1);
    }
  }, [step]);

  const taken = round % paths.length;

  return (
    <div ref={ref} className="site-fit" style={{ "--fit-w": `${W}px`, "--fit-h": `${H}px` } as React.CSSProperties}>
      <div className="relative mx-auto" style={{ width: W, height: H }}>
        <svg viewBox={`0 0 ${W} ${H}`} className="absolute inset-0 size-full" aria-hidden>
          <Wire d="M214 158 H 262" lit={step >= 1} />
          {paths.map((path, i) => (
            <Wire
              key={path.topic}
              d={`M420 158 C 448 158, 440 ${path.y + 36}, 468 ${path.y + 36}`}
              lit={step >= 2 && taken === i}
            />
          ))}
        </svg>
        <Node
          label="Trigger"
          name="First message"
          detail={`“${messages[taken]}”`}
          icon={RiMessage2Line}
          lit={step >= 0}
          style={{ left: 0, top: 122, width: 214 }}
        />
        <Node
          label="Step"
          name="Sort by topic"
          icon={RiSignpostLine}
          lit={step >= 1}
          style={{ left: 262, top: 129, width: 158 }}
        />
        {paths.map((path, i) => (
          <Node
            key={path.topic}
            label={path.topic}
            name={path.name}
            detail={path.detail}
            icon={path.icon}
            lit={step >= 2 && taken === i}
            done={step >= 3 && taken === i}
            className={cx(step >= 2 && taken !== i && "opacity-55")}
            style={{ left: 468, top: path.y, width: 232 }}
          />
        ))}
      </div>
    </div>
  );
}
