/*
 * The server half: a token for whoever is signed in, for a server action
 * the messenger calls, and the same answer as a route for apps that
 * prefer one. The token lives ten minutes and is minted per call.
 */
import { identityToken as sign, tokenHeaders, tokenResponse } from "@dimmi/server";

import type { DimmiConfig } from "./config.js";

/*
 * identityToken signs a token for the signed-in user, null for nobody.
 * Call it from a "use server" function you pass to <Dimmi identity>.
 */
export function identityToken(config: DimmiConfig): Promise<string | null> {
  return sign(() => config.identify(), undefined, config);
}

/*
 * createTokenRoute is the App Router handler for GET /api/dimmi/token:
 * export const GET = createTokenRoute(config).
 */
export function createTokenRoute(config: DimmiConfig): () => Promise<Response> {
  return async () => tokenResponse(await identityToken(config));
}

interface PagesResponse {
  setHeader(name: string, value: string): void;
  status(code: number): { send(body: string): void };
}

/*
 * createPagesHandler is the Pages Router API route:
 * export default createPagesHandler(config).
 */
export function createPagesHandler(config: DimmiConfig) {
  return async (_req: unknown, res: PagesResponse) => {
    const token = await identityToken(config);
    for (const [name, value] of Object.entries(tokenHeaders)) {
      res.setHeader(name, value);
    }
    res.status(200).send(JSON.stringify({ token }));
  };
}
