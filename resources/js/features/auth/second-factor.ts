/*
 * Which second factor the challenge page asks for: the one the person
 * prefers among those the challenge offers, and whether the other can
 * stand in for it.
 */
import type { ChallengeMethod } from "@/lib/api/auth";

export type SecondFactor = "totp" | "recovery_code";

/*
 * firstMethod returns the method to ask for first: the preferred one the
 * challenge offers, the authenticator when the challenge is not known.
 */
export function firstMethod(methods: ChallengeMethod[] | null): SecondFactor {
  const known = (methods ?? []).filter((m) => m.method === "totp" || m.method === "recovery_code");
  const preferred = known.find((m) => m.preferred) ?? known[0];

  return (preferred?.method as SecondFactor | undefined) ?? "totp";
}

/*
 * offers reports whether the challenge lets the person answer with the
 * method. An unknown challenge offers both, and the server says no.
 */
export function offers(methods: ChallengeMethod[] | null, method: SecondFactor): boolean {
  return methods === null || methods.some((m) => m.method === method);
}
