import { createHmac, generateKeyPairSync, verify } from "node:crypto";
import { dimmiIdentity as expressIdentity } from "./express";
import { dimmiIdentity as honoIdentity } from "./hono";
import { createIdentityHandler, DimmiIdentityError, signIdentityToken, toClaims } from "./index";

const secret = "dmi_idsec_0123456789abcdefghijklmnopqrstuvwxyzABCDEFG";

function parts(token: string) {
  const [header, payload, signature] = token.split(".");
  const decode = (part: string) => JSON.parse(Buffer.from(part, "base64url").toString());
  return { header: decode(header), claims: decode(payload), signed: `${header}.${payload}`, signature };
}

describe("identity tokens", () => {
  it("signs HS256 with the whole secret as typed and lives ten minutes", async () => {
    const token = await signIdentityToken({ id: 42, email: "ada@acme.test" }, { secret, kid: "isk_1" });
    const { header, claims, signed, signature } = parts(token);
    expect(header).toEqual({ alg: "HS256", typ: "JWT", kid: "isk_1" });
    expect(claims.sub).toBe("42");
    expect(claims.email).toBe("ada@acme.test");
    expect(claims.email_verified).toBeUndefined();
    expect(claims.exp - claims.iat).toBe(600);
    expect(createHmac("sha256", secret).update(signed).digest("base64url")).toBe(signature);
  });

  it("signs with an Ed25519, P-256 or RSA private key and sets its kid", async () => {
    const pairs = {
      EdDSA: generateKeyPairSync("ed25519"),
      ES256: generateKeyPairSync("ec", { namedCurve: "P-256" }),
      RS256: generateKeyPairSync("rsa", { modulusLength: 2048 }),
    };
    for (const [alg, pair] of Object.entries(pairs)) {
      const pem = pair.privateKey.export({ type: "pkcs8", format: "pem" }).toString();
      const token = await signIdentityToken({ id: "u" }, { privateKey: pem, kid: "isk_2" });
      const { header, signed, signature } = parts(token);
      expect(header.alg).toBe(alg);
      const key = pair.publicKey;
      const digest = alg === "EdDSA" ? null : "sha256";
      const ok = verify(
        digest,
        Buffer.from(signed),
        alg === "ES256" ? { key, dsaEncoding: "ieee-p1363" } : key,
        Buffer.from(signature, "base64url"),
      );
      expect(ok).toBe(true);
    }
  });

  it("maps a user to claims and refuses one dimmi cannot trust", () => {
    expect(
      toClaims({
        id: "u1",
        email: "a@b.test",
        emailVerified: true,
        name: "",
        signedUpAt: new Date("2026-01-01T00:00:00Z"),
        company: { id: 7, name: "Acme", attributes: { seats: 5 } },
        attributes: { plan_name: "pro" },
      }),
    ).toEqual({
      sub: "u1",
      email: "a@b.test",
      email_verified: true,
      custom_attributes: { plan_name: "pro", signed_up_at: 1767225600 },
      company: { id: "7", name: "Acme", custom_attributes: { seats: 5 } },
    });
    expect(() => toClaims({ id: "" })).toThrow(DimmiIdentityError);
    expect(() => toClaims({ id: "u", attributes: { "Plan Name": "x" } })).toThrow(/attribute_key_invalid/);
    expect(() => toClaims({ id: "u", plan: "pro" } as never)).toThrow(/user_field_unknown/);
  });

  it("refuses a lifetime over an hour and a missing secret", async () => {
    await expect(signIdentityToken({ id: "u" }, { secret, expiresIn: 7200 })).rejects.toThrow(/expires_in_invalid/);
    await expect(signIdentityToken({ id: "u" })).rejects.toThrow(/secret_missing/);
  });
});

describe("identity routes", () => {
  it("answers a token for a signed-in user and null otherwise, never cached", async () => {
    const handler = createIdentityHandler((request) => (request.headers.get("cookie") ? { id: "u1" } : null), {
      secret,
    });
    const signedIn = await handler(new Request("https://shop.test/api/dimmi/token", { headers: { cookie: "s=1" } }));
    expect(signedIn.status).toBe(200);
    expect(signedIn.headers.get("Cache-Control")).toBe("no-store, private");
    expect(parts((await signedIn.json()).token).claims.sub).toBe("u1");
    const signedOut = await handler(new Request("https://shop.test/api/dimmi/token"));
    expect(await signedOut.json()).toEqual({ token: null });
  });

  it("serves the same answer from Express and Hono", async () => {
    const sent: { headers?: Record<string, string>; status?: number; body?: string } = {};
    const res = {
      set(headers: Record<string, string>) {
        sent.headers = headers;
        return res;
      },
      status(code: number) {
        sent.status = code;
        return res;
      },
      send(body: string) {
        sent.body = body;
      },
    };
    await expressIdentity(() => ({ id: "u1" }), { secret })({}, res, () => undefined);
    expect(sent.status).toBe(200);
    expect(JSON.parse(sent.body ?? "{}").token).toMatch(/^ey/);
    const hono = await honoIdentity(() => null, { secret })({});
    expect(await hono.json()).toEqual({ token: null });
  });
});
