/*
 * A transfer between AI agents, as the team reads it in the thread:
 * which agent handed the conversation to which, when, and why, in the
 * words the first agent left for the team. The customer never saw it.
 */
import { RiArrowLeftRightLine } from "@remixicon/react";

import { KindMark } from "@/features/automation/parts";

import { When } from "./when";

export function TransferCard({ from, to, why, iso }: { from: string; to: string; why: string; iso?: string }) {
  return (
    <article
      aria-label={`${from} transferred the conversation to ${to}`}
      className="mx-auto flex w-full max-w-[520px] flex-col gap-2.5 rounded-[var(--radius-2-5xl)] bg-background-primary-default p-3 shadow-card"
    >
      <header className="flex items-start gap-2.5">
        <KindMark icon={RiArrowLeftRightLine} tint="purple" />
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="text-body-medium text-text-primary">
            {from} transferred the conversation to {to}
          </span>
          <span className="flex flex-wrap items-center gap-1 text-body-2-regular text-text-secondary">
            The customer was not told
            {iso ? (
              <>
                <span aria-hidden>·</span>
                <When iso={iso} />
              </>
            ) : null}
          </span>
        </div>
      </header>
      {why ? (
        <dl className="grid grid-cols-[minmax(0,auto)_minmax(0,1fr)] gap-x-4 rounded-xl bg-background-secondary-default p-3">
          <dt className="text-body-2-regular text-text-secondary">Why</dt>
          <dd className="min-w-0 break-words text-body-2-medium text-text-primary">{why}</dd>
        </dl>
      ) : null}
    </article>
  );
}
