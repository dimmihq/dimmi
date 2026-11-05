import { createFileRoute, redirect } from "@tanstack/react-router";

import { AccessRequired } from "@/features/auth/access-required";
import { PageOutlet } from "@/features/layout/page-outlet";
import { AppShell } from "@/features/shell/app-shell";
import { currentWorkspace } from "@/features/shell/workspace";
import { workspaceQuery } from "@/lib/api/workspace";
import { currentUser } from "@/lib/session";

export const Route = createFileRoute("/d")({
  beforeLoad: async ({ context, location }) => {
    const me = await currentUser(context.queryClient);
    if (!me) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
    const current = currentWorkspace(me.workspaces);
    if (!current) {
      throw redirect({ to: "/onboarding" });
    }
    const workspace = await context.queryClient.ensureQueryData(workspaceQuery(current.id)).catch(() => null);
    if (workspace && !workspace.onboarding.completed_at) {
      throw redirect({ to: "/onboarding" });
    }
  },
  component: SignedInApp,
});

function SignedInApp() {
  return (
    <AppShell>
      <AccessRequired>
        <PageOutlet className="flex h-full min-w-0 flex-1" />
      </AccessRequired>
    </AppShell>
  );
}
