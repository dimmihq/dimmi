/*
 * How a personal access token reads on the tokens page: what it may do
 * in a few words, when it expires and was last used, and the expiry
 * choices offered when minting one. Kept apart from the page so the
 * wording can be tested without rendering it.
 */
import { allAbilities, type PersonalAccessToken } from "@/lib/api/tokens";

/*
 * expiryChoices are the lifetimes offered for a new token, in days, with
 * null for a token that never expires.
 */
export const expiryChoices: { label: string; days: number | null }[] = [
  { label: "30 days", days: 30 },
  { label: "90 days", days: 90 },
  { label: "1 year", days: 365 },
  { label: "Never", days: null },
];

const day = 24 * 60 * 60 * 1000;

/*
 * expiryFor returns when a token made now with the chosen lifetime
 * expires, or null when it never does.
 */
export function expiryFor(label: string, now: Date): string | null {
  const days = expiryChoices.find((choice) => choice.label === label)?.days ?? null;

  return days === null ? null : new Date(now.getTime() + days * day).toISOString();
}

/*
 * abilityLabel names an ability for people: conversations:read reads
 * "Conversations: read", ai:keys "AI: keys".
 */
export function abilityLabel(ability: string): string {
  if (ability === allAbilities) {
    return "Everything your role allows";
  }

  const [resource = "", action = ""] = ability.split(":");
  const name = resource === "ai" ? "AI" : resource.charAt(0).toUpperCase() + resource.slice(1);

  return action ? `${name}: ${action}` : name;
}

/*
 * abilitySummary lists what a token may do in a few words, naming the
 * first abilities and counting the rest.
 */
export function abilitySummary(abilities: string[]): string {
  if (abilities.includes(allAbilities)) {
    return abilityLabel(allAbilities);
  }

  const named = abilities.slice(0, 2).map(abilityLabel).join(", ");
  const more = abilities.length - 2;

  return more > 0 ? `${named} and ${more} more` : named;
}

const dates = new Intl.DateTimeFormat("en", { dateStyle: "medium" });

/*
 * tokenDetail is the line under a token's name: its prefix, what it may
 * do, when it was last used and when it expires or ended.
 */
export function tokenDetail(token: PersonalAccessToken, now: Date): string {
  const used = token.last_used_at ? `Last used ${dates.format(new Date(token.last_used_at))}` : "Never used";

  return [`${token.prefix}…`, abilitySummary(token.abilities), used, ending(token, now)].join(" · ");
}

/*
 * ending says when the token stops working, or that it already did.
 */
function ending(token: PersonalAccessToken, now: Date): string {
  if (token.revoked_at) {
    return `Revoked ${dates.format(new Date(token.revoked_at))}`;
  }

  if (!token.expires_at) {
    return "Never expires";
  }

  const expires = new Date(token.expires_at);

  return expires.getTime() <= now.getTime() ? `Expired ${dates.format(expires)}` : `Expires ${dates.format(expires)}`;
}

/*
 * isLive reports whether the token still works.
 */
export function isLive(token: PersonalAccessToken, now: Date): boolean {
  return !token.revoked_at && (!token.expires_at || new Date(token.expires_at).getTime() > now.getTime());
}
