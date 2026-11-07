/*
 * Signing an identity token with nothing but the platform's Web Crypto,
 * so the same code runs on Node, Bun, Deno, Cloudflare Workers and the
 * Vercel edge without a dependency. A shared secret signs HS256 with the
 * whole dmi_idsec_ string as the key, never decoded. A private key in PEM
 * signs RS256, ES256 or EdDSA, whichever it is.
 */

export type Algorithm = "HS256" | "RS256" | "ES256" | "EdDSA";

export interface SigningKey {
  algorithm: Algorithm;
  key: CryptoKey;
  kid?: string;
}

const encoder = new TextEncoder();

/*
 * secretKey imports a shared secret for HS256.
 */
export async function secretKey(secret: string, kid?: string): Promise<SigningKey> {
  const key = await crypto.subtle.importKey("raw", encoder.encode(secret), { name: "HMAC", hash: "SHA-256" }, false, [
    "sign",
  ]);
  return { algorithm: "HS256", key, kid };
}

/*
 * privateKey imports a PKCS #8 PEM private key, trying Ed25519, P-256
 * and RSA in turn, or only the algorithm named.
 */
export async function privateKey(pem: string, kid: string, algorithm?: Algorithm): Promise<SigningKey> {
  const der = pemBytes(pem);
  const candidates: [Algorithm, AlgorithmIdentifier | EcKeyImportParams | RsaHashedImportParams][] = [
    ["EdDSA", { name: "Ed25519" }],
    ["ES256", { name: "ECDSA", namedCurve: "P-256" }],
    ["RS256", { name: "RSASSA-PKCS1-v1_5", hash: "SHA-256" }],
  ];
  for (const [alg, params] of candidates) {
    if (algorithm && algorithm !== alg) {
      continue;
    }
    const key = await crypto.subtle.importKey("pkcs8", der, params, false, ["sign"]).catch(() => undefined);
    if (key) {
      return { algorithm: alg, key, kid };
    }
  }
  throw new Error("[dimmi] private_key_invalid: the private key must be PKCS #8 PEM: Ed25519, P-256 or RSA.");
}

/*
 * sign makes a compact JWT of the claims with the key.
 */
export async function sign(claims: Record<string, unknown>, signing: SigningKey): Promise<string> {
  const header: Record<string, string> = { alg: signing.algorithm, typ: "JWT" };
  if (signing.kid) {
    header.kid = signing.kid;
  }
  const input = `${base64url(encoder.encode(JSON.stringify(header)))}.${base64url(encoder.encode(JSON.stringify(claims)))}`;
  const signature = await crypto.subtle.sign(params(signing.algorithm), signing.key, encoder.encode(input));
  return `${input}.${base64url(new Uint8Array(signature))}`;
}

function params(algorithm: Algorithm): AlgorithmIdentifier | EcdsaParams {
  switch (algorithm) {
    case "HS256":
      return { name: "HMAC" };
    case "ES256":
      return { name: "ECDSA", hash: "SHA-256" };
    case "EdDSA":
      return { name: "Ed25519" };
    default:
      return { name: "RSASSA-PKCS1-v1_5" };
  }
}

function pemBytes(pem: string): ArrayBuffer {
  if (/BEGIN (RSA |EC )?PUBLIC KEY/.test(pem)) {
    throw new Error("[dimmi] private_key_invalid: that is a public key; sign with the private key.");
  }
  const body = pem.replace(/-----[^-]+-----/g, "").replace(/\s+/g, "");
  const binary = atob(body);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    out[i] = binary.charCodeAt(i);
  }
  return out.buffer;
}

function base64url(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}
