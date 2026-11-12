/*
 * The messages dimmi drafted for a person to send to everyone a tracker
 * or incident links: an engineer's note written with @customers, or the
 * closing message of an incident its tool resolved. Nothing goes out on
 * its own. Each says why it was drafted, who it reaches and who would be
 * held and why. A teammate edits it, sends it after confirming, or
 * discards it.
 */
import { RiSendPlaneLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { decideDraft, refreshTicketKeys, type TicketUpdateDraft, ticketDraftsQuery } from "@/lib/api/ticket-issues";

import { draftReasons, heldReasons, toolLook } from "./tools";

/*
 * reach says who a draft reaches and who it holds back.
 */
function reach(draft: TicketUpdateDraft): string {
  const held = Object.entries(draft.held)
    .filter(([, n]) => n > 0)
    .map(([reason, n]) => `${n} ${heldReasons[reason] ?? reason}`);
  const total = draft.recipients === 1 ? "1 customer" : `${draft.recipients} customers`;

  return held.length > 0 ? `Reaches ${total}, holds ${held.join(" and ")}` : `Reaches ${total}`;
}

function Draft({ workspace, ticket, draft }: { workspace: string; ticket: string; draft: TicketUpdateDraft }) {
  const queryClient = useQueryClient();
  const [body, setBody] = useState(draft.body);
  const [confirming, setConfirming] = useState(false);
  const source = draft.source === "dimmi" ? "dimmi" : toolLook(draft.source).label;

  const decide = useMutation({
    mutationFn: (status: "sent" | "discarded") =>
      decideDraft(workspace, draft.id, { status, body: body !== draft.body ? body : undefined }),
    onSuccess: async (decided) => {
      await Promise.all(
        refreshTicketKeys(workspace, ticket).map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      );
      toast.success(decided.status === "sent" ? "Sent to everyone linked." : "Draft discarded.");
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <li className="flex flex-col gap-3 px-4 py-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-body-medium text-text-primary">{draftReasons[draft.reason] ?? draft.reason}</span>
        <span className="text-caption-1-regular text-text-tertiary">
          from {source}
          {draft.author_name ? `, ${draft.author_name}` : ""}
        </span>
      </div>
      <Textarea aria-label="Message" value={body} onChange={setBody} rows={3} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="text-caption-1-regular text-text-secondary">{reach(draft)}</span>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="small"
            loading={decide.isPending}
            onClick={() => decide.mutate("discarded")}
          >
            Discard
          </Button>
          <Button size="small" leadingIcon={RiSendPlaneLine} onClick={() => setConfirming(true)}>
            Send
          </Button>
        </div>
      </div>
      <AlertDialog
        isOpen={confirming}
        onOpenChange={setConfirming}
        title="Send to everyone linked?"
        description={`${reach(draft)}. Each customer gets it in their own channel.`}
        confirmLabel="Send"
        onConfirm={() => decide.mutateAsync("sent")}
      />
    </li>
  );
}

export function CloseLoopDrafts({ workspace, ticket }: { workspace: string; ticket: string }) {
  const { data } = useQuery(ticketDraftsQuery(workspace, ticket));
  const open = (data ?? []).filter((d) => d.status === "ready");

  if (open.length === 0) {
    return null;
  }

  return (
    <section aria-label="Close the loop" className="flex flex-col gap-2">
      <div className="flex flex-col gap-0.5">
        <h3 className="text-headline-medium text-text-primary">Drafts to everyone linked</h3>
        <p className="text-caption-1-regular text-text-tertiary">
          Customers who wrote since, or whom a teammate already told, are held back.
        </p>
      </div>
      <ul className="flex flex-col divide-y divide-separator-border rounded-xl border border-border-primary">
        {open.map((draft) => (
          <Draft key={draft.id} workspace={workspace} ticket={ticket} draft={draft} />
        ))}
      </ul>
    </section>
  );
}
