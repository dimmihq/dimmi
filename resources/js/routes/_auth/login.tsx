import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";

import { LoginForm } from "@/features/auth/login-form";
import { oauthError } from "@/features/auth/oauth-errors";
import { useSignedIn } from "@/features/auth/use-signed-in";
import { authConfigQuery } from "@/lib/api/auth";
import { safeRedirect } from "@/lib/location";
import { currentUser } from "@/lib/session";

interface LoginSearch {
  redirect?: string;
  error?: string;
  reauth?: boolean;
}

export const Route = createFileRoute("/_auth/login")({
  validateSearch: (search: Record<string, unknown>): LoginSearch => ({
    redirect: typeof search.redirect === "string" ? search.redirect : undefined,
    error: typeof search.error === "string" ? search.error : undefined,
    reauth: search.reauth === 1 || search.reauth === "1" || search.reauth === true ? true : undefined,
  }),
  beforeLoad: async ({ context, search }) => {
    if (!search.reauth && (await currentUser(context.queryClient))) {
      throw redirect({ href: safeRedirect(search.redirect) });
    }
  },
  component: LoginPage,
});

function LoginPage() {
  const search = Route.useSearch();
  const { data: config } = useSuspenseQuery(authConfigQuery);
  const target = safeRedirect(search.redirect);

  return (
    <LoginForm
      config={config}
      redirect={target}
      initialError={oauthError(search.error)}
      reauth={search.reauth}
      onSignedIn={useSignedIn(target)}
    />
  );
}
