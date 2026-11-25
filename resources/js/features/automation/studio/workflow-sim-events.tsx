/*
 * What a simulated workflow did to the conversation, shown the way the
 * inbox shows it: "Workflow tagged the conversation payments", "Workflow
 * assigned the conversation to Billing", a note as a note, each in the
 * chat at the point it happened. Nothing ran for real; the chat reads as
 * the conversation would.
 */
import { MessageBody } from "@/features/inbox/message-body";
import type { DryAction } from "@/lib/api/workflow-simulations";

import { actor, eventText, type Names } from "./workflow-sim-moments";

/*
 * Event is one action in the chat: a note as a note, anything else as a
 * centred line, as the inbox shows them.
 */
export function Event({ action, names }: { action: DryAction; names: Names }) {
  if (action.kind === "note") {
    return (
      <div className="flex flex-col items-end gap-1">
        <span className="text-caption-1-medium text-text-tertiary">{actor} · Note</span>
        <div className="w-fit max-w-[85%] rounded-[var(--radius-2-5xl)] border border-status-yellow-text/40 bg-background-primary-default p-3 shadow-card">
          <MessageBody text={String(action.detail?.text ?? "")} className="text-body-regular text-text-primary" />
        </div>
      </div>
    );
  }

  return (
    <p className="flex justify-center px-4">
      <span className="rounded-full bg-background-tertiary-default px-[7px] py-1 text-center text-body-2-medium text-text-secondary">
        {eventText(action, names)}
      </span>
    </p>
  );
}
