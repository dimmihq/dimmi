/*
 * What those who manage members read about each one beyond name and
 * role: whether the account has a second factor and when its owner last
 * came by. Teammates who do not manage members get neither from the
 * server, so nothing is said for them.
 */

import type { Member, MemberFilter } from "@/lib/api/workspace";
import { formatDateTime, formatRelative } from "@/lib/i18n/format";

/*
 * filters are the views of the member list a manager can pick.
 */
export const filters = { everyone: "everyone", withoutTwoFactor: "without_two_factor" } as const;

export type FilterKey = (typeof filters)[keyof typeof filters];

/*
 * filterFor turns a picked view into the list's filter.
 */
export function filterFor(key: FilterKey): MemberFilter {
  return key === filters.withoutTwoFactor ? { twoFactor: false } : {};
}

/*
 * filterKeyFrom reads the view asked for in the page's address, where
 * two_factor=false asks for members without a second factor.
 */
export function filterKeyFrom(search: Record<string, unknown>): FilterKey {
  const asked = search.two_factor;

  return asked === false || asked === "false" ? filters.withoutTwoFactor : filters.everyone;
}

/*
 * lacksTwoFactor says whether a manager should see that the member has
 * no second factor.
 */
export function lacksTwoFactor(member: Member): boolean {
  return member.two_factor_enabled === false;
}

/*
 * activity says when the member last used the workspace, relative and
 * in full, or null when the caller is not told or they never came by.
 */
export function activity(member: Member, now: number, timeZone?: string): { relative: string; full: string } | null {
  if (member.last_active_at === null) {
    return member.two_factor_enabled === null ? null : { relative: "Never active", full: "" };
  }

  return {
    relative: `Active ${formatRelative(member.last_active_at, { now })}`,
    full: formatDateTime(member.last_active_at, { timeZone }),
  };
}

/*
 * unprotectedLine says how many active members have no second factor,
 * or null when everyone has one.
 */
export function unprotectedLine(count: number): string | null {
  if (count === 0) {
    return null;
  }

  return count === 1 ? "1 member has no second factor yet." : `${count} members have no second factor yet.`;
}
