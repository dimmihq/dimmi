/*
 * Signs in Clerk users: identify: clerk(). The profile is read from
 * Clerk's Backend API and kept for five minutes per user, at most a
 * thousand users, so a busy site stays inside Clerk's rate limits.
 */
import type { IdentityUser } from "@dimmi/server";

import { type Extend, extended, type ServerIdentify } from "./config.js";

type ClerkUser = NonNullable<Awaited<ReturnType<typeof import("@clerk/nextjs/server").currentUser>>>;

const lifetime = 5 * 60 * 1000;
const capacity = 1000;

export function clerk(options: { extend?: Extend<ClerkUser> } = {}): ServerIdentify {
  const cache = new Map<string, { user: ClerkUser; at: number }>();

  return async () => {
    const { auth, currentUser } = await import("@clerk/nextjs/server");
    const { userId, orgId } = await auth();
    if (!userId) {
      return null;
    }
    const user = await cached(cache, userId, currentUser);
    if (!user) {
      return null;
    }
    const email = user.primaryEmailAddress;
    const mapped: IdentityUser = {
      id: user.id,
      email: email?.emailAddress,
      emailVerified: email?.verification?.status === "verified",
      name: user.fullName,
      avatarUrl: user.imageUrl,
      signedUpAt: user.createdAt,
      company: orgId ? { id: orgId } : null,
    };
    return extended(mapped, user, options.extend);
  };
}

async function cached(
  cache: Map<string, { user: ClerkUser; at: number }>,
  id: string,
  load: () => Promise<ClerkUser | null>,
): Promise<ClerkUser | null> {
  const hit = cache.get(id);
  if (hit && Date.now() - hit.at < lifetime) {
    return hit.user;
  }
  const user = await load();
  if (!user) {
    return null;
  }
  cache.delete(id);
  cache.set(id, { user, at: Date.now() });
  if (cache.size > capacity) {
    const oldest = cache.keys().next().value;
    if (oldest !== undefined) {
      cache.delete(oldest);
    }
  }
  return user;
}
