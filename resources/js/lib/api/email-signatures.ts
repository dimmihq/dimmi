/*
 * Email signatures: a teammate's own, for every address or one, and the
 * team's for each address. The queries read them, the calls keep, change
 * and remove one, and a rendering shows how one reads in a reply before
 * it is saved. Every call names the workspace in the Dimmi-Workspace
 * header.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface EmailSignature {
  id: string;
  kind: "personal" | "address";
  member_id: string | null;
  email_address_id: string | null;
  body: string;
  enabled: boolean;
}

export interface SignatureRendering {
  html: string;
  text: string;
}

export interface NewSignature {
  kind?: "personal" | "address";
  email_address_id?: string;
  body: string;
  enabled?: boolean;
}

/*
 * signaturesKey is the cache key every signature query starts with.
 */
export function signaturesKey(workspace: string) {
  return ["workspace", workspace, "email-signatures"];
}

export function mySignaturesQuery(workspace: string) {
  return queryOptions({
    queryKey: [...signaturesKey(workspace), "me"],
    queryFn: () => request<EmailSignature[]>("GET", "/email-signatures?filter[member]=me", undefined, { workspace }),
  });
}

export function teamSignaturesQuery(workspace: string) {
  return queryOptions({
    queryKey: [...signaturesKey(workspace), "address"],
    queryFn: () => request<EmailSignature[]>("GET", "/email-signatures?filter[kind]=address", undefined, { workspace }),
  });
}

export function createSignature(workspace: string, signature: NewSignature): Promise<EmailSignature> {
  return request<EmailSignature>("POST", "/email-signatures", signature, { workspace });
}

export function updateSignature(
  workspace: string,
  id: string,
  changes: { body?: string; enabled?: boolean },
): Promise<EmailSignature> {
  return request<EmailSignature>("PATCH", `/email-signatures/${encodeURIComponent(id)}`, changes, { workspace });
}

export function deleteSignature(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/email-signatures/${encodeURIComponent(id)}`, undefined, { workspace });
}

export function renderSignature(workspace: string, body: string, address?: string): Promise<SignatureRendering> {
  return request<SignatureRendering>(
    "POST",
    "/email-signatures/renderings",
    { body, email_address_id: address },
    { workspace },
  );
}
