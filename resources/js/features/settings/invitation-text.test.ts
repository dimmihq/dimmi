import type { InvitationResult } from "@/lib/api/invitations";

import { expiryLabel, outcome, roleName, splitEmails } from "./invitation-text";

function result(kind: InvitationResult["result"], error?: InvitationResult["error"]): InvitationResult {
  return { email: "ada@example.com", result: kind, invitation: null, error };
}

describe("splitEmails", () => {
  it("reads addresses separated by commas, semicolons, spaces and new lines", () => {
    expect(splitEmails("ada@example.com, grace@example.com;alan@example.com\nkatherine@example.com  x@y.z")).toEqual([
      "ada@example.com",
      "grace@example.com",
      "alan@example.com",
      "katherine@example.com",
      "x@y.z",
    ]);
  });

  it("keeps each address once whatever its case", () => {
    expect(splitEmails("Ada@Example.com ada@example.com")).toEqual(["Ada@Example.com"]);
  });

  it("finds nothing in blank input", () => {
    expect(splitEmails(" ,\n ; ")).toEqual([]);
  });
});

describe("outcome", () => {
  it("says the invitation went out for new, resent and reactivated addresses", () => {
    for (const kind of ["created", "resent", "reactivation"] as const) {
      expect(outcome(result(kind))).toEqual({ tone: "sent", text: "Invitation sent" });
    }
  });

  it("says a member needs no invitation", () => {
    expect(outcome(result("already_member"))).toEqual({ tone: "skipped", text: "Already a member" });
  });

  it("gives the server's reason for an address it refused, or a general one", () => {
    expect(outcome(result("invalid", { code: "invalid", message: "Not an email address." }))).toEqual({
      tone: "failed",
      text: "Not an email address.",
    });
    expect(outcome(result("invalid")).text).toBe("This address cannot be invited.");
  });
});

describe("roleName", () => {
  it("names the role in a sentence", () => {
    expect(roleName("admin")).toBe("an admin");
    expect(roleName("member")).toBe("a member");
  });
});

describe("expiryLabel", () => {
  it("says the date the invitation stops working, and nothing for a broken date", () => {
    expect(expiryLabel("2026-11-10T12:00:00Z", "en-US")).toBe("Expires Nov 10, 2026");
    expect(expiryLabel("soon", "en-US")).toBe("");
  });
});
