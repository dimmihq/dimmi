/*
 * The ways an admin can help a teammate who cannot sign in, as entries of
 * the teammate's menu: a link that turns off the second factor they lost,
 * and lifting the lock failed guesses set on their account. What is
 * offered follows the admin's permissions, never their role.
 */
import type { Member } from "@/lib/api/workspace";

/*
 * Help is a way to help one member, waiting for the admin to confirm it.
 */
export interface Help {
  kind: "two-factor" | "lock";
  member: Member;
}

/*
 * HelpPowers is what the admin may do: change members (write), change
 * owners (owners), and send reset links (reset).
 */
export interface HelpPowers {
  self: string;
  write: boolean;
  owners: boolean;
  reset: boolean;
}

/*
 * helpOffers lists the ways the admin may help an active teammate, never
 * themselves: a reset link, and lifting a lock while one holds.
 */
export function helpOffers(member: Member, powers: HelpPowers): Help[] {
  if (member.id === powers.self || member.status !== "active") {
    return [];
  }

  const offers: Help[] = [];
  if (powers.reset && (member.role !== "owner" || powers.owners)) {
    offers.push({ kind: "two-factor", member });
  }
  if (powers.write && member.sign_in_locked) {
    offers.push({ kind: "lock", member });
  }

  return offers;
}

/*
 * helpLabel names the way to help in the member's menu.
 */
export function helpLabel(help: Help): string {
  return help.kind === "two-factor" ? "Help with two-factor" : "Clear sign-in lock";
}
