/*
 * What the tracker screens compute rather than draw: money without
 * cents, a customer's name, and why a delivery was held back.
 */
import type { HeldReason, TrackerConversation } from "@/lib/api/ticket-trackers";
import { i18n } from "@/lib/i18n";
import { formatNumber } from "@/lib/i18n/format";

/*
 * money writes revenue in dollars, without cents.
 */
export function money(value: number): string {
  return formatNumber(value, { style: "currency", currency: "USD", maximumFractionDigits: 0 });
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
