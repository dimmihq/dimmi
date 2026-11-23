/*
 * A change Operator proposed, as a compact card the teammate decides on:
 * what it touches, why in two lines that open on a click, what each
 * operation does when asked, and what became of it.
 * Waiting changes apply or are declined in one click; applied ones can
 * be undone, which puts back every value Operator replaced unless
 * someone changed it since. A change that failed says why and can be
 * tried again. A change to an article or a procedure is a short line
 * per change with the way to review it in that editor, where it is
 * accepted or declined.
 */
import {
  RiArrowDownSLine,
  RiArticleLine,
  RiCheckLine,
  RiCloseLine,
  RiErrorWarningLine,
  RiFlashlightLine,
  RiGitPullRequestLine,
  RiListOrdered2,
  RiPriceTag3Line,
  RiStickyNoteLine,
} from "@remixicon/react";
import { useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { MessageBody } from "@/features/inbox/message-body";
import { Unfold } from "@/features/shell/reveal";
import type { ChangesetDecision, ChangesetStatus, OperatorChangeset } from "@/lib/api/operator";
import { cx } from "@/utils/cx";

import { reviewTarget } from "./review-target";
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
  children,
}: {
  changeset: OperatorChangeset;
  onDecide: (id: string, decision: ChangesetDecision) => Promise<unknown>;
  /* What the change looks like in place, such as a diff, under its operations. */
  children?: ReactNode;
}) {
  const [pending, setPending] = useState<ChangesetDecision | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const target = reviewTarget(changeset);
  const navigate = useNavigate();
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
  const label = changeset.preview
    ? "Dry run preview"
    : changeset.auto && changeset.status === "applied"
      ? "Applied on its own"
      : statusLabels[changeset.status];
  const reviewing = target !== null && changeset.status === "pending";

  return (
    <section
      aria-label={changeset.title}
      className="flex flex-col gap-2.5 rounded-2xl border border-border-button-default bg-background-primary-default p-3 shadow-xs"
    >
      <div className="flex items-start gap-3">
        <KindTile changeset={changeset} />
        <div className="flex min-w-0 flex-1 flex-col">
          <h3 className="truncate text-body-2-medium text-text-primary">{changeset.title}</h3>
          <p className="truncate text-body-2-regular text-text-tertiary">{targetLine(changeset)}</p>
        </div>
        {reviewing ? (
          <Button size="xs" variant="secondary" onClick={() => void navigate(target)} className="mt-0.5 shrink-0">
            Review
          </Button>
        ) : (
          <Chip variant="caption" color={statusColors[changeset.status]} className="mt-0.5 shrink-0 py-0">
            {label}
          </Chip>
        )}
      </div>
      {changeset.reason ? <Reason text={changeset.reason} /> : null}
      {target === null ? <Details changeset={changeset} /> : null}
      {children}
      {changeset.error || failure ? (
        <p className="flex items-start gap-1.5 text-body-2-regular text-text-error-primary">
          <RiErrorWarningLine aria-hidden className="mt-0.5 size-3.5 shrink-0" />
          {failure ?? changeset.error}
        </p>
      ) : null}
      {reviewing || changeset.preview ? null : <Decisions changeset={changeset} pending={pending} decide={decide} />}
    </section>
  );
}

/*
 * kindOf is what a changeset mostly changes, for its tile and its line.
 */
function kindOf(changeset: OperatorChangeset): { icon: typeof RiArticleLine; noun: string } {
  const kind = changeset.operations[0]?.kind ?? "";
  if (kind.startsWith("article.")) {
    return { icon: RiArticleLine, noun: "Article" };
  }
  if (kind.startsWith("procedure.")) {
    return { icon: RiListOrdered2, noun: "Procedure" };
  }
  if (kind.startsWith("action.")) {
    return { icon: RiFlashlightLine, noun: "Action" };
  }
  if (kind.includes("tag")) {
    return { icon: RiPriceTag3Line, noun: "Tags" };
  }
  if (kind.includes("note")) {
    return { icon: RiStickyNoteLine, noun: "Note" };
  }
  return { icon: RiGitPullRequestLine, noun: "Change" };
}

