/*
 * What every way of signing in does once the server started a session:
 * remember who is signed in, then go where the person was headed.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import { type Me, meQuery } from "@/lib/api/auth";

export function useSignedIn(redirect: string) {
  const queryClient = useQueryClient();
  const router = useRouter();

  return (me: Me) => {
    queryClient.setQueryData(meQuery.queryKey, me);
    router.history.push(redirect);
  };
}
