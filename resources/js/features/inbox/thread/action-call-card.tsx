/*
 * An action the agent asked to run, as the team sees it in the thread.
 * While it waits for approval it is a card across the thread: the
 * action, what exactly would run (the arguments frozen when the customer
 * confirmed), why the agent asked, and how long is left before the
 * request expires and the conversation comes to the team. Approve runs
 * it once; Decline opens a short form for the team's own reason and an
 * optional message for the customer; Take over assigns the conversation
 * to the reader. Waiting is never blocking: the customer was already
 * told the team is checking. Once decided, the card folds into a single
 * line on the timeline that opens again to show what ran.
 */
import {
  RiArrowDownSLine,
  RiCheckLine,
  RiCloseCircleLine,
  RiFlashlightLine,
  RiTimeLine,
  RiUserReceivedLine,
} from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { type ActionCall, type CallDecision, decideCall } from "@/features/ai/actions/api";
import {
  argumentLines,
  callStateLabels,
  callStateTones,
  callTitle,
  decisionLine,
  timeLeft,
} from "@/features/ai/actions/format";
import { KindMark } from "@/features/automation/parts";
import { DimmiIconSmall } from "@/features/brand/logo";
import { ApiError } from "@/lib/api/client";
import { updateConversation } from "@/lib/api/inbox";
import { callerQuery } from "@/lib/api/workspace";
import { useMutationId } from "@/lib/realtime/context";
import { cx } from "@/utils/cx";

import { useInbox } from "../conversations";

/*
 * useNow ticks once a minute, so a countdown stays true without a
 * redraw every second.
 */
function useNow(): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30000);
    return () => window.clearInterval(timer);
  }, []);

  return now;
}

function Arguments({ call }: { call: ActionCall }) {
  const lines = argumentLines(call.arguments);
  if (lines.length === 0) {
    return null;
  }

  return (
    <dl className="grid grid-cols-[minmax(0,auto)_minmax(0,1fr)] gap-x-4 gap-y-1.5 rounded-xl bg-background-secondary-default p-3">
      {lines.map((line) => (
        <div key={line.label} className="contents">
          <dt className="text-body-2-regular text-text-secondary">{line.label}</dt>
          <dd className="min-w-0 break-words text-body-2-medium text-text-primary">
            <bdi>{line.value}</bdi>
          </dd>
        </div>
      ))}
    </dl>
  );
}

/*
 * useDecision sends the team's decision and takes the conversation over,
 * reading the timeline again after either.
 */
function useDecision(conversation: string, call: ActionCall) {
  const { workspace, me } = useInbox();
  const client = useQueryClient();
  const mutationId = useMutationId();
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const [local, setLocal] = useState<ActionCall | null>(null);
  const [busy, setBusy] = useState<"approved" | "declined" | "take" | null>(null);
  const refresh = () => client.invalidateQueries({ queryKey: ["workspace", workspace] });

  const decide = async (decision: CallDecision) => {
    setBusy(decision.status);
    try {
      const result = await decideCall(workspace, call.id, decision);
      setLocal({
        ...call,
        ...result,
        state: result.state ?? decision.status,
        decided_by: result.decided_by ?? { id: caller?.member?.id ?? "", name: me },
        decision_reason: result.decision_reason ?? decision.reason ?? null,
      });
      toast.success(decision.status === "approved" ? "Approved" : "Declined", {
        description:
          decision.status === "approved"
            ? "dimmi runs it once and tells the customer how it went."
            : "dimmi lets the customer know.",
      });
      await refresh();
    } catch (error) {
      toast.error("Not decided", { description: error instanceof ApiError ? error.message : "Try again." });
    } finally {
      setBusy(null);
    }
  };

  const takeOver = async () => {
    const member = caller?.member?.id;
    if (!member) {
      return;
    }
    setBusy("take");
    try {
      await updateConversation({ workspace, mutation: mutationId() }, conversation, { assignee_id: member });
      toast.success("The conversation is yours", { description: "dimmi stops answering here." });
      await refresh();
    } catch (error) {
      toast.error("Not assigned", { description: error instanceof ApiError ? error.message : "Try again." });
    } finally {
      setBusy(null);
    }
  };

  return {
    shown: local ?? call,
    busy,
    decide,
    takeOver,
    allowed: caller?.member?.permissions.includes("ai_action_calls:approve") ?? false,
  };
}

function DeclineForm({
  busy,
  onDecline,
  onBack,
}: {
  busy: boolean;
  onDecline: (d: CallDecision) => void;
  onBack: () => void;
}) {
  const [reason, setReason] = useState("");
  const [message, setMessage] = useState("");

  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault();
        onDecline({ status: "declined", reason: reason.trim(), customer_message: message.trim() || undefined });
      }}
    >
      <Textarea
        label="Why (only the team sees this)"
        rows={2}
        autoResize
        maxRows={5}
        maxLength={500}
        value={reason}
        onChange={setReason}
        placeholder="Past the 30-day refund window."
        isRequired
      />
      <Textarea
        label="Message for the customer"
        hint="Optional. Without one, dimmi explains it could not be done and offers the team."
        rows={2}
        autoResize
        maxRows={5}
        maxLength={500}
        value={message}
        onChange={setMessage}
      />
      <div className="flex flex-wrap gap-2">
        <Button type="submit" variant="danger" size="small" loading={busy} disabled={!reason.trim()}>
          Decline
        </Button>
        <Button variant="secondary" size="small" onClick={onBack} disabled={busy}>
          Back
        </Button>
      </div>
    </form>
  );
}

