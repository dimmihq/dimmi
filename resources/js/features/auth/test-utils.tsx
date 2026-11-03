/*
 * Rendering auth forms in tests: a fresh query cache and router per test,
 * and the API answered by a stub of fetch. The router lets the forms'
 * links to other pages render.
 */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRootRoute, createRouter, RouterContextProvider } from "@tanstack/react-router";
import { render } from "@testing-library/react";
import type { ReactNode } from "react";

import type { AuthConfig } from "@/lib/api/auth";

export const config: AuthConfig = {
  edition: "self_host",
  signup_mode: "open",
  magic_link: true,
  mail_configured: true,
  password_min_length: 10,
  providers: [{ key: "github", name: "GitHub", protocol: "oauth2", redirect_url: "/auth/github/redirect" }],
};

export function renderWithQueries(ui: ReactNode) {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });

  const router = createRouter({ routeTree: createRootRoute(), history: createMemoryHistory() });

  return render(
    <QueryClientProvider client={client}>
      <RouterContextProvider router={router}>{ui}</RouterContextProvider>
    </QueryClientProvider>,
  );
}

/*
 * answer makes every API call answer with the status and body, in turn.
 */
export function answer(...responses: [number, unknown][]) {
  const fetch = vi.fn();
  for (const [status, body] of responses) {
    fetch.mockResolvedValueOnce(new Response(JSON.stringify(body), { status }));
  }

  vi.stubGlobal("fetch", fetch);

  return fetch;
}
