/*
 * The person's own second factors: an authenticator app, the passkeys on
 * their devices, and the set of recovery codes that stands in for them.
 * Secrets, the authenticator's key and the codes, come back once, in the
 * answer that made them, and are never listed again. A passkey's setup is
 * only the options the device makes it with.
 */
import { queryOptions } from "@tanstack/react-query";

import { type CreationOptions, createPasskey } from "@/lib/passkeys";

import { request } from "./client";

export interface AuthFactor {
  id: string;
  object: "auth_factor";
  type: "totp" | "passkey" | "recovery_code";
  name: string;
  status: "pending" | "active";
  preferred: boolean;
  confirmed_at: string | null;
  last_used_at: string | null;
  created_at: string;
  remaining?: number;
  total?: number;
}

export interface TotpSetup {
  secret: string;
  otpauth_url: string;
  qr_svg: string;
}

export interface NewAuthenticator extends AuthFactor {
  setup: TotpSetup;
}

export interface NewPasskey extends AuthFactor {
  setup: { options: CreationOptions };
}

export interface NewRecoveryCodes extends AuthFactor {
  codes: string[];
}

export interface ActivatedFactor extends AuthFactor {
  recovery_codes?: NewRecoveryCodes;
}

export const authFactorsQuery = queryOptions({
  queryKey: ["auth-factors"],
  queryFn: () => request<AuthFactor[]>("GET", "/user/auth-factors"),
});

export function addAuthenticator(): Promise<NewAuthenticator> {
  return request<NewAuthenticator>("POST", "/user/auth-factors", { type: "totp" });
}

export function activateFactor(id: string, code: string): Promise<ActivatedFactor> {
  return request<ActivatedFactor>("PATCH", `/user/auth-factors/${encodeURIComponent(id)}`, { code });
}

/*
 * startPasskey adds a pending passkey under the name, which the device
 * then makes with the options it carries.
 */
export function startPasskey(name: string): Promise<NewPasskey> {
  const named = name.trim();

  return request<NewPasskey>(
    "POST",
    "/user/auth-factors",
    named ? { type: "passkey", name: named } : { type: "passkey" },
  );
}

/*
 * finishPasskey has the device make the pending passkey and activates it
 * with the device's answer.
 */
export async function finishPasskey(pending: NewPasskey): Promise<ActivatedFactor> {
  const credential = await createPasskey(pending.setup.options);

  return request<ActivatedFactor>("PATCH", `/user/auth-factors/${encodeURIComponent(pending.id)}`, { credential });
}

export function renameFactor(id: string, name: string): Promise<AuthFactor> {
  return request<AuthFactor>("PATCH", `/user/auth-factors/${encodeURIComponent(id)}`, { name: name.trim() });
}

export function regenerateRecoveryCodes(): Promise<NewRecoveryCodes> {
  return request<NewRecoveryCodes>("POST", "/user/auth-factors", { type: "recovery_code" });
}

export function removeFactor(id: string): Promise<void> {
  return request<void>("DELETE", `/user/auth-factors/${encodeURIComponent(id)}`);
}

export function turnOffTwoFactor(): Promise<void> {
  return request<void>("DELETE", "/user/auth-factors");
}

/*
 * confirmPassword proves again who the person is, which adding or
 * removing a factor asks for when the last proof is not recent.
 */
export function confirmPassword(password: string): Promise<unknown> {
  return request<unknown>("POST", "/user/confirmation", { method: "password", password });
}

/*
 * qrImage returns the QR code drawing as an address an image can show,
 * so the server's markup is never put into the page as HTML.
 */
export function qrImage(svg: string): string {
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}
