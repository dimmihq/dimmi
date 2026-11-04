/*
 * Invitations bring people into a workspace by mail: admins send,
 * resend and revoke them in the workspace, and the invitee reads one
 * and accepts it, signing up first when they have no account. The
 * invitee's token travels in the Dimmi-Invitation-Token header, never
 * in an address, so it stays out of logs and referrers.
 */
import { queryOptions } from "@tanstack/react-query";

import type { Me } from "./auth";
import { request } from "./client";

export type InvitationRole = "admin" | "member";

export type InvitationStatus = "pending" | "accepted" | "revoked" | "expired";

/*
 * Invitation is one person asked to join the workspace. accept_url is
 * set in the answer that issued a link, so an admin can pass it on by
 * hand; only its digest is kept afterwards.
 */
export interface Invitation {
  id: string;
  object: "invitation";
  email: string;
  role: InvitationRole;
  status: InvitationStatus;
  expires_at: string;
  last_sent_at: string;
  send_count: number;
  created_at: string;
  invited_by: { id: string; name: string } | null;
  accept_url: string | null;
}

export type InvitationResultKind = "created" | "resent" | "already_member" | "reactivation" | "invalid";

/*
 * InvitationResult is what happened to one address of a batch of
 * invitations.
 */
export interface InvitationResult {
  email: string;
  result: InvitationResultKind;
  invitation: Invitation | null;
  error?: { code: string; message: string };
}

/*
 * InvitationDelivery is one more mail of an invitation.
 */
export interface InvitationDelivery {
  object: "invitation_delivery";
  sent_at: string;
  invitation: Invitation;
}

/*
 * InvitationPreview is what the invitee may see before accepting: the
 * workspace, who asked and as what, and whether the invited address
 * already has an account here. The address itself is masked.
 */
export interface InvitationPreview {
  object: "invitation_preview";
  workspace: { id: string; name: string; avatar_url: string | null };
  inviter: { name: string } | null;
  role: InvitationRole;
  email_masked: string;
  status: InvitationStatus;
  expires_at: string;
  has_account: boolean;
}

/*
 * Membership is the signed-in user's place in a workspace.
 */
export interface Membership {
  id: string;
  object: "membership";
  role: string;
  status: string;
  joined_via: string;
  last_accessed_at: string | null;
  workspace: {
    id: string;
    object: "workspace";
    name: string;
    slug: string;
    deleted_at: string | null;
    purge_after: string | null;
  };
}

function tokenHeader(token: string) {
  return { headers: { "Dimmi-Invitation-Token": token } };
}

export function invitationsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "invitations"],
    queryFn: () =>
      request<Invitation[]>("GET", "/invitations?filter[status]=pending&limit=100", undefined, { workspace }),
  });
}

/*
 * sendInvitations invites every address with the role and answers what
 * happened to each one.
 */
export function sendInvitations(
  workspace: string,
  emails: string[],
  role: InvitationRole,
  teamIds: string[] = [],
): Promise<InvitationResult[]> {
  const invitations = emails.map((email) =>
    teamIds.length > 0 ? { email, role, team_ids: teamIds } : { email, role },
  );

  return request<InvitationResult[]>("POST", "/invitations", { invitations }, { workspace });
}

/*
 * resendInvitation sends the invitation again with a fresh link: by mail,
 * or, on the link channel, only back to the caller to pass on by hand.
 * Either way the link sent before stops working.
 */
export function resendInvitation(
  workspace: string,
  invitation: string,
  channel: "email" | "link" = "email",
): Promise<InvitationDelivery> {
  return request<InvitationDelivery>(
    "POST",
    `/invitations/${encodeURIComponent(invitation)}/deliveries`,
    { channel },
    { workspace },
  );
}

export function revokeInvitation(workspace: string, invitation: string): Promise<void> {
  return request<void>("DELETE", `/invitations/${encodeURIComponent(invitation)}`, undefined, { workspace });
}

export function invitationPreviewQuery(token: string) {
  return queryOptions({
    queryKey: ["invitation-preview", token],
    queryFn: () => request<InvitationPreview>("GET", "/invitations/preview", undefined, tokenHeader(token)),
    retry: false,
  });
}

/*
 * acceptInvitation makes the signed-in user a member of the workspace
 * the invitation is for.
 */
export function acceptInvitation(token: string): Promise<Membership> {
  return request<Membership>("POST", "/user/memberships", { invitation_token: token });
}

/*
 * signUpWithInvitation creates the account of an invitee. The server
 * trusts the invited address as proven and makes the new user a member
 * of the workspace at once.
 */
export function signUpWithInvitation(input: {
  name: string;
  email: string;
  password: string;
  invitation_token: string;
}): Promise<Me> {
  return request<Me>("POST", "/users", input);
}
