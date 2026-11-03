import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";

import { SignupForm } from "@/features/auth/signup-form";
import { useSignedIn } from "@/features/auth/use-signed-in";
import { authConfigQuery } from "@/lib/api/auth";
import { safeRedirect } from "@/lib/location";
import { currentUser } from "@/lib/session";

interface SignupSearch {
  redirect?: string;
}

export const Route = createFileRoute("/_auth/signup")({
  validateSearch: (search: Record<string, unknown>): SignupSearch => ({
    redirect: typeof search.redirect === "string" ? search.redirect : undefined,
  }),
  beforeLoad: async ({ context, search }) => {
    if (await currentUser(context.queryClient)) {
      throw redirect({ href: safeRedirect(search.redirect) });
    }
  },
  component: SignupPage,
});

function SignupPage() {
  const search = Route.useSearch();
  const { data: config } = useSuspenseQuery(authConfigQuery);
  const target = safeRedirect(search.redirect);

  return <SignupForm config={config} redirect={target} onSignedIn={useSignedIn(target)} />;
}
