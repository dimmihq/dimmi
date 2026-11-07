/*
 * How a user from your app becomes the claims of an identity token. You
 * hand over a plain object, never claims, and it is checked here: the id
 * is required, an email counts as verified only when you say so, and
 * attributes must be flat values under snake_case keys. Empty fields are
 * left out of the token rather than sent as null.
 */

export interface IdentityCompany {
  id: string | number;
  name?: string | null;
  domain?: string | null;
  attributes?: Record<string, AttributeValue>;
}

export type AttributeValue = string | number | boolean | null;

export interface IdentityUser {
  id: string | number;
  email?: string | null;
  emailVerified?: boolean;
  name?: string | null;
  phone?: string | null;
  phoneVerified?: boolean;
  avatarUrl?: string | null;
  locale?: string | null;
  timezone?: string | null;
  signedUpAt?: Date | number | string | null;
  company?: IdentityCompany | null;
  attributes?: Record<string, AttributeValue>;
}

/*
 * DimmiIdentityError is a user object that cannot become a token, with a
 * stable code and the fix in its message.
 */
export class DimmiIdentityError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(`[dimmi] ${code}: ${message}`);
    this.name = "DimmiIdentityError";
    this.code = code;
  }
}

const attributeKey = /^[a-z0-9_]{1,64}$/;
const known = new Set([
  "id",
  "email",
  "emailVerified",
  "name",
  "phone",
  "phoneVerified",
  "avatarUrl",
  "locale",
  "timezone",
  "signedUpAt",
  "company",
  "attributes",
]);

/*
 * toClaims turns a user into token claims, throwing DimmiIdentityError
 * for a user that names nobody or carries attributes dimmi refuses.
 */
export function toClaims(user: IdentityUser): Record<string, unknown> {
  const sub = typeof user.id === "number" ? String(user.id) : user.id;
  if (typeof sub !== "string" || sub.trim() === "" || sub.length > 255) {
    throw new DimmiIdentityError("identify_user_invalid", "the user needs an id: a string or number, 1 to 255 long.");
  }
  for (const field of Object.keys(user)) {
    if (!known.has(field)) {
      throw new DimmiIdentityError("user_field_unknown", `put ${field} under attributes.`);
    }
  }
  const claims: Record<string, unknown> = { sub };
  set(claims, "email", user.email);
  if (user.email && user.emailVerified === true) {
    claims.email_verified = true;
  }
  set(claims, "name", user.name);
  set(claims, "phone", user.phone);
  if (user.phone && user.phoneVerified === true) {
    claims.phone_verified = true;
  }
  set(claims, "avatar_url", user.avatarUrl);
  set(claims, "locale", user.locale);
  set(claims, "timezone", user.timezone);
  const attributes = { ...checkAttributes(user.attributes ?? {}) };
  const signedUp = epochSeconds(user.signedUpAt);
  if (signedUp !== undefined) {
    attributes.signed_up_at = signedUp;
  }
  if (Object.keys(attributes).length > 0) {
    claims.custom_attributes = attributes;
  }
  if (user.company) {
    claims.company = companyClaims(user.company);
  }
  return claims;
}

function set(claims: Record<string, unknown>, name: string, value: string | null | undefined) {
  if (typeof value === "string" && value !== "") {
    claims[name] = value;
  }
}

function companyClaims(company: IdentityCompany): Record<string, unknown> {
  const id = typeof company.id === "number" ? String(company.id) : company.id;
  if (typeof id !== "string" || id === "") {
    throw new DimmiIdentityError("identify_user_invalid", "company needs an id.");
  }
  const out: Record<string, unknown> = { id };
  set(out, "name", company.name);
  set(out, "domain", company.domain);
  if (company.attributes && Object.keys(company.attributes).length > 0) {
    out.custom_attributes = checkAttributes(company.attributes);
  }
  return out;
}

function checkAttributes(attributes: Record<string, AttributeValue>): Record<string, AttributeValue> {
  const keys = Object.keys(attributes);
  if (keys.length > 100) {
    throw new DimmiIdentityError("attribute_limit", "send at most 100 attributes; update others through the API.");
  }
  for (const key of keys) {
    if (!attributeKey.test(key)) {
      throw new DimmiIdentityError("attribute_key_invalid", `use snake_case keys such as plan_name, not ${key}.`);
    }
    const value = attributes[key];
    if (value !== null && typeof value === "object") {
      throw new DimmiIdentityError("attribute_value_invalid", `${key} must be a string, number, boolean or null.`);
    }
  }
  return attributes;
}

function epochSeconds(value: Date | number | string | null | undefined): number | undefined {
  if (value === null || value === undefined || value === "") {
    return undefined;
  }
  if (value instanceof Date) {
    return Math.floor(value.getTime() / 1000);
  }
  if (typeof value === "number") {
    return value > 1e11 ? Math.floor(value / 1000) : Math.floor(value);
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? undefined : Math.floor(parsed / 1000);
}
