/*
 * A change Operator proposed, as a card the teammate decides on: what it
 * does and why, each operation in plain words, and what became of it.
 * Waiting changes apply or are declined in one click; applied ones can
 * be undone, which puts back every value Operator replaced unless
 * someone changed it since. A change that failed says why and can be
 * tried again.
 */
import { RiArrowGoBackLine, RiCheckLine, RiCloseLine, RiErrorWarningLine } from "@remixicon/react";
import { useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { MessageBody } from "@/features/inbox/message-body";
import type { ChangesetDecision, ChangesetStatus, OperatorChangeset } from "@/lib/api/operator";
import { cx } from "@/utils/cx";

import { failureText } from "./use-operator-chat";

const statusLabels: Record<ChangesetStatus, string> = {
  pending: "Waiting for you",
  applied: "Applied",
  declined: "Declined",
  reverted: "Undone",
  failed: "Not applied",
};

const statusColors: Record<ChangesetStatus, ChipColor> = {
  pending: "blue",
  applied: "lime",
  declined: "neutral",
  reverted: "neutral",
  failed: "rose",
};

const outcomeLabels: Record<string, string> = {
  reverted: "undone",
  skipped: "kept: changed since",
};

/*
 * FullText shows what an operation writes, folded until asked for:
 * Markdown as the reader will see it, a procedure as its JSON.
 */
function FullText({ kind, text }: { kind: string; text: string }) {
  return (
    <details>
      <summary className="cursor-pointer list-none text-caption-1-medium text-text-tertiary outline-none hover:text-text-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring">
        Show the text
      </summary>
      <div className="mt-1.5 max-h-72 overflow-y-auto rounded-xl border border-separator-border bg-background-primary-default p-2.5">
        {kind.startsWith("procedure.") ? (
          <pre className="text-caption-1-regular whitespace-pre-wrap text-text-secondary">{text}</pre>
        ) : (
          <MessageBody text={text} className="text-body-2-regular text-text-primary" />
        )}
      </div>
    </details>
  );
}

export function ChangesetCard({
  changeset,
  onDecide,
}: {
  changeset: OperatorChangeset;
  onDecide: (id: string, decision: ChangesetDecision) => Promise<void>;
}) {
  const [pending, setPending] = useState<ChangesetDecision | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const decide = async (decision: ChangesetDecision) => {
    setPending(decision);
    setFailure(null);
    try {
      await onDecide(changeset.id, decision);
    } catch (error) {
      setFailure(failureText(error));
    } finally {
      setPending(null);
    }
  };
  const open = changeset.status === "pending" || changeset.status === "failed";

  return (
    <section
      aria-label={changeset.title}
      className="flex flex-col gap-3 rounded-2xl border border-border-button-default bg-background-secondary-default p-3"
    >
      <header className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 flex-col gap-0.5">
          <h3 className="text-body-medium text-text-primary">{changeset.title}</h3>
          {changeset.reason ? <p className="text-body-2-regular text-text-secondary">{changeset.reason}</p> : null}
        </div>
        <Chip variant="caption" color={statusColors[changeset.status]} className="shrink-0 py-0">
          {changeset.auto && changeset.status === "applied" ? "Applied on its own" : statusLabels[changeset.status]}
        </Chip>
      </header>
      <ul className="flex flex-col gap-1.5">
        {changeset.operations.map((operation) => (
          <li
            key={`${operation.kind}:${operation.label}`}
            className="flex items-start gap-2 text-body-2-regular text-text-primary"
          >
            <RiCheckLine
              aria-hidden
              className={cx(
                "mt-0.5 size-3.5 shrink-0",
                operation.outcome === "applied" ? "text-foreground-icon-primary" : "text-foreground-icon-quaternary",
              )}
            />
            <span className="flex min-w-0 flex-col gap-1">
              <span>
                {operation.label}
                {operation.kind === "action.run" ? (
                  <span className="text-text-tertiary"> · cannot be undone</span>
                ) : operation.outcome && outcomeLabels[operation.outcome] ? (
                  <span className="text-text-tertiary"> · {outcomeLabels[operation.outcome]}</span>
                ) : null}
              </span>
              {operation.text ? <FullText kind={operation.kind} text={operation.text} /> : null}
            </span>
          </li>
        ))}
      </ul>
      {changeset.error || failure ? (
        <p className="flex items-start gap-1.5 text-body-2-regular text-text-error-primary">
          <RiErrorWarningLine aria-hidden className="mt-0.5 size-3.5 shrink-0" />
          {failure ?? changeset.error}
        </p>
      ) : null}
      {open ? (
        <div className="flex gap-2">
          <Button size="small" loading={pending === "applications"} onClick={() => decide("applications")}>
            {changeset.status === "failed" ? "Try again" : "Apply"}
          </Button>
          <Button
            size="small"
            variant="secondary"
            leadingIcon={RiCloseLine}
            loading={pending === "declines"}
            onClick={() => decide("declines")}
          >
            Decline
          </Button>
        </div>
      ) : null}
      {changeset.status === "applied" && changeset.operations.some((op) => op.kind !== "action.run") ? (
        <Button
          size="small"
          variant="secondary"
          className="self-start"
          leadingIcon={RiArrowGoBackLine}
          loading={pending === "reversions"}
          onClick={() => decide("reversions")}
        >
          Undo
        </Button>
      ) : null}
    </section>
  );
}
