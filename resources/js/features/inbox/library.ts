/*
 * What a teammate can bring into a reply or note besides files: help
 * center articles, saved replies, past conversations, issues, feature
 * requests and changelog entries. Until those parts of the workspace
 * exist, this is the example workspace's library, so the composer's add
 * menu can be tried end to end. Articles, conversations, issues, requests
 * and changelog entries attach as tiles; a saved reply drops its text
 * into the draft.
 */
export type SourceKind = "article" | "reply" | "conversation" | "issue" | "request" | "changelog";

export interface LibraryItem {
  id: string;
  kind: SourceKind;
  title: string;
  detail: string;
  /* The text a saved reply puts in the draft. */
  body?: string;
}

const items: LibraryItem[] = [
  { id: "a1", kind: "article", title: "Reset your password", detail: "Help center · Account" },
  { id: "a2", kind: "article", title: "Sign in with a one-time code", detail: "Help center · Account" },
  { id: "a3", kind: "article", title: "Why reset links expire", detail: "Help center · Security" },
  { id: "a4", kind: "article", title: "Use dimmi with a password manager", detail: "Help center · Account" },
  {
    id: "r1",
    kind: "reply",
    title: "Send a sign-in code",
    detail: "Saved reply",
    body: "I've sent a one-time sign-in code to your email. Enter it on the sign-in page and you'll be in right away.",
  },
  {
    id: "r2",
    kind: "reply",
    title: "Ask for the mail app",
    detail: "Saved reply",
    body: "Could you tell me which email app you open the link in? Some apps open links in a preview first, which can use them up.",
  },
  {
    id: "r3",
    kind: "reply",
    title: "Closing the loop",
    detail: "Saved reply",
    body: "Glad that's sorted! I'll close this conversation, but reply any time and it reopens.",
  },
  { id: "c1", kind: "conversation", title: "Reset link expired on open", detail: "Noor Haddad · 3 days ago" },
  { id: "c2", kind: "conversation", title: "Outlook marks reset links as used", detail: "Tom Becker · Last week" },
  { id: "i1", kind: "issue", title: "Mail previews spend single-use links", detail: "GitHub · #412 · Open" },
  { id: "i2", kind: "issue", title: "Offer sign-in codes beside reset links", detail: "GitHub · #398 · In progress" },
  { id: "f1", kind: "request", title: "Passwordless sign-in", detail: "Feature request · 24 votes" },
  { id: "l1", kind: "changelog", title: "Sign-in codes are here", detail: "Changelog · October" },
];

/*
 * search returns the library items of a kind whose title or detail holds
 * the query, ignoring case.
 */
export function search(kind: SourceKind, query: string): LibraryItem[] {
  const needle = query.trim().toLocaleLowerCase();

  return items.filter(
    (item) =>
      item.kind === kind &&
      (item.title.toLocaleLowerCase().includes(needle) || item.detail.toLocaleLowerCase().includes(needle)),
  );
}
