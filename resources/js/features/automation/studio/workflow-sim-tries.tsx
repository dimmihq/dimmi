/*
 * Operator's tries of a workflow, shown live at the top of Simulations:
 * each one says what the customer wrote and their situation, then plays
 * the chat as it happens, and once done shows the chat and every step
 * the workflow took. A try plays Operator's suggested version when it
 * has one, and is not saved: it is gone once the page is left.
 */
import { RiSparkling2Line } from "@remixicon/react";
import { cx } from "@/utils/cx";
import { card, hint } from "../procedure-simulator/simulator";
import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import type { OperatorTry } from "./use-operator-tries";
import { LiveWfTalk, Trace, WfTalk } from "./workflow-sim-parts";
import type { Steps } from "./workflow-sim-words";

function TryCard({ t, steps, sources }: { t: OperatorTry; steps: Steps; sources: ChipSources }) {
  const result = t.run.result ?? null;

  return (
    <li className={cx("flex flex-col gap-4 p-4", card)}>
      <div className="flex flex-col gap-0.5">
        <span className="flex items-center gap-1.5 text-body-2-medium text-text-primary">
          <RiSparkling2Line
            aria-hidden
            className={cx("size-4 text-foreground-icon-secondary", !t.done && "animate-pulse")}
          />
          {t.done ? "Operator tried" : "Operator is trying"}
        </span>
        {t.situation ? <span className={hint}>{t.situation}</span> : null}
      </div>
      {t.failed ? <p className="text-body-2-regular text-text-error-primary">{t.failed}</p> : null}
      {t.done && result ? (
        <>
          <WfTalk messages={t.message ? [{ text: t.message }] : []} result={result} sources={sources} />
          <Trace trace={result.trace} steps={steps} />
        </>
      ) : null}
      {!t.done ? (
        <>
          <LiveWfTalk run={t.run} sources={sources} />
          {t.run.steps.length > 0 ? <Trace steps={steps} live={t.run.steps} /> : null}
        </>
      ) : null}
    </li>
  );
}

/* OperatorTries lists Operator's tries, the newest first, or nothing when there are none. */
export function OperatorTries({ tries, steps, sources }: { tries: OperatorTry[]; steps: Steps; sources: ChipSources }) {
  if (tries.length === 0) {
    return null;
  }

  return (
    <ol aria-label="Operator's tries" className="mx-4 mt-1 mb-3 flex flex-col gap-3">
      {[...tries].reverse().map((t) => (
        <TryCard key={t.id} t={t} steps={steps} sources={sources} />
      ))}
    </ol>
  );
}
