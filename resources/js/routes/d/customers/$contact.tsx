import { createFileRoute, redirect } from "@tanstack/react-router";

/*
 * A customer's old address opens them in the drawer over the people list,
 * so links from the inbox, mails and companies keep working.
 */
export const Route = createFileRoute("/d/customers/$contact")({
  beforeLoad: ({ params }) => {
    throw redirect({ to: "/d/customers", search: { contact: params.contact }, replace: true });
  },
});
