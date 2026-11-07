/*
 * dimmi.config.ts holds how your app tells dimmi who is signed in: an
 * adapter for your auth library, or your own identify function, plus the
 * signing options. It is read only on the server.
 */
import type { IdentityOptions, IdentityUser } from "@dimmi/server";

export type { IdentityUser } from "@dimmi/server";

export type ServerIdentify = () => IdentityUser | null | undefined | Promise<IdentityUser | null | undefined>;

export interface DimmiConfig extends IdentityOptions {
  identify: ServerIdentify;
}

/*
 * defineDimmi types your config.
 */
export function defineDimmi(config: DimmiConfig): DimmiConfig {
  return config;
}

/*
 * Extend adds to or overrides what an adapter read, or returns null to
 * treat the user as signed out.
 */
export type Extend<Source> = (source: Source) => Partial<IdentityUser> | null | Promise<Partial<IdentityUser> | null>;

/*
 * extended applies an adapter's extend to the user it mapped.
 */
export async function extended<Source>(
  user: IdentityUser,
  source: Source,
  extend: Extend<Source> | undefined,
): Promise<IdentityUser | null> {
  if (!extend) {
    return user;
  }
  const more = await extend(source);
  return more === null ? null : { ...user, ...more, id: more.id ?? user.id };
}
