import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";

import { currentUser } from "@/lib/session";

export const Route = createFileRoute("/d")({
  beforeLoad: async ({ context, location }) => {
    if (!(await currentUser(context.queryClient))) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
  },
  component: Outlet,
});
