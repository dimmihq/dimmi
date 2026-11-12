/*
 * Signs in Better Auth users: identify: betterAuth(auth), with auth your
 * server instance. The session is read with the request's headers; with
 * the organization plugin, the active organization becomes the company.
 */
import type { IdentityUser } from "@dimmi/server";

import { type Extend, extended, type ServerIdentify } from "./config.js";

export interface BetterAuthSession {
  user: {
    id: string;
    email?: string | null;
    emailVerified?: boolean;
    name?: string | null;
    image?: string | null;
    createdAt?: Date | string;
  };
  session?: { activeOrganizationId?: string | null };
}

export interface BetterAuthServer<Session extends BetterAuthSession> {
  api: { getSession(input: { headers: Headers }): Promise<Session | null> };
}

export function betterAuth<Session extends BetterAuthSession>(
  auth: BetterAuthServer<Session>,
  options: { extend?: Extend<Session> } = {},
): ServerIdentify {
  return async () => {
    const { headers } = await import("next/headers");
    const session = await auth.api.getSession({ headers: await headers() });
    if (!session) {
      return null;
    }
    const organization = session.session?.activeOrganizationId;
    const user: IdentityUser = {
      id: session.user.id,
      email: session.user.email,
      emailVerified: session.user.emailVerified === true,
      name: session.user.name,
      avatarUrl: session.user.image,
      signedUpAt: session.user.createdAt,
      company: organization ? { id: organization } : null,
    };
    return extended(user, session, options.extend);
  };
}
