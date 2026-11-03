import { createFileRoute, Outlet } from "@tanstack/react-router";

import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { authConfigQuery } from "@/lib/api/auth";

export const Route = createFileRoute("/_auth")({
  loader: ({ context }) => context.queryClient.ensureQueryData(authConfigQuery),
  component: AuthLayout,
});

/*
 * The frame of the sign-in pages: the product's name, the theme switch,
 * and the page's card in the middle of the screen.
 */
function AuthLayout() {
  return (
    <div className="flex min-h-screen flex-col bg-background-full">
      <header className="flex items-center justify-between p-4 sm:p-6">
        <a href="/" className="text-title-3-semibold text-text-primary">
          Dimmi
        </a>
        <ThemeToggle appearance="segmented" size="small" />
      </header>
      <main className="flex flex-1 items-start justify-center px-4 pt-8 pb-16 sm:items-center sm:pt-0">
        <Outlet />
      </main>
    </div>
  );
}
