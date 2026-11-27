/*
 * Author directory for blog posts and signed-off content pages.
 * Used for visible bylines and JSON-LD @type: "Person" blocks.
 */

export type Author = {
  id: string;
  name: string;
  role: string;
  bio: string;
  url: string;
  sameAs: string[];
};

export const AUTHORS: Record<string, Author> = {
  "frantz-kati": {
    id: "frantz-kati",
    name: "Frantz Kati",
    role: "Founder, dimmi",
    bio: "Founder of dimmi. Builds open-source customer support software and audits helpdesk pricing and architecture first-hand.",
    url: "https://github.com/dimmihq/dimmi",
    sameAs: ["https://github.com/dimmihq/dimmi"],
  },
};

export function getAuthor(id: string): Author {
  const found = AUTHORS[id];
  if (found) {
    return found;
  }
  return AUTHORS["frantz-kati"];
}
