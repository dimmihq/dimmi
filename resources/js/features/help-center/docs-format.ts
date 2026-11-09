/*
 * How an article reads in the docs list and editor: where it stands and
 * the line of detail under its title. Kept apart from the pages so the
 * wording can be tested without rendering them.
 */
import type { Article, Collection } from "@/lib/api/help-centers";
import { formatDate } from "@/lib/i18n/format";

import { treeOptions } from "./collection-tree";

/*
 * articleState names where an article stands for its chip.
 */
export function articleState(article: Pick<Article, "status" | "has_unpublished_changes">): string {
  if (article.status === "archived") {
    return "Archived";
  }
  if (article.status === "draft") {
    return "Draft";
  }

  return article.has_unpublished_changes ? "Unpublished changes" : "Published";
}

/*
 * articleDetail is the line under an article's title: its collection,
 * who may read it and when it last changed.
 */
export function articleDetail(article: Article, collections: Collection[]): string {
  const collection = collections.find((c) => c.id === article.collection_id)?.name ?? "No collection";
  const parts = [collection];
  if (article.visibility === "agent_only") {
    parts.push("Agent only");
  }
  parts.push(`Updated ${formatDate(article.updated_at)}`);

  return parts.join(" · ");
}

/*
 * collectionOptions are the choices of an article's collection in tree
 * order, each labelled with its whole path and placed at its depth.
 */
export function collectionOptions(collections: Collection[]): { id: string; label: string; depth: number }[] {
  return [
    { id: "", label: "No collection", depth: 0 },
    ...treeOptions(collections).map((o) => ({ id: o.id, label: o.path, depth: o.depth })),
  ];
}

/*
 * Draft is the article as the writer edits it, before it is saved.
 */
export interface Draft {
  title: string;
  slug: string;
  description: string;
  body: string;
  collection: string;
  visibility: "public" | "agent_only";
}
