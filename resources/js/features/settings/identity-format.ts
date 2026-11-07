/*
 * How an identity secret reads on the identity page, and the code a
 * developer pastes to sign tokens with it. Kept apart from the page so
 * the wording and the snippets can be tested without rendering them.
 */
import type { IdentitySecret } from "@/lib/api/identity";
import { formatDate } from "@/lib/i18n/format";

/*
 * secretDetail is the line under a secret's name: its algorithm, how to
 * tell it apart, and when it was last used.
 */
export function secretDetail(secret: IdentitySecret): string {
  const mark = secret.algorithm === "HS256" ? `…${secret.fingerprint}` : secret.fingerprint;
  const used = secret.last_used_at ? `Last used ${formatDate(secret.last_used_at)}` : "Never used";

  return [secret.algorithm, mark, used].join(" · ");
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

export type SnippetLanguage = "Node.js" | "Python" | "Script tag";

export const snippetLanguages: SnippetLanguage[] = ["Node.js", "Python", "Script tag"];

/*
 * signingSnippet is the code for one stack: an endpoint on the host app
 * that answers the signed-in user's token, or the snippet attribute that
 * points the messenger at it.
 */
export function signingSnippet(language: SnippetLanguage, kid: string): string {
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
