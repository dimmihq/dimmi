import { fireEvent, screen, waitFor } from "@testing-library/react";

import type { InvitationPreview } from "@/lib/api/invitations";

import { InvitationPage } from "./invitation-page";
import { config, renderWithQueries } from "./test-utils";

afterEach(() => {
  vi.unstubAllGlobals();
});

const preview: InvitationPreview = {
  object: "invitation_preview",
  workspace: { id: "ws_acme", name: "Acme", avatar_url: null },
  inviter: { name: "Grace" },
  role: "admin",
  email_masked: "a***@acme.com",
  status: "pending",
  expires_at: "2026-11-10T12:00:00Z",
  has_account: false,
};

/*
 * serve answers each API path with its own status and body, whatever
 * order the page asks in.
 */
function serve(routes: Record<string, [number, unknown]>) {
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const [status, body] = routes[`${init?.method ?? "GET"} ${url}`] ?? [404, { code: "not_found" }];

    return Promise.resolve(new Response(JSON.stringify(body), { status }));
  });
  vi.stubGlobal("fetch", fetch);

  return fetch;
}

function fill(label: RegExp, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

describe("InvitationPage", () => {
  it("offers a signup form to an invitee without an account, sending the token", async () => {
    const fetch = serve({
      "GET /api/v1/me": [401, { code: "unauthenticated" }],
      "GET /api/v1/invitations/preview": [200, { data: preview }],
      "POST /api/v1/users": [403, { code: "invitation_email_mismatch", message: "Wrong address." }],
    });
    renderWithQueries(<InvitationPage token="inv-secret" config={config} />);

    expect(await screen.findByText("Join Acme")).toBeTruthy();
    expect(screen.getByText("Grace invited you to join as an admin.")).toBeTruthy();
    const previewCall = fetch.mock.calls.find(([url]) => url === "/api/v1/invitations/preview");
    const headers = (previewCall?.[1]?.headers ?? {}) as Record<string, string>;
    expect(headers["Dimmi-Invitation-Token"]).toBe("inv-secret");

    fill(/full name/i, "Ada Lovelace");
    fill(/email/i, "ada@other.com");
    fill(/password/i, "analytical-engine-1843");
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    expect(await screen.findByText("Use the address the invitation was sent to (a***@acme.com).")).toBeTruthy();
    const signup = fetch.mock.calls.find(([url, init]) => url === "/api/v1/users" && init?.method === "POST");
    expect(JSON.parse(String(signup?.[1]?.body)).invitation_token).toBe("inv-secret");
  });

  it("says an expired invitation has ended and who can send a new one", async () => {
    serve({
      "GET /api/v1/me": [401, { code: "unauthenticated" }],
      "GET /api/v1/invitations/preview": [200, { data: { ...preview, status: "expired" } }],
    });
    renderWithQueries(<InvitationPage token="inv-secret" config={config} />);

    expect(await screen.findByText("This invitation has expired")).toBeTruthy();
    expect(screen.getByText("Ask Grace to send a new one.")).toBeTruthy();
  });

  it("says an unknown link is not valid", async () => {
    serve({ "GET /api/v1/me": [401, { code: "unauthenticated" }] });
    renderWithQueries(<InvitationPage token="nope" config={config} />);

    await waitFor(() => expect(screen.getByText("This invitation link is not valid")).toBeTruthy());
  });

  it("asks someone with an account to sign in first", async () => {
    serve({
      "GET /api/v1/me": [401, { code: "unauthenticated" }],
      "GET /api/v1/invitations/preview": [200, { data: { ...preview, has_account: true } }],
    });
    renderWithQueries(<InvitationPage token="inv-secret" config={config} />);

    expect(await screen.findByRole("button", { name: "Sign in to accept" })).toBeTruthy();
    const github = screen.getByRole("link", { name: "Continue with GitHub" });
    expect(github.getAttribute("href")).toBe("/auth/github/redirect?redirect=%2Finvitation&invitation=inv-secret");
  });
});
