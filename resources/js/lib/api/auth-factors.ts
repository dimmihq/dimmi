/*
 * The person's own second factors: an authenticator app and the set of
 * recovery codes that stands in for it. Secrets, the authenticator's
 * key and the codes, come back once, in the answer that made them, and
 * are never listed again.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface AuthFactor {
  id: string;
  object: "auth_factor";
  type: "totp" | "recovery_code";
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
