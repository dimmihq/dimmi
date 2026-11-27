/*
 * Authors and reviewers for marketing comparison pages, guides, and blog posts.
 * Update bio and social links once confirmed.
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
    role: "Founder at dimmi",
    bio: "Building dimmi, the open-source customer support platform.",
    url: "https://github.com/bahdcoder",
    sameAs: ["https://github.com/bahdcoder", "https://github.com/dimmihq/dimmi"],
  },
};

export function getAuthor(id: string): Author {
  return (
    AUTHORS[id] ?? {
      id,
      name: id,
      role: "dimmi team",
      bio: "Customer support engineering team at dimmi.",
      url: "https://trydimmi.com",
      sameAs: ["https://github.com/dimmihq/dimmi"],
    }
  );
}
