/*
 * Which second factor the challenge page asks for: the one the person
 * prefers among those the challenge offers, and which others can stand
 * in for it. A passkey needs the options the challenge carries, so a
 * page that does not know the challenge offers only the typed codes.
 */
import type { ChallengeMethod } from "@/lib/api/auth";
import type { RequestOptions } from "@/lib/passkeys";

export type SecondFactor = "passkey" | "totp" | "recovery_code";

/*
 * secondFactors lists every kind of second factor, in the order the
 * page offers them as alternatives.
 */
export const secondFactors: SecondFactor[] = ["passkey", "totp", "recovery_code"];

/*
 * firstMethod returns the method to ask for first: the preferred one the
 * challenge offers, the authenticator when the challenge is not known.
 */
export function firstMethod(methods: ChallengeMethod[] | null): SecondFactor {
  const known = (methods ?? []).filter((m) => offers(methods, m.method as SecondFactor));
  const preferred = known.find((m) => m.preferred) ?? known[0];

  return (preferred?.method as SecondFactor | undefined) ?? "totp";
}

/*
 * offers reports whether the challenge lets the person answer with the
 * method. An unknown challenge offers both kinds of code, and the server
 * says no; a passkey is offered only with its options.
 */
export function offers(methods: ChallengeMethod[] | null, method: SecondFactor): boolean {
  if (method === "passkey") {
    return passkeyOptions(methods) !== null;
  }

  if (!secondFactors.includes(method)) {
    return false;
  }

  return methods === null || methods.some((m) => m.method === method);
}

/*
 * passkeyOptions returns the options the challenge's passkeys sign with,
 * the same for each of them, or null when it offers none.
 */
export function passkeyOptions(methods: ChallengeMethod[] | null): RequestOptions | null {
  return methods?.find((m) => m.method === "passkey" && m.options)?.options ?? null;
}
