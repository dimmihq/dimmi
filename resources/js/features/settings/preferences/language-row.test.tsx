import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";

import { config } from "@/features/auth/test-utils";
import { authConfigQuery, type Locale, type Me, meQuery } from "@/lib/api/auth";
import { type Workspace, workspaceQuery } from "@/lib/api/workspace";

import { LanguageRow, WorkspaceLanguageRow } from "./language-row";

const me: Me = {
  object: "me",
  user: {
    id: "usr_1",
    name: "Ada Lovelace",
    email: "ada@example.com",
    email_verified: true,
    has_password: true,
    two_factor_enabled: false,
    avatar_url: null,
    locale: null,
    timezone: null,
  },
  workspaces: [{ id: "ws_1", name: "Acme", slug: "acme", role: "owner" }],
  instance: { edition: "cloud", version: "dev" },
  locale: "en",
};

const workspace: Workspace = {
  id: "ws_1",
  name: "Acme",
  slug: "acme",
  logo_url: null,
  timezone: "UTC",
  locale: "en",
  test_mode: false,
  require_two_factor: false,
  seen_receipts: true,
  seen_receipt_shows_name: false,
  customer_facing_name: null,
  onboarding: { step: "done", completed_at: "2026-11-01T00:00:00Z" },
  deleted_at: null,
  purge_after: null,
};

/*
 * renderRows shows both language rows with the server serving the
 * languages given.
 */
function renderRows(locales: Locale[]) {
  const client = new QueryClient();
  client.setQueryData(authConfigQuery.queryKey, { ...config, locales });
  client.setQueryData(meQuery.queryKey, me);
  client.setQueryData(workspaceQuery("ws_1").queryKey, workspace);

  return render(
    <QueryClientProvider client={client}>
      <LanguageRow />
      <WorkspaceLanguageRow workspace="ws_1" />
    </QueryClientProvider>,
  );
}

describe("language rows", () => {
  it("are hidden while the server serves a single language", () => {
    const { container } = renderRows([{ code: "en", name: "English" }]);

    expect(container.innerHTML).toBe("");
  });

  it("offer every language by its own name, and the workspace's to follow", () => {
    renderRows([
      { code: "en", name: "English" },
      { code: "de", name: "Deutsch" },
    ]);

    expect(screen.getByText("Falls back to the workspace's language.")).toBeTruthy();
    expect(screen.getByRole("button", { name: /Workspace default \(English\)/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /English Workspace language/ })).toBeTruthy();
  });
});
