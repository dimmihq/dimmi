/*
 * Folding runs of back-to-back links in a ticket's history into one
 * line each.
 */
import type { TicketEvent } from "@/lib/api/tickets";

/*
 * foldLinks groups each run of back-to-back links in a list, so a long
 * history of conversations joining reads as one line. isLink says which
 * items are links; the rest pass through.
 */
export function foldLinks<T>(items: T[], linkOf: (item: T) => TicketEvent | null): (T | TicketEvent[])[] {
  const out: (T | TicketEvent[])[] = [];
  for (const item of items) {
    const link = linkOf(item);
    const previous = out[out.length - 1];
    if (link && Array.isArray(previous)) {
      previous.push(link);
    } else {
      out.push(link ? [link] : item);
    }
  }

  return out;
}
