/*
 * Who is signed in, as routes need it before they render: the app's
 * pages require someone, the sign-in pages send someone already signed
 * in onward.
 */
import type { QueryClient } from "@tanstack/react-query";

import { type Me, meQuery } from "./api/auth";

/*
 * currentUser returns the signed-in caller, or null when the browser
 * holds no valid session.
 */
export function currentUser(queryClient: QueryClient): Promise<Me | null> {
  return queryClient.ensureQueryData(meQuery);
}
