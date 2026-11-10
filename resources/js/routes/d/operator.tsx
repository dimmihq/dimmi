/*
 * Operator's own page moved onto the agent's page, as a tab beside
 * dimmi; older links land there.
 */
import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/d/operator")({
  beforeLoad: () => {
    throw redirect({ to: "/d/agent", search: { tab: "operator" }, replace: true });
  },
});
