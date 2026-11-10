import { createFileRoute, redirect } from "@tanstack/react-router";

import { ConnectAppPage } from "@/features/auth/connect-app-page";
import { currentUser } from "@/lib/session";

export const Route = createFileRoute("/_auth/oauth/authorize")({
  beforeLoad: async ({ context, location }) => {
    if (!(await currentUser(context.queryClient))) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
  },
  component: ConnectAppRoute,
});

function ConnectAppRoute() {
  return <ConnectAppPage query={window.location.search.slice(1)} />;
}
