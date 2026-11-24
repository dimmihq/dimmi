/*
 * The routines list lives on Operator's page, as a tab; older links
 * land there.
 */
import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/d/routines/")({
  beforeLoad: () => {
    throw redirect({ to: "/d/operator/$tab", params: { tab: "routines" }, replace: true });
  },
});
