import { createFileRoute, redirect } from "@tanstack/react-router";

import { Onboarding } from "@/features/onboarding/onboarding";
import { currentWorkspace } from "@/features/shell/workspace";
import { workspaceQuery } from "@/lib/api/workspace";
import { currentUser } from "@/lib/session";

interface OnboardingSearch {
  new?: true;
}

export const Route = createFileRoute("/onboarding")({
  validateSearch: (search: Record<string, unknown>): OnboardingSearch => ({
    new: search.new ? true : undefined,
  }),
  loaderDeps: ({ search }) => ({ fresh: search.new === true }),
  loader: async ({ context, location, deps }) => {
    const me = await currentUser(context.queryClient);
    if (!me) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
    if (deps.fresh) {
      return { me, workspace: null, canSetUp: true, canLeave: me.workspaces.length > 0 };
    }
    const current = currentWorkspace(me.workspaces);
    const workspace = current
      ? await context.queryClient.query({ ...workspaceQuery(current.id), staleTime: 60_000 })
      : null;
    if (workspace?.onboarding.completed_at) {
      throw redirect({ to: "/d" });
    }

    return { me, workspace, canSetUp: current === null || current.role !== "member", canLeave: false };
  },
  component: OnboardingPage,
});

function OnboardingPage() {
  const { me, workspace, canSetUp, canLeave } = Route.useLoaderData();

  return <Onboarding me={me} workspace={workspace} canSetUp={canSetUp} canLeave={canLeave} />;
}
