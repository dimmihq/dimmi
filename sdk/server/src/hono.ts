/*
 * The identity route for Hono: app.get("/api/dimmi/token",
 * dimmiIdentity((c) => ...)). identify gets the context, so it can read
 * whatever your auth middleware set on it.
 */
import { type Identify, type IdentityOptions, identityToken, tokenResponse } from "./index";

export function dimmiIdentity<Context>(identify: Identify<Context>, options: IdentityOptions = {}) {
  return async (c: Context): Promise<Response> => tokenResponse(await identityToken(identify, c, options));
}
