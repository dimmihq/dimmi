import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useRouter } from "@tanstack/react-router";

import { TwoFactorForm } from "@/features/auth/two-factor-form";
import { challengeQuery, useSignedIn } from "@/features/auth/use-signed-in";
import { safeRedirect } from "@/lib/location";

interface TwoFactorSearch {
  redirect?: string;
}

export const Route = createFileRoute("/_auth/login_/two-factor")({
  validateSearch: (search: Record<string, unknown>): TwoFactorSearch => ({
    redirect: typeof search.redirect === "string" ? search.redirect : undefined,
  }),
  component: TwoFactorPage,
});

/*
 * TwoFactorPage asks for the second factor of the sign-in under way. The
 * challenge itself lives in a cookie the browser cannot read; the
 * methods it offers come along from the form that started it, and a page
 * opened straight from a provider's callback asks with both.
 */
function TwoFactorPage() {
  const search = Route.useSearch();
  const target = safeRedirect(search.redirect);
  const queryClient = useQueryClient();
  const router = useRouter();
  const { data: challenge } = useQuery(challengeQuery);
  const signedIn = useSignedIn(target);

  return (
    <TwoFactorForm
      methods={challenge?.methods ?? null}
      onSignedIn={(me) => {
        queryClient.removeQueries({ queryKey: challengeQuery.queryKey });
        signedIn(me);
      }}
      onExpired={() => {
        queryClient.removeQueries({ queryKey: challengeQuery.queryKey });
        router.history.push(`/login?${new URLSearchParams({ error: "two_factor_challenge_expired" }).toString()}`);
      }}
    />
  );
}
