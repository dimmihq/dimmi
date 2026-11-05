/*
 * A small OpenID Connect provider on this machine, so the end-to-end
 * tests can link and sign in with a provider without reaching the
 * internet. The release binary talks to it as it would to Google: it
 * reads the discovery document, trades a code for really signed tokens
 * and checks them against the published keys. Like a real provider it
 * binds each code to the PKCE challenge and callback it was issued for
 * and honours it once. Its sign-in page asks only for an address; the
 * account is named by that address, so every test brings its own.
 */
import { createHash, generateKeyPairSync, randomBytes, sign } from "node:crypto";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";

export interface IdentityProvider {
  issuer: string;
  clientId: string;
  clientSecret: string;
  close: () => void;
}

/*
 * Grant is what a code was issued for.
 */
interface Grant {
  email: string;
  nonce: string;
  challenge: string;
  redirectUri: string;
}

const keyId = "e2e-key";

function base64url(data: Buffer | string): string {
  return Buffer.from(data).toString("base64url");
}

function random(): string {
  return randomBytes(24).toString("base64url");
}

function send(res: ServerResponse, status: number, type: string, body: string) {
  res.writeHead(status, { "Content-Type": type, "Cache-Control": "no-store" });
  res.end(body);
}

function json(res: ServerResponse, status: number, body: unknown) {
  send(res, status, "application/json", JSON.stringify(body));
}

async function readForm(req: IncomingMessage): Promise<URLSearchParams> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) {
    chunks.push(chunk as Buffer);
  }

  return new URLSearchParams(Buffer.concat(chunks).toString());
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (char) => `&#${char.charCodeAt(0)};`);
}

/*
 * signInPage is the provider's own page: one field for the address, and
 * the attempt carried along in hidden fields.
 */
function signInPage(query: URLSearchParams): string {
  const hidden = ["client_id", "redirect_uri", "state", "nonce", "code_challenge"]
    .map((name) => `<input type="hidden" name="${name}" value="${escapeHtml(query.get(name) ?? "")}">`)
    .join("");

  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Test provider</title>
<link rel="icon" href="data:,"></head><body><main><h1>Sign in to the test provider</h1>
<form method="get" action="/approve">${hidden}<label>Provider email <input name="email" type="email"></label>
<button type="submit">Approve</button></form></main></body></html>`;
}

/*
 * startIdentityProvider starts the provider on a free port of this
 * machine and resolves once it listens.
 */
export async function startIdentityProvider(): Promise<IdentityProvider> {
  const { privateKey, publicKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
  const jwk = { ...publicKey.export({ format: "jwk" }), kid: keyId, alg: "RS256", use: "sig" };
  const clientId = `e2e-${random()}`;
  const clientSecret = random();
  const codes = new Map<string, Grant>();
  let issuer = "";

  const idToken = (grant: Grant): string => {
    const now = Math.floor(Date.now() / 1000);
    const header = base64url(JSON.stringify({ alg: "RS256", kid: keyId, typ: "JWT" }));
    const claims = base64url(
      JSON.stringify({
        iss: issuer,
        aud: clientId,
        sub: createHash("sha256").update(grant.email).digest("hex").slice(0, 24),
        email: grant.email,
        email_verified: true,
        name: "Test Provider Person",
        nonce: grant.nonce,
        iat: now,
        exp: now + 600,
      }),
    );

    return `${header}.${claims}.${base64url(sign("RSA-SHA256", Buffer.from(`${header}.${claims}`), privateKey))}`;
  };

  const client = (req: IncomingMessage, form: URLSearchParams): boolean => {
    const basic = Buffer.from((req.headers.authorization ?? "").replace(/^Basic /, ""), "base64").toString();

    return basic === `${clientId}:${clientSecret}` || form.get("client_secret") === clientSecret;
  };

  const token = async (req: IncomingMessage, res: ServerResponse) => {
    const form = await readForm(req);
    const grant = codes.get(form.get("code") ?? "");
    codes.delete(form.get("code") ?? "");
    const verifier = form.get("code_verifier") ?? "";
    const proven = grant && base64url(createHash("sha256").update(verifier).digest()) === grant.challenge;
    if (!(grant && proven && client(req, form) && form.get("redirect_uri") === grant.redirectUri)) {
      json(res, 400, { error: "invalid_grant" });
      return;
    }

    json(res, 200, { access_token: random(), token_type: "Bearer", expires_in: 3600, id_token: idToken(grant) });
  };

  const approve = (query: URLSearchParams, res: ServerResponse) => {
    const code = random();
    const redirectUri = query.get("redirect_uri") ?? "";
    codes.set(code, {
      email: query.get("email") ?? "",
      nonce: query.get("nonce") ?? "",
      challenge: query.get("code_challenge") ?? "",
      redirectUri,
    });
    const back = new URL(redirectUri);
    back.searchParams.set("code", code);
    back.searchParams.set("state", query.get("state") ?? "");
    res.writeHead(302, { Location: back.toString() });
    res.end();
  };

  const server = createServer((req, res) => {
    const url = new URL(req.url ?? "/", issuer);
    const route = `${req.method} ${url.pathname}`;

    if (route === "GET /.well-known/openid-configuration") {
      json(res, 200, {
        issuer,
        authorization_endpoint: `${issuer}/authorize`,
        token_endpoint: `${issuer}/token`,
        jwks_uri: `${issuer}/jwks`,
        response_types_supported: ["code"],
        subject_types_supported: ["public"],
        id_token_signing_alg_values_supported: ["RS256"],
      });
    } else if (route === "GET /jwks") {
      json(res, 200, { keys: [jwk] });
    } else if (route === "GET /authorize") {
      send(res, 200, "text/html; charset=utf-8", signInPage(url.searchParams));
    } else if (route === "GET /approve") {
      approve(url.searchParams, res);
    } else if (route === "POST /token") {
      void token(req, res);
    } else {
      json(res, 404, { error: "not_found" });
    }
  });

  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  issuer = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;

  return { issuer, clientId, clientSecret, close: () => server.close() };
}
