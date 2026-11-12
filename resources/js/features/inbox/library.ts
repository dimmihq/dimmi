/*
 * What a teammate can bring into a reply or note besides files: the
 * team's saved replies, the default help center's published articles, its feature requests and its
 * published changelog entries. Each drops a line naming it, with the link
 * a customer can open.
 */
import { useQuery } from "@tanstack/react-query";
import type { ChangelogEntry } from "@/lib/api/changelog";
import { entriesQuery } from "@/lib/api/changelog";
import type { FeedbackPost } from "@/lib/api/feedback";
import { postsQuery } from "@/lib/api/feedback";
import { type Article, articlesQuery, helpCentersQuery } from "@/lib/api/help-centers";
import { type SavedReply, savedRepliesQuery } from "@/lib/api/saved-replies";

export type SourceKind = "saved" | "article" | "request" | "changelog";

export interface LibraryItem {
  id: string;
  kind: SourceKind;
  title: string;
  detail: string;
  /* Where a customer opens it, when it has a public page. */
  url?: string;
  /* The saved reply it is, whose text and actions a pick takes. */
  reply?: SavedReply;
}

function matches(item: LibraryItem, needle: string): boolean {
  return item.title.toLocaleLowerCase().includes(needle) || item.detail.toLocaleLowerCase().includes(needle);
}

function joinURL(base: string, path: string): string {
  return `${base.replace(/\/+$/, "")}/${path.replace(/^\/+/, "")}`;
}

/*
 * articleItems are the published public articles whose title or
 * description holds the needle.
 */
export function articleItems(articles: Article[], base: string, needle: string): LibraryItem[] {
  return articles
    .filter((article) => article.status === "published" && article.visibility === "public")
    .map((article) => ({
      id: article.id,
      kind: "article" as const,
      title: article.title,
      detail: article.description ?? "Help center",
      url: base ? joinURL(base, `articles/${article.slug}`) : undefined,
    }))
    .filter((item) => matches(item, needle));
}

/*
 * requestItems are feature requests, found by the server.
 */
export function requestItems(posts: FeedbackPost[], base: string): LibraryItem[] {
  return posts.map((post) => ({
    id: post.id,
    kind: "request" as const,
    title: post.title,
    detail: `Feature request · ${post.vote_count} ${post.vote_count === 1 ? "vote" : "votes"}`,
    url: base ? joinURL(base, post.path) : undefined,
  }));
}

/*
 * changelogItems are published changelog entries, found by the server.
 */
export function changelogItems(entries: ChangelogEntry[], base: string): LibraryItem[] {
  return entries.map((entry) => ({
    id: entry.id,
    kind: "changelog" as const,
    title: entry.title,
    detail: entry.summary ?? "Changelog",
    url: base ? joinURL(base, `changelog/${entry.slug}`) : undefined,
  }));
}

/*
 * savedItems are the saved replies the server found, shared and the
 * teammate's own.
 */
export function savedItems(replies: SavedReply[]): LibraryItem[] {
  return replies.map((reply) => ({
    id: reply.id,
    kind: "saved" as const,
    title: reply.title,
    detail: reply.shortcut ? `#${reply.shortcut} · ${reply.body}` : reply.body,
    reply,
  }));
}

/*
 * useLibrary returns the items of a kind whose title or detail holds the
 * query, ignoring case, and whether they are still loading.
 */
export function useLibrary(workspace: string, kind: SourceKind, query: string) {
  const needle = query.trim().toLocaleLowerCase();
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((candidate) => candidate.is_default) ?? centers[0];
  const id = center?.id ?? "";
  const base = center?.url ?? "";
  const search = needle || undefined;

  const saved = useQuery({ ...savedRepliesQuery(workspace, needle), enabled: kind === "saved" && workspace !== "" });
  const articles = useQuery({ ...articlesQuery(workspace, id), enabled: kind === "article" && id !== "" });
  const posts = useQuery({
    ...postsQuery(workspace, id, { sort: "trending", page: 1, query: search }),
    enabled: kind === "request" && id !== "",
  });
  const entries = useQuery({
    ...entriesQuery(workspace, id, { status: "published", page: 1, query: search }),
    enabled: kind === "changelog" && id !== "",
  });

  if (kind === "saved") {
    return { items: savedItems(saved.data ?? []).slice(0, 50), loading: saved.isPending, missing: false };
  }

  const read = {
    article: () => ({ pending: articles.isPending, items: articleItems(articles.data ?? [], base, needle) }),
    request: () => ({ pending: posts.isPending, items: requestItems(posts.data?.data ?? [], base) }),
    changelog: () => ({ pending: entries.isPending, items: changelogItems(entries.data?.data ?? [], base) }),
  }[kind]();

  return { items: read.items.slice(0, 50), loading: read.pending && id !== "", missing: centers.length === 0 };
}
