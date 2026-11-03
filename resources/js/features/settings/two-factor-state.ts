/*
 * What the list of the person's factors means for the security page.
 */
import type { AuthFactor } from "@/lib/api/auth-factors";

/*
 * twoFactorState reads what the list of factors means for the page: the
 * active authenticator, the recovery code set, and whether two-factor is
 * on.
 */
export function twoFactorState(factors: AuthFactor[]) {
  const authenticator = factors.find((f) => f.type === "totp" && f.status === "active");
  const recovery = factors.find((f) => f.type === "recovery_code" && f.status === "active");

  return { authenticator, recovery, on: Boolean(authenticator) };
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
