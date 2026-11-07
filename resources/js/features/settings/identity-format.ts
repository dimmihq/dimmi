/*
 * How an identity secret reads on the identity page, and the code a
 * developer pastes to sign tokens with it. Kept apart from the page so
 * the wording and the snippets can be tested without rendering them.
 */
import type { IdentityMode, IdentitySecret, IdentityStats } from "@/lib/api/identity";
import { formatDate } from "@/lib/i18n/format";

/*
 * secretDetail is the line under a secret's name: its algorithm, how to
 * tell it apart, and when it was last used.
 */
export function secretDetail(secret: IdentitySecret): string {
  const used = secret.last_used_at ? `Last used ${formatDate(secret.last_used_at)}` : "Never used";
  if (secret.source === "jwks") {
    return [`${secret.algorithm} key set`, hostOf(secret.jwks_url), used].join(" · ");
  }
  const mark = secret.algorithm === "HS256" ? `…${secret.fingerprint}` : secret.fingerprint;

  return [secret.algorithm, mark, used].join(" · ");
}

function hostOf(address: string | null): string {
  try {
    return address ? new URL(address).host : "";
  } catch {
    return address ?? "";
  }
}

/*
 * modeLabels names each sign-in mode as the page offers it.
 */
export const modeLabels: Record<IdentityMode, string> = {
  off: "Off",
  optional: "Optional",
  required: "Required",
};

/*
 * modeDetail says what the chosen mode does for visitors.
 */
export function modeDetail(mode: IdentityMode): string {
  switch (mode) {
    case "off":
      return "Identity tokens are ignored and every visitor chats anonymously.";
    case "required":
      return "Only signed-in customers see the messenger.";
    default:
      return "Signed-in customers are verified; everyone else chats anonymously.";
  }
}

/*
 * statsSummary is the line about how messenger starts went lately, or
 * null when there were none.
 */
export function statsSummary(stats: IdentityStats): string | null {
  const total = stats.verified + stats.invalid + stats.anonymous;
  if (total === 0) {
    return null;
  }
  const share = Math.round(stats.verified_share * 100);

  return `${share}% of ${total.toLocaleString()} messenger starts were signed in over the last ${stats.days} days.`;
}

/*
 * topFailures are the refusal codes seen most, most first, at most three.
 */
export function topFailures(stats: IdentityStats): { code: string; count: number }[] {
  return Object.entries(stats.totals)
    .filter(([code]) => code !== "verified" && code !== "anonymous_started")
    .map(([code, count]) => ({ code, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 3);
}

/*
 * secretState is the chip a secret that no longer signs new sessions
 * shows, null for an active one.
 */
export function secretState(secret: IdentitySecret): string | null {
  if (secret.status === "revoked") {
    return "Revoked";
  }
  if (secret.status === "retiring" && secret.retires_at) {
    return `Works until ${formatDate(secret.retires_at)}`;
  }

  return null;
}

export type SnippetLanguage = "Next.js" | "Express" | "Node.js" | "Python" | "Script tag";

export const snippetLanguages: SnippetLanguage[] = ["Next.js", "Express", "Node.js", "Python", "Script tag"];

/*
 * signingSnippet is the code for one stack: an endpoint on the host app
 * that answers the signed-in user's token, or the snippet attribute that
 * points the messenger at it.
 */
export function signingSnippet(language: SnippetLanguage, kid: string): string {
  if (language === "Next.js") {
    return [
      "// dimmi.config.ts: npm install @dimmi/nextjs",
      'import { defineDimmi } from "@dimmi/nextjs/config";',
      'import { clerk } from "@dimmi/nextjs/clerk";',
      "",
      `export default defineDimmi({ identify: clerk(), kid: "${kid}" });`,
      "",
      "// app/dimmi-token.ts",
      '"use server";',
      'import config from "@/dimmi.config";',
      'import { identityToken } from "@dimmi/nextjs/server";',
      "",
      "export async function dimmiToken() {",
      "  return identityToken(config);",
      "}",
      "",
      '// app/layout.tsx: <Dimmi appId="wgt_…" host="…" identity={dimmiToken} />',
    ].join("\n");
  }
  if (language === "Express") {
    return [
      "// npm install @dimmi/server",
      'import { dimmiIdentity } from "@dimmi/server/express";',
      "",
      'app.get("/api/dimmi/token", dimmiIdentity((req) =>',
      "  req.user && { id: req.user.id, email: req.user.email, emailVerified: req.user.emailVerified },",
      `, { kid: "${kid}" }));`,
    ].join("\n");
  }
  if (language === "Script tag") {
    return '<script src="…/widget/v1/loader.js" data-app-id="wgt_…"\n  data-identity-url="/api/dimmi/token" async></script>';
  }
  if (language === "Python") {
    return [
      "# GET /api/dimmi/token, behind your sign-in",
      "import os, time, jwt",
      "",
      "def dimmi_token(user):",
      "    if user is None:",
      '        return {"token": None}',
      "    now = int(time.time())",
      "    claims = {",
      '        "sub": str(user.id), "email": user.email, "email_verified": user.email_verified,',
      '        "name": user.name, "iat": now, "exp": now + 600,',
      "    }",
      '    token = jwt.encode(claims, os.environ["DIMMI_IDENTITY_SECRET"], algorithm="HS256",',
      `        headers={"kid": "${kid}"})`,
      '    return {"token": token}',
    ].join("\n");
  }

  return [
    "// GET /api/dimmi/token, behind your sign-in",
    'import { SignJWT } from "jose";',
    "",
    "const secret = new TextEncoder().encode(process.env.DIMMI_IDENTITY_SECRET);",
    "",
    "export async function dimmiToken(user) {",
    "  if (!user) return { token: null };",
    "  const token = await new SignJWT({ email: user.email, email_verified: user.emailVerified, name: user.name })",
    `    .setProtectedHeader({ alg: "HS256", kid: "${kid}" })`,
    "    .setSubject(String(user.id))",
    "    .setIssuedAt()",
    '    .setExpirationTime("10m")',
    "    .sign(secret);",
    "  return { token };",
    "}",
  ].join("\n");
}
