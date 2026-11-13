/*
 * Telling the items opened in the inbox apart by their ids.
 */

/*
 * isTicketId reports whether an opened item's id names a ticket rather
 * than a conversation.
 */
export function isTicketId(id: string): boolean {
  return id.startsWith("tkt_");
}
