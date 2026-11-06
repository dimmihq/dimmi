/*
 * Who is signed in, as routes need it before they render: the app's
 * pages require someone, the sign-in pages send someone already signed
 * in onward.
 */
import type { QueryClient } from "@tanstack/react-query";

import { type Me, meQuery } from "./api/auth";

/*
 * currentUser returns the signed-in caller, or null when the browser
 * holds no valid session. The answer is read fresh once it has been
 * invalidated or aged out, so a workspace made or joined during the
 * session is seen by the next route that asks.
 */
export function currentUser(queryClient: QueryClient): Promise<Me | null> {
  return queryClient.query({ ...meQuery, staleTime: 60_000 });
}
