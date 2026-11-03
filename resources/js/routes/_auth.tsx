import { createFileRoute } from "@tanstack/react-router";

import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { PageOutlet } from "@/features/layout/page-outlet";
import { authConfigQuery } from "@/lib/api/auth";

export const Route = createFileRoute("/_auth")({
  loader: ({ context }) => context.queryClient.ensureQueryData(authConfigQuery),
  component: AuthLayout,
});

/*
 * The frame of the sign-in pages: the theme switch, and the page's card
 * in the middle of the screen. The card carries the logo, so the frame
 * shows no second one.
 */
function AuthLayout() {
  return (
    <div className="flex min-h-screen flex-col bg-background-full">
      <header className="flex items-center justify-end p-4 sm:p-6">
        <ThemeToggle appearance="segmented" size="small" />
      </header>
      <main className="flex flex-1 items-start justify-center px-4 pt-8 pb-16 sm:items-center sm:pt-0">
        <PageOutlet className="flex w-full justify-center" />
      </main>
    </div>
  );
}
