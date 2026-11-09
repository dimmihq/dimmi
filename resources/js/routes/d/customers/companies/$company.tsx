import { createFileRoute, redirect } from "@tanstack/react-router";

/*
 * A company's old address opens it in the drawer over the companies list,
 * so links from mails and elsewhere keep working.
 */
export const Route = createFileRoute("/d/customers/companies/$company")({
  beforeLoad: ({ params }) => {
    throw redirect({ to: "/d/customers/companies", search: { company: params.company }, replace: true });
  },
});
