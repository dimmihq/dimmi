/*
 * The words of inviting people: reading the addresses an admin pastes,
 * however they are separated, and saying what happened to each one.
 */
import type { InvitationResult } from "@/lib/api/invitations";

/*
 * maxInvitations is how many addresses one batch may carry; the server
 * refuses more.
 */
export const maxInvitations = 50;

/*
 * splitEmails reads the addresses out of what was typed or pasted:
 * separated by commas, semicolons, spaces or new lines, each once,
 * whatever its case.
 */
export function splitEmails(text: string): string[] {
  const seen = new Set<string>();
  const emails: string[] = [];

  for (const part of text.split(/[\s,;]+/)) {
    const email = part.trim();
    if (email === "" || seen.has(email.toLowerCase())) {
      continue;
    }

    seen.add(email.toLowerCase());
    emails.push(email);
  }

  return emails;
}

/*
 * Outcome is what happened to one address, in words, and whether it
 * went out, was not needed, or failed.
 */
export interface Outcome {
  tone: "sent" | "skipped" | "failed";
  text: string;
}

/*
 * outcome says what happened to one address of a batch.
 */
export function outcome(result: InvitationResult): Outcome {
  switch (result.result) {
    case "created":
    case "resent":
    case "reactivation":
      return { tone: "sent", text: "Invitation sent" };
    case "already_member":
      return { tone: "skipped", text: "Already a member" };
    default:
      return { tone: "failed", text: result.error?.message ?? "This address cannot be invited." };
  }
}

/*
 * roleName is the role as a person reads it in a sentence, such as "a
 * member" or "an admin".
 */
export function roleName(role: string): string {
  return role === "admin" ? "an admin" : "a member";
}

/*
 * expiryLabel says when an invitation stops working, as a date in the
 * reader's language.
 */
export function expiryLabel(iso: string, locale?: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  return `Expires ${new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(date)}`;
}
