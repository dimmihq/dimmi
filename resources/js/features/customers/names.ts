/*
 * What the customer pages share: the workspace they work in, and how a
 * customer is named when they gave no name.
 */
import { useInbox } from "@/features/inbox/conversations";
import type { Contact } from "@/lib/api/customers";

/*
 * useWorkspace returns the id of the workspace the pages work in.
 */
export function useWorkspace(): string {
  return useInbox().workspace;
}

export function personName(contact: Pick<Contact, "name" | "email">): string {
  return contact.name ?? contact.email ?? "Unknown customer";
}
