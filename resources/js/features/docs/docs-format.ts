/*
 * How an article reads in the docs list and editor: where it stands and
 * the line of detail under its title. Kept apart from the pages so the
 * wording can be tested without rendering them.
 */
import type { Article, Collection } from "@/lib/api/help-centers";
import { formatDate } from "@/lib/i18n/format";

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
 * collectionOptions are the choices of an article's collection, with
 * sub-collections under their parent.
 */
export function collectionOptions(collections: Collection[]): { id: string; label: string }[] {
  const out = [{ id: "", label: "No collection" }];
  for (const top of collections.filter((c) => c.parent_id === null)) {
    out.push({ id: top.id, label: top.name });
    for (const child of collections.filter((c) => c.parent_id === top.id)) {
      out.push({ id: child.id, label: `${top.name} / ${child.name}` });
    }
  }

  return out;
}
