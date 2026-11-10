/*
 * A change Operator proposed, as a card the teammate decides on: what it
 * does and why, each operation in plain words, and what became of it.
 * Waiting changes apply or are declined in one click; applied ones can
 * be undone, which puts back every value Operator replaced unless
 * someone changed it since. A change that failed says why and can be
 * tried again.
 */
import {
  RiArrowDownSLine,
  RiArrowGoBackLine,
  RiCheckLine,
  RiCloseLine,
  RiErrorWarningLine,
  RiGitPullRequestLine,
} from "@remixicon/react";
import { useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { MessageBody } from "@/features/inbox/message-body";
import { Unfold } from "@/features/shell/reveal";
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
 * asJSON gives the text as indented JSON when it is JSON, such as an
 * action's arguments or a procedure, or null when it is prose.
 */
function asJSON(text: string): string | null {
  const trimmed = text.trim();
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) {
    return null;
  }
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return null;
  }
}

/*
 * FullText shows what an operation writes, folded until asked for and
 * unfolding in place: Markdown as the reader will see it, and JSON, such
 * as an action's arguments or a procedure, as highlighted code.
 */
function FullText({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  const json = asJSON(text);

  return (
    <div className="flex flex-col">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex w-fit cursor-pointer items-center gap-0.5 rounded-md text-caption-1-medium text-text-tertiary outline-none transition-colors duration-150 hover:text-text-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        {open ? "Hide the text" : "Show the text"}
        <RiArrowDownSLine
          aria-hidden
          className={cx("size-3.5 transition-transform duration-300 ease", open && "rotate-180")}
        />
      </button>
      <Unfold open={open}>
        <div className="pt-1.5">
          {json ? (
            <MessageBody text={`\`\`\`json\n${json}\n\`\`\``} className="text-caption-1-regular" />
          ) : (
            <div className="max-h-72 overflow-y-auto rounded-xl border border-separator-border bg-background-secondary-default p-2.5">
              <MessageBody text={text} className="text-body-2-regular text-text-primary" />
            </div>
          )}
        </div>
      </Unfold>
    </div>
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
  return (
    <section
      aria-label={changeset.title}
      className="flex flex-col overflow-hidden rounded-2xl border border-border-button-default bg-background-primary-default shadow-xs"
    >
      <header className="flex min-h-10 items-center gap-2 border-b border-separator-border ps-3 pe-2 py-2">
        <RiGitPullRequestLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <h3 className="min-w-0 flex-1 truncate text-body-2-medium text-text-primary">{changeset.title}</h3>
        <Chip variant="caption" color={statusColors[changeset.status]} className="shrink-0 py-0">
          {changeset.auto && changeset.status === "applied" ? "Applied on its own" : statusLabels[changeset.status]}
        </Chip>
      </header>
      <div className="flex flex-col gap-2.5 px-3 py-2.5">
        {changeset.reason ? <p className="text-body-2-regular text-text-secondary">{changeset.reason}</p> : null}
        <Operations changeset={changeset} />
        {changeset.error || failure ? (
          <p className="flex items-start gap-1.5 text-body-2-regular text-text-error-primary">
            <RiErrorWarningLine aria-hidden className="mt-0.5 size-3.5 shrink-0" />
            {failure ?? changeset.error}
          </p>
        ) : null}
      </div>
      <Decisions changeset={changeset} pending={pending} decide={decide} />
    </section>
  );
}

/*
 * Operations lists what a changeset does, one plain line each, marked
 * once applied.
 */
function Operations({ changeset }: { changeset: OperatorChangeset }) {
  return (
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
            {operation.text ? <FullText text={operation.text} /> : null}
          </span>
        </li>
      ))}
    </ul>
  );
}

/*
 * Decisions are the buttons a changeset offers in its state: apply or
 * decline while it waits or after it failed, undo once applied unless
 * all it did was run actions.
 */
function Decisions({
  changeset,
  pending,
  decide,
}: {
  changeset: OperatorChangeset;
  pending: ChangesetDecision | null;
  decide: (decision: ChangesetDecision) => void;
}) {
  const open = changeset.status === "pending" || changeset.status === "failed";
  const undoable = changeset.status === "applied" && changeset.operations.some((op) => op.kind !== "action.run");
  if (!open && !undoable) {
    return null;
  }

  return (
    <footer className="flex gap-2 px-3 pb-3">
      {open ? (
        <>
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
        </>
      ) : (
        <Button
          size="small"
          variant="secondary"
          leadingIcon={RiArrowGoBackLine}
          loading={pending === "reversions"}
          onClick={() => decide("reversions")}
        >
          Undo
        </Button>
      )}
    </footer>
  );
}
