/*
 * Entry of the browser app. The Go server renders the page this mounts
 * into, so the router only takes over once the page has loaded. Every
 * page sits inside the app's language, which English starts in before
 * the server says which language the person reads. Errors are reported
 * from the first line, so a crash while mounting is reported too.
 */
import "./app.css";

import { reactErrorHandler } from "@sentry/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRouter, RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { LocaleRoot } from "./features/shell/locale-root";
import { RouteError } from "./features/shell/route-error";
import { syncTheme } from "./features/theme/theme-sync";
import { startReporting, startTracing } from "./lib/errors";
import "./lib/i18n";
import { workspaceAddress } from "./features/shell/workspace";
import { routeTree } from "./route-tree.gen";

startReporting();
syncTheme();

const queryClient = new QueryClient({ defaultOptions: { queries: { refetchOnWindowFocus: false } } });
const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: "intent",
  defaultErrorComponent: RouteError,
  rewrite: workspaceAddress,
});
startTracing(router);

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

const root = document.getElementById("root");

if (root) {
  createRoot(root, {
    onUncaughtError: reactErrorHandler(),
    onCaughtError: reactErrorHandler(),
    onRecoverableError: reactErrorHandler(),
  }).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <LocaleRoot>
          <RouterProvider router={router} />
        </LocaleRoot>
      </QueryClientProvider>
    </StrictMode>,
  );
}
