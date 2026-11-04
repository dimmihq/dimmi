/*
 * The invitation link an invitee follows. Its token sits after the #,
 * so no server or referrer sees it; the page moves it into this tab's
 * session storage and out of the address, so it survives a detour
 * through signing in and never lingers in history or a shared screen.
 * Also the words for an invitation that can no longer be used.
 */
import { ApiError } from "@/lib/api/client";
import type { InvitationStatus } from "@/lib/api/invitations";
import { fragment } from "@/lib/location";

const key = "dimmi:invitation";

function session(): Storage | null {
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

/*
 * keep stores the token for this tab, and says whether it could.
 */
function keep(token: string): boolean {
  try {
    session()?.setItem(key, token);

    return true;
  } catch {
    return false;
  }
}

function kept(): string {
  try {
    return session()?.getItem(key) ?? "";
  } catch {
    return "";
  }
}

/*
 * takeInvitationToken returns the invitation's token: the one in the
 * address, which it then keeps for the tab and removes from the
 * address, or else the one kept earlier, or "".
 */
export function takeInvitationToken(): string {
  const linked = fragment("token");
  if (!linked) {
    return kept();
  }

  keep(linked);
  window.history.replaceState(window.history.state, "", `${window.location.pathname}${window.location.search}`);

  return linked;
}

/*
 * forgetInvitationToken drops the kept token once the invitation is
 * used, and says whether it could.
 */
export function forgetInvitationToken(): boolean {
  try {
    session()?.removeItem(key);

    return true;
  } catch {
    return false;
  }
}

/*
 * Ended is why an invitation cannot be used: it was never valid, or it
 * expired, was revoked or was already accepted.
 */
export type Ended = Exclude<InvitationStatus, "pending"> | "invalid";

/*
 * endedBy reads why the server refused an invitation, or null when the
 * refusal is about something else.
 */
export function endedBy(error: unknown): Ended | null {
  if (!(error instanceof ApiError)) {
    return null;
  }

  switch (error.code) {
    case "not_found":
      return "invalid";
    case "invitation_expired":
      return "expired";
    case "invitation_revoked":
      return "revoked";
    default:
      return null;
  }
}

/*
 * alreadyJoined reports whether the server refused only because the
 * person is in the workspace already, which is what they wanted.
 */
export function alreadyJoined(error: unknown): boolean {
  return error instanceof ApiError && (error.code === "already_member" || error.code === "invitation_already_accepted");
}

/*
 * emailMismatch reports whether the account is not the address the
 * invitation was sent to.
 */
export function emailMismatch(error: unknown): boolean {
  return error instanceof ApiError && error.code === "invitation_email_mismatch";
}

/*
 * endedText says why an invitation cannot be used and what to do,
 * naming the person who sent it when it is known.
 */
export function endedText(ended: Ended, inviter: string | null): { title: string; description: string } {
  const who = inviter ?? "the person who invited you";

  switch (ended) {
    case "expired":
      return { title: "This invitation has expired", description: `Ask ${who} to send a new one.` };
    case "revoked":
      return {
        title: "This invitation was revoked",
        description: `Ask ${who} to invite you again if you still need access.`,
      };
    case "accepted":
      return { title: "This invitation was already accepted", description: "Sign in to open the workspace." };
    default:
      return {
        title: "This invitation link is not valid",
        description: "Open the link from the mail again, or ask for a new invitation.",
      };
  }
}
