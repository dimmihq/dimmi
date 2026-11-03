import type { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext } from "@tanstack/react-router";

import { PageOutlet } from "@/features/layout/page-outlet";

/*
 * What every route can reach: the query cache the server state lives in.
 */
export interface RouterContext {
  queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => <PageOutlet />,
});
