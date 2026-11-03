import { createFileRoute, redirect } from "@tanstack/react-router";

import { PageOutlet } from "@/features/layout/page-outlet";
import { AppShell } from "@/features/shell/app-shell";
import { currentUser } from "@/lib/session";

export const Route = createFileRoute("/d")({
  beforeLoad: async ({ context, location }) => {
    if (!(await currentUser(context.queryClient))) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
  },
  component: SignedInApp,
});

function SignedInApp() {
  return (
    <AppShell>
      <PageOutlet className="flex h-full min-w-0 flex-1" />
    </AppShell>
  );
}
