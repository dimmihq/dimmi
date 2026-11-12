/*
 * Conditions applied to the list without saving a view travel in the
 * address as base64url JSON, the way the list's filter[where] reads
 * them, so an applied filter survives a reload and can be shared as a
 * link. Anything the address holds that is not a definition is ignored.
 */
import type { ViewDefinition } from "@/lib/api/views";

export function encodeWhere(definition: ViewDefinition): string {
  const bytes = new TextEncoder().encode(JSON.stringify(definition));
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }

  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

export function decodeWhere(where: string | undefined): ViewDefinition | null {
  if (!where) {
    return null;
  }
  try {
    const binary = atob(where.replaceAll("-", "+").replaceAll("_", "/"));
    const parsed: unknown = JSON.parse(new TextDecoder().decode(Uint8Array.from(binary, (c) => c.charCodeAt(0))));
    const groups = (parsed as Partial<ViewDefinition> | null)?.groups;

    return Array.isArray(groups) ? (parsed as ViewDefinition) : null;
  } catch {
    return null;
  }
}

/*
 * hasConditions says whether a definition narrows anything at all.
 */
export function hasConditions(definition: ViewDefinition): boolean {
  return definition.groups.some((group) => group.conditions.length > 0);
}
