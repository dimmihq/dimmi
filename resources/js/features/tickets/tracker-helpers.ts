/*
 * What the tracker screens compute rather than draw: money without
 * cents and a customer's name.
 */
import type { TrackerConversation } from "@/lib/api/ticket-trackers";
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
