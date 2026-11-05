import { createFileRoute, redirect } from "@tanstack/react-router";

import { Onboarding } from "@/features/onboarding/onboarding";
import { currentWorkspace } from "@/features/shell/workspace";
import { workspaceQuery } from "@/lib/api/workspace";
import { currentUser } from "@/lib/session";

export const Route = createFileRoute("/onboarding")({
  loader: async ({ context, location }) => {
    const me = await currentUser(context.queryClient);
    if (!me) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
    const current = currentWorkspace(me.workspaces);
    const workspace = current ? await context.queryClient.ensureQueryData(workspaceQuery(current.id)) : null;
    if (workspace?.onboarding.completed_at) {
      throw redirect({ to: "/d" });
    }

    return { me, workspace, canSetUp: current === null || current.role !== "member" };
  },
  component: OnboardingPage,
});

function OnboardingPage() {
  const { me, workspace, canSetUp } = Route.useLoaderData();

  return <Onboarding me={me} workspace={workspace} canSetUp={canSetUp} />;
}