function KindTile({ changeset }: { changeset: OperatorChangeset }) {
  const Icon = kindOf(changeset).icon;
  return (
    <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border-button-default bg-background-secondary-default">
      <Icon aria-hidden className="size-4 text-foreground-icon-secondary" />
    </span>
  );
}

/*
 * targetLine names what the change touches: the article or procedure by
 * name, or how many things it changes.
 */
function targetLine(changeset: OperatorChangeset): string {
  const { noun } = kindOf(changeset);
  const first = changeset.operations[0];
  const quoted = first?.label.match(/["“]([^"”]+)["”]/)?.[1] ?? first?.title;
  if (quoted) {
    return `${noun} · ${quoted}`;
  }
  const n = changeset.operations.length;
  return n === 1 ? (changeset.operations[0]?.label ?? noun) : `${noun} · ${n} changes`;
}

/*
 * Reason is why Operator suggests the change, two lines until opened.
 */
function Reason({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <button
      type="button"
      aria-expanded={open}
      onClick={() => setOpen(!open)}
      className={cx(
        "cursor-pointer rounded-md text-start text-body-2-regular text-text-secondary outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        !open && "line-clamp-2",
      )}
    >
      {emphasized(text)}
    </button>
  );
}

/*
 * emphasized shows the bold and italics Operator writes inline, so a
 * reason never shows its asterisks.
 */
function emphasized(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|\*[^*\s][^*]*\*)/g).map((part, i) => {
    const key = `${i}:${part}`;
    if (part.startsWith("**") && part.endsWith("**") && part.length > 4) {
      return (
        <strong key={key} className="font-medium text-text-primary">
          {part.slice(2, -2)}
        </strong>
      );
    }
    if (part.startsWith("*") && part.endsWith("*") && part.length > 2) {
      return <em key={key}>{part.slice(1, -1)}</em>;
    }
    return part;
  });
}

/*
 * Details folds what each operation does, for changes applied right here.
 */
function Details({ changeset }: { changeset: OperatorChangeset }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="flex flex-col">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex w-fit cursor-pointer items-center gap-0.5 rounded-md text-caption-1-medium text-text-tertiary outline-none transition-colors duration-150 hover:text-text-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        {changeset.operations.length === 1 ? "What it changes" : `What it changes (${changeset.operations.length})`}
        <RiArrowDownSLine
          aria-hidden
          className={cx("size-3.5 transition-transform duration-300 ease", open && "rotate-180")}
        />
      </button>
      <Unfold open={open}>
        <div className="pt-2">
          <Operations changeset={changeset} brief={false} />
        </div>
      </Unfold>
    </div>
  );
}

/*
 * Operations lists what a changeset does, one plain line each, marked
 * once applied; brief leaves out the text, which its editor shows.
 */
function Operations({ changeset, brief }: { changeset: OperatorChangeset; brief: boolean }) {
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
            {operation.text && !brief ? <FullText text={operation.text} /> : null}
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
  const [asking, setAsking] = useState(false);
  const open = changeset.status === "pending" || changeset.status === "failed";
  const undoable = changeset.status === "applied" && changeset.operations.some((op) => op.kind !== "action.run");
  if (!open && !undoable) {
    return null;
  }

  return (
    <footer className="flex gap-2">
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
        <Button size="small" variant="secondary" loading={pending === "reversions"} onClick={() => setAsking(true)}>
          Undo
        </Button>
      )}
      {asking ? (
        <AlertDialog
          isOpen
          onOpenChange={(shown) => (shown ? undefined : setAsking(false))}
          title="Undo this change?"
          description={`“${changeset.title}” is taken back: every value Operator replaced goes back to what it was, unless someone changed it since.`}
          confirmLabel="Undo change"
          onConfirm={() => decide("reversions")}
        />
      ) : null}
    </footer>
  );
}
