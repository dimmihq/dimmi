/*
 * The few parts of optional peer packages the adapters use, so this
 * package builds without installing every auth library. The host app's
 * own copies are what run.
 */
declare module "next/headers" {
  export function headers(): Promise<Headers>;
  export function cookies(): Promise<{ getAll(): { name: string; value: string }[] }>;
}

declare module "@clerk/nextjs/server" {
  export function auth(): Promise<{ userId: string | null; orgId?: string | null }>;
  export function currentUser(): Promise<{
    id: string;
    fullName: string | null;
    imageUrl: string;
    createdAt: number;
    primaryEmailAddress: { emailAddress: string; verification: { status: string } | null } | null;
  } | null>;
}

declare module "@supabase/ssr" {
  export function createServerClient(
    url: string,
    key: string,
    options: { cookies: { getAll(): { name: string; value: string }[]; setAll(cookies: unknown[]): void } },
  ): {
    auth: {
      getUser(): Promise<{
        data: {
          user: {
            id: string;
            email?: string;
            email_confirmed_at?: string | null;
            created_at?: string;
            user_metadata?: Record<string, unknown>;
          } | null;
        };
      }>;
    };
  };
}
