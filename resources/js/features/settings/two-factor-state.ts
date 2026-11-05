/*
 * What the list of the person's factors means for the security page.
 */
import type { AuthFactor } from "@/lib/api/auth-factors";
import { formatDate } from "@/lib/i18n/format";

/*
 * twoFactorState reads what the list of factors means for the page: the
 * active authenticator, the active passkeys, the recovery code set, and
 * whether two-factor is on, which any authenticator or passkey turns on.
 */
export function twoFactorState(factors: AuthFactor[]) {
  const active = factors.filter((f) => f.status === "active");
  const authenticator = active.find((f) => f.type === "totp");
  const passkeys = active.filter((f) => f.type === "passkey");
  const recovery = active.find((f) => f.type === "recovery_code");

  return { authenticator, passkeys, recovery, on: Boolean(authenticator) || passkeys.length > 0 };
}

/*
 * isLastFactor reports whether removing the factor turns two-factor off:
 * it is the only authenticator or passkey left.
 */
export function isLastFactor(factors: AuthFactor[], factor: AuthFactor): boolean {
  const { authenticator, passkeys } = twoFactorState(factors);
  const protecting = [...(authenticator ? [authenticator] : []), ...passkeys];

  return protecting.length === 1 && protecting[0]?.id === factor.id;
}

/*
 * remainingLabel says how many recovery codes are left, and warns when
 * few are.
 */
export function remainingLabel(remaining: number, total: number): string {
  if (remaining === 0) {
    return "No codes left. Make new ones now.";
  }

  const left = `${remaining} of ${total} left`;

  return remaining <= 2 ? `${left}. Make new ones soon.` : left;
}

/*
 * usedLabel says when a factor was added and last used, writing each day
 * with the date given, else in the reader's language.
 */
export function usedLabel(factor: AuthFactor, date: (iso: string) => string = (iso) => formatDate(iso)): string {
  const added = `Added ${date(factor.confirmed_at ?? factor.created_at)}`;
  const used = factor.last_used_at ? `last used ${date(factor.last_used_at)}` : "never used";

  return `${added}, ${used}`;
}
