/*
 * @dimmi/server answers one question for the messenger: who is signed in
 * on this request? You write identify, which reads your own session and
 * returns a user or null; the handler signs a ten-minute token for that
 * user and answers {"token": ...}, never cached. The secret comes from
 * DIMMI_IDENTITY_SECRET, or a private key from DIMMI_IDENTITY_PRIVATE_KEY
 * with its key id in DIMMI_IDENTITY_KID, and never reaches a browser.
 */
import { type IdentityUser, toClaims } from "./claims";
import { type Algorithm, privateKey, type SigningKey, secretKey, sign } from "./sign";

export { type AttributeValue, DimmiIdentityError, type IdentityCompany, type IdentityUser, toClaims } from "./claims";
export type { Algorithm } from "./sign";

export interface IdentityOptions {
  /* The shared secret, dmi_idsec_..., as shown once in dimmi. */
  secret?: string;
  /* The key id dimmi showed for the secret or public key. */
  kid?: string;
  /* A PKCS #8 PEM private key whose public key you uploaded to dimmi. */
  privateKey?: string;
  /* The private key's algorithm, found from the key when left out. */
  algorithm?: Algorithm;
  /* How long the token lives, in seconds: 600 unless set, 3600 at most. */
  expiresIn?: number;
  /* Who the token is for: "dimmi", a messenger app id or a workspace id. */
  audience?: string;
  /* A one-time value, for tokens that must work only once. */
  nonce?: string;
}

export type Identify<Context> = (
  context: Context,
) => IdentityUser | null | undefined | Promise<IdentityUser | null | undefined>;

/*
 * signIdentityToken signs a token for the user.
 */
export async function signIdentityToken(user: IdentityUser, options: IdentityOptions = {}): Promise<string> {
  const expiresIn = options.expiresIn ?? 600;
  if (expiresIn <= 0 || expiresIn > 3600) {
    throw new RangeError("[dimmi] expires_in_invalid: expiresIn is 1 to 3600 seconds.");
  }
  const now = Math.floor(Date.now() / 1000);
  const claims: Record<string, unknown> = { ...toClaims(user), iat: now, exp: now + expiresIn };
  if (options.audience) {
    claims.aud = options.audience;
  }
  if (options.nonce) {
    claims.nonce = options.nonce;
  }
  return sign(claims, await signingKey(options));
}

/*
 * createIdentityHandler makes a Fetch API handler for GET /api/dimmi/token:
 * React Router and Remix resource routes, SvelteKit, Astro, Nuxt,
 * Workers, Bun and Deno take it as is.
 */
export function createIdentityHandler(
  identify: Identify<Request>,
  options: IdentityOptions = {},
): (request: Request) => Promise<Response> {
  return async (request) => tokenResponse(await identityToken(identify, request, options));
}

/*
 * identityToken runs identify and signs a token for whoever it returns,
 * null when nobody is signed in.
 */
export async function identityToken<Context>(
  identify: Identify<Context>,
  context: Context,
  options: IdentityOptions = {},
): Promise<string | null> {
  const user = await identify(context);
  return user ? signIdentityToken(user, options) : null;
}

/*
 * tokenResponse is the answer the messenger expects: 200 with the token
 * or null, never cached, never a redirect to a login page.
 */
export function tokenResponse(token: string | null): Response {
  return new Response(JSON.stringify({ token }), { status: 200, headers: tokenHeaders });
}

/*
 * tokenHeaders keep the answer out of every cache.
 */
export const tokenHeaders: Record<string, string> = {
  "Content-Type": "application/json",
  "Cache-Control": "no-store, private",
};

async function signingKey(options: IdentityOptions): Promise<SigningKey> {
  const kid = options.kid ?? env("DIMMI_IDENTITY_KID");
  const pem = options.privateKey ?? env("DIMMI_IDENTITY_PRIVATE_KEY");
  if (pem) {
    if (!kid) {
      throw new Error("[dimmi] kid_required: set DIMMI_IDENTITY_KID to the key id dimmi shows for your public key.");
    }
    return privateKey(pem, kid, options.algorithm);
  }
  const secret = options.secret ?? env("DIMMI_IDENTITY_SECRET");
  if (!secret) {
    throw new Error(
      "[dimmi] secret_missing: set DIMMI_IDENTITY_SECRET to the secret from Settings > Identity verification.",
    );
  }
  return secretKey(secret.trim(), kid);
}

function env(name: string): string | undefined {
  const process = (globalThis as { process?: { env?: Record<string, string | undefined> } }).process;
  return process?.env?.[name] || undefined;
}
