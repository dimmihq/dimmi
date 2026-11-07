/*
 * The identity route for Express: app.get("/api/dimmi/token",
 * dimmiIdentity((req) => ...)). identify gets the request as Express
 * hands it over, with your session middleware already run.
 */
import { type Identify, type IdentityOptions, identityToken, tokenHeaders } from "./index";

interface ExpressResponse {
  set(headers: Record<string, string>): ExpressResponse;
  status(code: number): ExpressResponse;
  send(body: string): unknown;
}

export function dimmiIdentity<Request>(identify: Identify<Request>, options: IdentityOptions = {}) {
  return async (req: Request, res: ExpressResponse, next: (error?: unknown) => void) => {
    try {
      const token = await identityToken(identify, req, options);
      res.set(tokenHeaders).status(200).send(JSON.stringify({ token }));
    } catch (error) {
      next(error);
    }
  };
}
