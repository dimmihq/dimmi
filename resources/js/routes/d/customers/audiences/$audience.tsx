import { createFileRoute, redirect } from "@tanstack/react-router";

/*
 * An audience's old address opens it in the drawer over the audiences
 * list, so links from workflows and elsewhere keep working.
 */
export const Route = createFileRoute("/d/customers/audiences/$audience")({
  beforeLoad: ({ params }) => {
    throw redirect({ to: "/d/customers/audiences", search: { audience: params.audience }, replace: true });
  },
});
