import { createFileRoute, redirect } from "@tanstack/react-router";

/*
 * A ticket's old address forwards to the inbox, where tickets open
 * beside the ticket list.
 */
export const Route = createFileRoute("/d/tickets/$ticket")({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/d/inbox/$view/$conversation",
      params: { view: "tickets", conversation: params.ticket },
      replace: true,
    });
  },
});
