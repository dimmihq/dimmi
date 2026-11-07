import type { Article, Collection } from "@/lib/api/help-centers";

import { articleDetail, articleState, collectionOptions } from "./docs-format";

const collection = (id: string, name: string, parent: string | null = null): Collection => ({
  id,
  object: "collection",
  help_center_id: "hc_1",
  parent_id: parent,
  name,
  slug: id,
  description: null,
  icon: null,
  position: 0,
});

const article: Article = {
  id: "art_1",
  object: "article",
  help_center_id: "hc_1",
  collection_id: "col_1",
  title: "Invite your team",
  slug: "invite-your-team",
  description: null,
  status: "published",
  visibility: "agent_only",
  has_unpublished_changes: false,
  lock_version: 1,
  word_count: 10,
  published_at: null,
  updated_at: "2026-11-07T10:00:00Z",
};

describe("docs", () => {
  it("names where an article stands", () => {
    expect(articleState(article)).toBe("Published");
    expect(articleState({ ...article, has_unpublished_changes: true })).toBe("Unpublished changes");
    expect(articleState({ ...article, status: "draft" })).toBe("Draft");
    expect(articleState({ ...article, status: "archived" })).toBe("Archived");
  });

  it("describes an article by collection and audience", () => {
    expect(articleDetail(article, [collection("col_1", "Getting started")])).toMatch(
      /^Getting started · Agent only · Updated /,
    );
  });

  it("lists sub-collections under their parent", () => {
    const options = collectionOptions([collection("a", "Billing"), collection("b", "Invoices", "a")]);
    expect(options.map((o) => o.label)).toEqual(["No collection", "Billing", "Billing / Invoices"]);
  });
});
