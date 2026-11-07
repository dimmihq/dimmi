/*
 * Signs in Supabase users: identify: supabase(). The user is fetched
 * from Supabase Auth, which verifies the session, never read from the
 * cookie alone. It reads NEXT_PUBLIC_SUPABASE_URL and the publishable
 * key, or the anon key.
 */
import type { IdentityUser } from "@dimmi/server";

import { type Extend, extended, type ServerIdentify } from "./config";

type SupabaseUser = NonNullable<
  Awaited<ReturnType<ReturnType<typeof import("@supabase/ssr").createServerClient>["auth"]["getUser"]>>["data"]["user"]
>;

export function supabase(options: { extend?: Extend<SupabaseUser> } = {}): ServerIdentify {
  return async () => {
    const [{ createServerClient }, { cookies }] = await Promise.all([import("@supabase/ssr"), import("next/headers")]);
    const store = await cookies();
    const env = process.env;
    const key = env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY ?? env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? "";
    const client = createServerClient(env.NEXT_PUBLIC_SUPABASE_URL ?? "", key, {
      cookies: { getAll: () => store.getAll(), setAll: () => undefined },
    });
    const { data } = await client.auth.getUser();
    const found = data.user;
    if (!found) {
      return null;
    }
    const meta = found.user_metadata ?? {};
    const user: IdentityUser = {
      id: found.id,
      email: found.email,
      emailVerified: found.email_confirmed_at != null,
      name: text(meta.full_name) ?? text(meta.name),
      avatarUrl: text(meta.avatar_url),
      signedUpAt: found.created_at,
    };
    return extended(user, found, options.extend);
  };
}

function text(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}
