import { describe, expect, it } from "vitest";

import type { ChangelogEntry } from "@/lib/api/changelog";

import { authorNames, changeOf, draftOf, entryState, initialsOf } from "./entry-format";

function entry(over: Partial<ChangelogEntry> = {}): ChangelogEntry {
  return {
    id: "cle_1",
    object: "changelog_entry",
    help_center_id: "hc_1",
    title: "Shared inboxes",
    slug: "shared-inboxes",
    summary: null,
    body_markdown: "Hello",
    cover_url: null,
    status: "draft",
    scheduled_for: null,
    published_at: null,
    has_unpublished_changes: false,
    notify_subscribers: false,
    notified_at: null,
    authors_chosen: false,
    labels: [{ id: "cll_1", name: "New", slug: "new", color: "#22c55e" }],
    authors: [{ member_id: "mem_1", name: "Ana Lima", avatar_url: null }],
    editors: [{ member_id: "mem_1", name: "Ana Lima", avatar_url: null }],
    lock_version: 1,
    created_at: "2026-11-01T10:00:00Z",
    updated_at: "2026-11-01T10:00:00Z",
    ...over,
  };
}

describe("changelog entry format", () => {
  it("leaves authors empty while the editors are named", () => {
    expect(draftOf(entry()).authors).toEqual([]);
    expect(draftOf(entry({ authors_chosen: true })).authors).toEqual(["mem_1"]);
  });

  it("sends every field of the draft", () => {
    const change = changeOf({ ...draftOf(entry()), cover: "/help-images/a.png" }, entry());
    expect(change).toMatchObject({ title: "Shared inboxes", label_ids: ["cll_1"], cover_url: "/help-images/a.png" });
    expect(change.slug).toBeUndefined();
    expect(changeOf({ ...draftOf(entry()), slug: "inboxes" }, entry()).slug).toBe("inboxes");
  });

  it("says where an entry stands", () => {
    expect(entryState(entry())).toBe("Draft");
    expect(entryState(entry({ status: "scheduled" }))).toBe("Scheduled");
    expect(entryState(entry({ status: "published" }))).toBe("Published");
    expect(entryState(entry({ status: "published", has_unpublished_changes: true }))).toBe("Unpublished changes");
  });

  it("names a few authors in full and counts the rest", () => {
    const people = ["Ana Lima", "Bo Chen", "Cy Diaz", "Di Eze"].map((name, i) => ({
      member_id: `mem_${i}`,
      name,
      avatar_url: null,
    }));
    expect(authorNames(entry({ authors: people.slice(0, 2) }))).toBe("Ana Lima and Bo Chen");
    expect(authorNames(entry({ authors: people }))).toBe("Ana Lima, Bo Chen and 2 more");
    expect(initialsOf("ana maria lima")).toBe("AM");
  });
});
