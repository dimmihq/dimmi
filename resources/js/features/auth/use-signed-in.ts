/*
 * What every way of signing in does once the server answered: remember
 * who is signed in and go where the person was headed, or, when the
 * account asks for a second factor, keep the challenge and open the page
 * that answers it, still headed to the same place. A check of who is
 * signed in that started before the answer is stopped first, so its
 * older "nobody" cannot land afterwards and send the person back to the
 * sign-in page.
 */
import { queryOptions, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import { isChallenge, type Me, meQuery, type SignInResult, type TwoFactorChallenge } from "@/lib/api/auth";

/*
 * challengeQuery holds the challenge the last sign-in answered with, for
 * the page that asks for the second factor. It is never fetched: a page
 * opened without one, such as after a provider sign-in, asks with the
 * usual methods.
 */
export const challengeQuery = queryOptions<TwoFactorChallenge | null>({
  queryKey: ["two-factor-challenge"],
  queryFn: () => null,
  staleTime: Number.POSITIVE_INFINITY,
});

/*
 * challengePath returns the page that asks for the second factor, still
 * headed to the redirect.
 */
export function challengePath(redirect: string): string {
  return `/login/two-factor?${new URLSearchParams({ redirect }).toString()}`;
}

export function useSignedIn(redirect: string) {
  const queryClient = useQueryClient();
  const router = useRouter();

  return async (result: SignInResult) => {
    if (isChallenge(result)) {
      queryClient.setQueryData(challengeQuery.queryKey, result);
      router.history.push(challengePath(redirect));

      return;
    }

    await queryClient.cancelQueries({ queryKey: meQuery.queryKey });
    queryClient.setQueryData(meQuery.queryKey, result as Me);
    router.history.push(redirect);
  };
}
