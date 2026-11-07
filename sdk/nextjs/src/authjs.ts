/*
 * Signs in Auth.js users: identify: authjs(auth), with auth exported by
 * your auth.ts. The session carries no flag saying the email was
 * verified, so it never counts as verified unless extend says so.
 */
import type { IdentityUser } from "@dimmi/server";

import { type Extend, extended, type ServerIdentify } from "./config";

export interface AuthjsSession {
  user?: { id?: string | null; email?: string | null; name?: string | null; image?: string | null } & Record<
    string,
    unknown
  >;
}

export function authjs<Session extends AuthjsSession>(
  auth: () => Promise<Session | null>,
  options: { extend?: Extend<Session> } = {},
): ServerIdentify {
  return async () => {
    const session = await auth();
    if (!session?.user) {
      return null;
    }
    if (!session.user.id) {
      throw new Error("[dimmi] identify_user_invalid: expose user.id in your Auth.js session callback.");
    }
    const user: IdentityUser = {
      id: session.user.id,
      email: session.user.email,
      emailVerified: false,
      name: session.user.name,
      avatarUrl: session.user.image,
    };
    return extended(user, session, options.extend);
  };
}
