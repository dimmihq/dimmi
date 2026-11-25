import { describe, expect, it } from "vitest";

import { grouped } from "./looks";

const of = (...keys: string[]) => keys.map((key) => ({ key }));

describe("grouped", () => {
  it("sorts templates into their categories, in the gallery's order", () => {
    const groups = grouped(
      of("tpl_custom", "tpl_qa_procedure_digest", "tpl_github_pr_docs_sync", "tpl_knowledge_gap_finder"),
    );

    expect(groups.map((g) => [g.title, g.templates.map((t) => t.key)])).toEqual([
      ["Keep knowledge up to date", ["tpl_knowledge_gap_finder", "tpl_github_pr_docs_sync"]],
      ["Improve how the team works", ["tpl_qa_procedure_digest"]],
      ["Your own", ["tpl_custom"]],
    ]);
  });

  it("leaves out a category with nothing in it", () => {
    expect(grouped(of("tpl_custom")).map((g) => g.title)).toEqual(["Your own"]);
    expect(grouped([])).toEqual([]);
  });

  it("puts a template it does not know last, under More", () => {
    const groups = grouped(of("tpl_new_one", "tpl_custom"));

    expect(groups.map((g) => g.title)).toEqual(["Your own", "More"]);
    expect(groups[1]?.templates).toEqual([{ key: "tpl_new_one" }]);
  });
});
