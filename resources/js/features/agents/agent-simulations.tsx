/*
 * An agent's simulations, the way a procedure's work: chats the agent
 * must keep getting right, each with the checks that judge it, run
 * against the draft or the live version. The list says how many pass;
 * opening one slides in its conversation and checks. Mockup: sample
 * simulations, and Run only pretends.
 */
import {
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiCheckboxBlankCircleLine,
  RiCheckboxCircleFill,
  RiCloseCircleFill,
  RiCloseLine,
  RiDeleteBinLine,
  RiPlayFill,
} from "@remixicon/react";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { toast } from "@/components/base/toast/toast";
import { MessageBody } from "@/features/inbox/message-body";
import { Swap } from "@/features/inbox/motion";
import { ScrollArea } from "@/features/shell/page";
import { cx } from "@/utils/cx";

import { card, fieldLabel, fromCustomer, hint } from "../automation/procedure-simulator/simulator";
import { type Simulation, simulations } from "./mock";

const look = {
  pass: { color: "lime", label: "Passed" },
  fail: { color: "rose", label: "Failed" },
  unrun: { color: "neutral", label: "Not run" },
} as const;

function Mark({ result }: { result: "pass" | "fail" | "unrun" }) {
  if (result === "unrun") {
    return (
      <RiCheckboxBlankCircleLine aria-label="Not run" className="size-4.5 shrink-0 text-foreground-icon-quaternary" />
    );
  }

  return result === "pass" ? (
    <RiCheckboxCircleFill aria-label="Passed" className="size-4.5 shrink-0 text-notification-success-foreground" />
  ) : (
    <RiCloseCircleFill aria-label="Failed" className="size-4.5 shrink-0 text-foreground-icon-error" />
  );
}

/*
 * Checks are what judge a simulation, each with how it went on the last
 * run and why when it failed. A teammate removes one or adds one in
 * their own words; a model judges words, so "transfers to dimmi" works.
 */
function Checks({ sim, result }: { sim: Simulation; result: Simulation["draft"] }) {
  const [checks, setChecks] = useState(sim.checks);
  const [adding, setAdding] = useState("");
  const add = () => {
    const text = adding.trim();
    if (text) {
      setChecks([...checks, { text, passedOnFail: true, fresh: true }]);
      setAdding("");
    }
  };
  const outcome = (c: (typeof checks)[number]) => {
    if (result === "unrun" || c.fresh) {
      return "unrun";
    }

    return result === "fail" && !c.passedOnFail ? "fail" : "pass";
  };

  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-col">
        <span className={fieldLabel}>Checks</span>
        <span className={hint}>The simulation passes when all of these are true.</span>
      </div>
      <ul className="flex flex-col gap-1">
        {checks.map((c) => (
          <li key={c.text} className="flex items-start gap-2">
            <span className="flex h-7 items-center">
              <Mark result={outcome(c)} />
            </span>
            <span className="flex min-w-0 flex-1 flex-col py-1">
              <span className="text-body-2-regular text-text-primary">{c.text}</span>
              {outcome(c) === "fail" && c.why ? (
                <span className="text-body-2-regular text-text-error-primary">{c.why}</span>
              ) : null}
            </span>
            <IconButton
              variant="quiet"
              size="small"
              icon={RiCloseLine}
              aria-label="Remove check"
              onClick={() => setChecks(checks.filter((other) => other !== c))}
            />
          </li>
        ))}
      </ul>
      <div className="flex items-center gap-2">
        <Input
          aria-label="Add a check"
          size="small"
          placeholder="Add a check: the agent should…"
          className="min-w-0 flex-1"
          value={adding}
          onChange={setAdding}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              add();
            }
          }}
        />
        <Button variant="secondary" size="small" disabled={!adding.trim()} onClick={add}>
          Add
        </Button>
      </div>
    </div>
  );
}

function Detail({ sim, version, onBack }: { sim: Simulation; version: string; onBack: () => void }) {
  const result = version === "live" ? sim.live : sim.draft;
  const status = look[result];

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2.5 ps-3 pe-3">
        <IconButton
          variant="quiet"
          size="small"
          icon={RiArrowLeftSLine}
          aria-label="Back to simulations"
          onClick={onBack}
        />
        <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{sim.name}</span>
        <span className="flex shrink-0 items-center gap-1">
          <IconButton variant="quiet" size="small" icon={RiDeleteBinLine} aria-label={`Delete ${sim.name}`} />
          <Button
            variant="tertiary"
            size="xs"
            leadingIcon={RiPlayFill}
            onClick={() => toast.success("Ran 1 simulation")}
          >
            Run
          </Button>
        </span>
      </div>
      <ScrollArea>
        <div className="flex flex-col gap-6 px-4 pt-2 pb-10">
          <span className="flex items-center gap-2">
            <Chip variant="caption" color={status.color} className="py-0.5">
              {status.label}
            </Chip>
            <span className={hint}>{sim.source}</span>
          </span>
          <div className="flex flex-col gap-2.5">
            <span className={fieldLabel}>Conversation</span>
            <ol aria-label="Simulated conversation" className="flex flex-col gap-5">
              {sim.turns.map((t) => (
                <li key={t.customer} className="flex flex-col gap-5">
                  <p className={fromCustomer}>{t.customer}</p>
                  <MessageBody text={t.reply} className="text-body-regular text-text-primary" />
                </li>
              ))}
            </ol>
          </div>
          <Checks sim={sim} result={result} />
        </div>
      </ScrollArea>
    </div>
  );
}

export function AgentSimulations({ version }: { version: string }) {
  const [openId, setOpenId] = useState<string | null>(null);
  const opened = simulations.find((s) => s.id === openId);
  const passing = simulations.filter((s) => (version === "live" ? s.live : s.draft) === "pass").length;

  const all = (
    <>
      <div className="flex h-11 shrink-0 items-center justify-between gap-2 ps-4 pe-3">
        <p className={cx(hint, "min-w-0 truncate tabular-nums")}>
          {passing} of {simulations.length} passing on the {version === "live" ? "live version" : "draft"}
        </p>
        <span className="flex shrink-0 items-center gap-1">
          <Button
            variant="tertiary"
            size="xs"
            leadingIcon={RiPlayFill}
            onClick={() => toast.success(`Ran ${simulations.length} simulations`)}
          >
            Run all
          </Button>
        </span>
      </div>
      <ScrollArea>
        <ul className={cx("mx-4 mt-1 mb-10 flex flex-col gap-0.5 p-1", card)}>
          {simulations.map((sim) => {
            const status = look[version === "live" ? sim.live : sim.draft];

            return (
              <li key={sim.id}>
                <button
                  type="button"
                  onClick={() => setOpenId(sim.id)}
                  className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                >
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="flex min-w-0 items-center gap-1.5">
                      <span className="truncate text-body-regular text-text-primary">{sim.name}</span>
                    </span>
                    <span className={cx(hint, "truncate")}>{sim.turns[0]?.customer}</span>
                  </span>
                  <Chip variant="caption" color={status.color} className="shrink-0 py-0.5">
                    {status.label}
                  </Chip>
                  <RiArrowRightSLine
                    aria-hidden
                    className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
                  />
                </button>
              </li>
            );
          })}
        </ul>
      </ScrollArea>
    </>
  );
  const detail = opened ? <Detail sim={opened} version={version} onBack={() => setOpenId(null)} /> : null;

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
      <Swap view={opened ? opened.id : "list"} forward={opened !== undefined} className="flex min-h-0 flex-1 flex-col">
        {detail ?? all}
      </Swap>
    </div>
  );
}