function Pending({ conversation, call }: { conversation: string; call: ActionCall }) {
  const now = useNow();
  const { shown, busy, decide, takeOver, allowed } = useDecision(conversation, call);
  const [declining, setDeclining] = useState(false);
  const left = timeLeft(call.expires_at, now);
  const title = callTitle(call);

  if (shown.state !== "pending_approval") {
    return <Decided call={shown} />;
  }

  return (
    <article
      aria-label={`Approval needed: ${title}`}
      className="mx-3 flex flex-col gap-3 rounded-[var(--radius-2-5xl)] bg-background-primary-default p-3 shadow-card"
    >
      <header className="flex items-start gap-2.5">
        <KindMark icon={RiFlashlightLine} tint="orange" />
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="text-body-medium text-text-primary">{title}</span>
          <span className="flex items-center gap-1 text-body-2-regular text-text-secondary">
            <DimmiIconSmall className="size-3.5 text-text-primary" />
            Requested by dimmi · waiting for approval
          </span>
        </div>
        {left ? (
          <Chip variant="caption" color={left === "Expired" ? "neutral" : "orange"} className="shrink-0 gap-0.5">
            <RiTimeLine aria-hidden className="size-3" />
            {left}
          </Chip>
        ) : null}
      </header>
      {call.summary ? <p className="text-body-regular text-text-primary">{call.summary}</p> : null}
      <Arguments call={call} />
      {call.reason ? (
        <p className="text-body-2-regular text-text-secondary">
          <span className="text-text-primary">Why dimmi asked: </span>
          {call.reason}
        </p>
      ) : null}
      {declining ? (
        <DeclineForm busy={busy === "declined"} onDecline={decide} onBack={() => setDeclining(false)} />
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="small"
            leadingIcon={RiCheckLine}
            loading={busy === "approved"}
            disabled={!allowed || busy !== null}
            onClick={() => void decide({ status: "approved" })}
          >
            Approve
          </Button>
          <Button
            variant="secondary"
            size="small"
            leadingIcon={RiCloseCircleLine}
            disabled={!allowed || busy !== null}
            onClick={() => setDeclining(true)}
          >
            Decline
          </Button>
          <Button
            variant="ghost"
            size="small"
            leadingIcon={RiUserReceivedLine}
            loading={busy === "take"}
            disabled={busy !== null}
            onClick={() => void takeOver()}
          >
            Take over
          </Button>
          {allowed ? null : (
            <span className="text-body-2-regular text-text-tertiary">Only teammates who approve actions decide.</span>
          )}
        </div>
      )}
    </article>
  );
}

const lineIcons = { lime: RiCheckLine, rose: RiCloseCircleLine } as const;

/*
 * Decided is the single timeline line a call becomes, opening to show
 * what ran.
 */
function Decided({ call }: { call: ActionCall }) {
  const [open, setOpen] = useState(false);
  const tone = callStateTones[call.state];
  const Icon = tone === "lime" || tone === "rose" ? lineIcons[tone] : RiFlashlightLine;

  return (
    <div className="flex flex-col items-center gap-2 px-6">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex max-w-full cursor-pointer items-center gap-1.5 rounded-full bg-background-tertiary-default px-2 py-1 text-body-2-medium text-text-secondary outline-none transition-colors duration-150 hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <Icon
          aria-hidden
          className={cx(
            "size-3.5 shrink-0",
            tone === "lime" && "text-status-lime-text",
            tone === "rose" && "text-status-rose-text",
          )}
        />
        <span className="truncate">
          {callTitle(call)} · {decisionLine(call)}
        </span>
        <RiArrowDownSLine aria-hidden className={cx("size-3.5 shrink-0 transition-transform", open && "rotate-180")} />
      </button>
      {open ? (
        <div className="flex w-full max-w-md flex-col gap-2">
          {call.summary ? <p className="text-center text-body-2-regular text-text-secondary">{call.summary}</p> : null}
          <Arguments call={call} />
        </div>
      ) : null}
    </div>
  );
}

/*
 * Waiting is a call the customer has yet to confirm: nothing for the
 * team to do, said in one line.
 */
function Waiting({ call }: { call: ActionCall }) {
  return (
    <p className="flex justify-center px-6">
      <span className="flex max-w-full items-center gap-1.5 rounded-full bg-background-tertiary-default px-2 py-1 text-body-2-medium text-text-secondary">
        <RiTimeLine aria-hidden className="size-3.5 shrink-0" />
        <span className="truncate">
          {callTitle(call)} · {callStateLabels[call.state]}
        </span>
      </span>
    </p>
  );
}

export function ActionCallCard({ conversation, call }: { conversation: string; call: ActionCall }) {
  if (call.state === "pending_approval") {
    return <Pending conversation={conversation} call={call} />;
  }
  if (call.state === "awaiting_confirmation") {
    return <Waiting call={call} />;
  }

  return <Decided call={call} />;
}
