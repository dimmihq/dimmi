import { createFileRoute, redirect } from "@tanstack/react-router";

import { AwaitingAccessPage } from "@/features/auth/awaiting-access";
import { currentUser } from "@/lib/session";

/*
 * The page of a person whose only way into a workspace waits for an
 * admin to enable them. Anyone else is sent on: to sign in, or to work.
 */
export const Route = createFileRoute("/_auth/waiting")({
  loader: async ({ context, location }) => {
    const me = await currentUser(context.queryClient);
    if (!me) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
    const awaiting = me.awaiting_access ?? [];
    if (me.workspaces.length > 0 || awaiting.length === 0) {
      throw redirect({ to: "/d" });
    }

    return { awaiting };
  },
  component: WaitingRoute,
});

function WaitingRoute() {
  const { awaiting } = Route.useLoaderData();

  return <AwaitingAccessPage awaiting={awaiting} />;
}
