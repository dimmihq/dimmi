import { createFileRoute, redirect } from "@tanstack/react-router";

import { PageOutlet } from "@/features/layout/page-outlet";
import { currentUser } from "@/lib/session";

export const Route = createFileRoute("/d")({
  beforeLoad: async ({ context, location }) => {
    if (!(await currentUser(context.queryClient))) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
  },
  component: () => <PageOutlet />,
});
