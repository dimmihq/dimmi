/*
 * What the tracker and incident screens compute rather than draw: money
 * without cents, a customer's name, why a delivery was held back, the
 * closing note drafted from an incident's log, the states a ticket's
 * type moves through, and saving a change at the version read.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { toast } from "@/components/base/toast/toast";
import type { HeldReason, PublicUpdate, TrackerConversation } from "@/lib/api/ticket-trackers";
import {
  type Ticket,
  type TicketChange,
  type TicketState,
  ticketKeys,
  ticketStatesQuery,
  ticketTypesQuery,
  updateTicket,
} from "@/lib/api/tickets";
import { i18n } from "@/lib/i18n";
import { formatNumber } from "@/lib/i18n/format";

/*
 * money writes revenue without cents.
 */
export function money(value: number): string {
  return formatNumber(value, { maximumFractionDigits: 0 });
}

export function customerName(row: Pick<TrackerConversation, "contact_name" | "contact_email">): string {
  return row.contact_name || row.contact_email || "—";
}

/*
 * heldReasonLabel names why a customer was held back or why a delivery
 * failed; a reason the app does not know shows as the server wrote it.
 */
export function heldReasonLabel(reason: HeldReason | null): string {
  if (!reason) {
    return "";
  }
  const key = `trackers:reasons.${reason}`;

  return i18n.exists(key) ? i18n.t(key) : reason;
}

/*
 * draftFromLog writes the closing note from the newest post, for the
 * teammate to edit before sending.
 */
export function draftFromLog(ticket: Ticket, posts: PublicUpdate[]): string {
  const latest = posts[0]?.body.trim();

  return latest
    ? i18n.t("trackers:incident.draftWithLatest", { title: ticket.title, latest })
    : i18n.t("trackers:incident.draft", { title: ticket.title });
}

/*
 * useTypeStates lists the states the ticket's type moves through, in
 * the type's order.
 */
export function useTypeStates(workspace: string, ticket: Ticket | undefined): TicketState[] {
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const { data: states = [] } = useQuery(ticketStatesQuery(workspace));
  const type = types.find((ty) => ty.id === ticket?.type_id);
  if (!type) {
    return [];
  }
  const byId = new Map(states.map((s) => [s.id, s]));

  return type.state_ids.flatMap((id) => {
    const state = byId.get(id);
    return state ? [state] : [];
  });
}

/*
 * useChangeTicket saves a change at the version read and keeps the page
 * in step; a teammate's change in between asks to look again.
 */
export function useChangeTicket(workspace: string, ticket: Ticket | undefined) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();

  return useMutation({
    mutationFn: (change: TicketChange) => {
      if (!ticket) {
        throw new Error("no ticket");
      }
      return updateTicket(workspace, ticket, change);
    },
    onSuccess: (next) => {
      client.setQueryData(ticketKeys.one(workspace, next.id), (old: Ticket | undefined) => ({ ...old, ...next }));
      void client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
    },
    onError: (error: Error & { status?: number }) => {
      toast.error(error.status === 412 || error.status === 409 ? t("common.changedMeanwhile") : error.message);
      if (ticket) {
        void client.invalidateQueries({ queryKey: ticketKeys.one(workspace, ticket.id) });
      }
    },
  });
}
