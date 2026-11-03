/*
 * The codes an authenticator app shows, computed the way every app does
 * (RFC 6238: HMAC-SHA-1 over the 30-second step, six digits), so a test
 * can play the person's phone from the key the setup dialog shows.
 */
import { createHmac } from "node:crypto";

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
const period = 30;

function decodeBase32(secret: string): Buffer {
  let bits = "";
  for (const char of secret.replace(/[\s=]/g, "").toUpperCase()) {
    const value = alphabet.indexOf(char);
    if (value < 0) {
      throw new Error(`not base32: ${char}`);
    }
    bits += value.toString(2).padStart(5, "0");
  }

  const bytes: number[] = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) {
    bytes.push(Number.parseInt(bits.slice(i, i + 8), 2));
  }

  return Buffer.from(bytes);
}

/*
 * currentStep is the 30-second step the clock is in.
 */
export function currentStep(now: number = Date.now()): number {
  return Math.floor(now / 1000 / period);
}

/*
 * totpCode returns the six-digit code of the secret for the step.
 */
export function totpCode(secret: string, step: number = currentStep()): string {
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(step));

  const digest = createHmac("sha1", decodeBase32(secret)).update(counter).digest();
  const offset = (digest.at(-1) ?? 0) & 0x0f;
  const binary = digest.readUInt32BE(offset) & 0x7fffffff;

  return String(binary % 1_000_000).padStart(6, "0");
}
