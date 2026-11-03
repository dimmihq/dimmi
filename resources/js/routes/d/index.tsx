import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/d/")({
  beforeLoad: () => {
    throw redirect({ to: "/d/inbox/$view", params: { view: "unassigned" } });
  },
});
