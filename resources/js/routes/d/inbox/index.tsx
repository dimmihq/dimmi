import { createFileRoute, redirect } from "@tanstack/react-router";

import { lastView } from "@/features/inbox/views/last-view";

export const Route = createFileRoute("/d/inbox/")({
  beforeLoad: () => {
    throw redirect({ to: "/d/inbox/$view", params: { view: lastView("unassigned") } });
  },
});
