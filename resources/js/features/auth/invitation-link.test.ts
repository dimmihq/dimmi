import { ApiError } from "@/lib/api/client";

import {
  alreadyJoined,
  emailMismatch,
  endedBy,
  endedText,
  forgetInvitationToken,
  takeInvitationToken,
} from "./invitation-link";

afterEach(() => {
  window.sessionStorage.clear();
  window.history.replaceState(null, "", "/");
});

describe("takeInvitationToken", () => {
  it("takes the token from the address, keeps it and removes it from the address", () => {
    window.history.replaceState(null, "", "/invitation?error=email_mismatch#token=inv-secret");

    expect(takeInvitationToken()).toBe("inv-secret");
    expect(window.location.hash).toBe("");
    expect(window.location.search).toBe("?error=email_mismatch");
    expect(takeInvitationToken()).toBe("inv-secret");
  });

  it("finds nothing without a token, and nothing once forgotten", () => {
    window.history.replaceState(null, "", "/invitation");
    expect(takeInvitationToken()).toBe("");

    window.history.replaceState(null, "", "/invitation#token=inv-secret");
    takeInvitationToken();
    forgetInvitationToken();
    expect(takeInvitationToken()).toBe("");
  });
});

describe("endedBy", () => {
  it("reads why the server refused an invitation", () => {
    expect(endedBy(new ApiError(404, "not_found", "Not found."))).toBe("invalid");
    expect(endedBy(new ApiError(410, "invitation_expired", "Expired."))).toBe("expired");
    expect(endedBy(new ApiError(410, "invitation_revoked", "Revoked."))).toBe("revoked");
  });

  it("leaves other failures alone", () => {
    expect(endedBy(new ApiError(500, "internal", "Oops."))).toBeNull();
    expect(endedBy(new Error("offline"))).toBeNull();
  });
});

describe("alreadyJoined and emailMismatch", () => {
  it("tell a person already in the workspace and an account at another address", () => {
    expect(alreadyJoined(new ApiError(409, "already_member", "Member."))).toBe(true);
    expect(alreadyJoined(new ApiError(409, "invitation_already_accepted", "Accepted."))).toBe(true);
    expect(alreadyJoined(new ApiError(410, "invitation_expired", "Expired."))).toBe(false);
    expect(emailMismatch(new ApiError(403, "invitation_email_mismatch", "Mismatch."))).toBe(true);
    expect(emailMismatch(new ApiError(403, "forbidden", "No."))).toBe(false);
  });
});

describe("endedText", () => {
  it("names the inviter when known", () => {
    expect(endedText("expired", "Grace")).toEqual({
      title: "This invitation has expired",
      description: "Ask Grace to send a new one.",
    });
    expect(endedText("revoked", null).description).toBe(
      "Ask the person who invited you to invite you again if you still need access.",
    );
  });

  it("explains an accepted and an invalid invitation", () => {
    expect(endedText("accepted", "Grace").title).toBe("This invitation was already accepted");
    expect(endedText("invalid", null).title).toBe("This invitation link is not valid");
  });
});
