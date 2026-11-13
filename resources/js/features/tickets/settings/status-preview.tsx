/*
 * What a customer sees of their ticket in the state picked in the type's
 * states: the state's customer label, a bar of three steps (sent, being
 * handled, done) filled up to where the state sits, and the note that
 * updates come here and by email. Nothing here is saved.
 */
import type { StateGroup, TicketState } from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

/*
 * stepOf places a group on the customer's three steps.
 */
function stepOf(group: StateGroup): number {
  if (group === "submitted") {
    return 1;
  }

  return group === "resolved" ? 3 : 2;
}

export function StatusPreview({ typeName, state }: { typeName: string; state: TicketState | undefined }) {
  const step = state ? stepOf(state.group) : 1;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between gap-2 px-1">
        <span className="text-body-2-medium text-text-secondary">Ticket status preview</span>
        <span className="text-caption-1-regular text-text-tertiary">Pick a state to see it</span>
      </div>
      <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-primary-default p-4 shadow-xs">
        <div className="flex flex-col gap-1 text-center">
          <span className="text-headline-medium text-text-primary">{typeName || "Untitled type"}</span>
          <span className="text-body-medium text-accent-600">{state?.customer_label ?? "Submitted"}</span>
        </div>
        <div className="flex gap-1.5" aria-hidden>
          {[1, 2, 3].map((n) => (
            <span
              key={n}
              className={cx("h-1 flex-1 rounded-full", n <= step ? "bg-accent-500" : "bg-background-tertiary-default")}
            />
          ))}
        </div>
        <div className="rounded-xl bg-background-secondary-default px-3 py-2.5 text-body-2-regular text-text-secondary">
          You'll be notified here and by email.
        </div>
      </div>
    </div>
  );
}
