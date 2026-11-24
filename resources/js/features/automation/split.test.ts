import { describe, expect, it } from "vitest";

import { freshStep } from "./edit-steps";
import {
  evenShares,
  sharesTotal,
  splitSentence,
  variantsOf,
  winnerOf,
  withNewVariant,
  withoutVariant,
  withVariantLabel,
  withVariantWeight,
  withWinner,
} from "./split";

describe("a/b splits", () => {
  it("starts with variants A and B, half each, one path per variant", () => {
    const step = freshStep("split");
    const variants = variantsOf(step);

    expect(variants.map((v) => [v.label, v.weight])).toEqual([
      ["A", 50],
      ["B", 50],
    ]);
    expect(step.paths?.map((p) => [p.id, p.label])).toEqual(variants.map((v) => [v.id, v.label]));
    expect(splitSentence(step)).toBe("Split A 50% · B 50%");
  });

  it("shares evenly again when a variant comes or goes", () => {
    const three = withNewVariant(freshStep("split"));

    expect(variantsOf(three).map((v) => v.weight)).toEqual([34, 33, 33]);
    expect(three.paths).toHaveLength(3);
    expect(sharesTotal(three)).toBe(100);

    const gone = variantsOf(three)[2]?.id ?? "";
    const two = withoutVariant(three, gone);
    expect(variantsOf(two).map((v) => v.weight)).toEqual([50, 50]);
    expect(two.paths?.map((p) => p.id)).not.toContain(gone);
    expect(evenShares(4)).toEqual([25, 25, 25, 25]);
  });

  it("renames the variant and its path together, and keeps shares in range", () => {
    const step = freshStep("split");
    const id = variantsOf(step)[0]?.id ?? "";
    const renamed = withVariantWeight(withVariantLabel(step, id, "Short reply"), id, 140);

    expect(variantsOf(renamed)[0]).toEqual({ id, label: "Short reply", weight: 100 });
    expect(renamed.paths?.[0]?.label).toBe("Short reply");
    expect(sharesTotal(renamed)).toBe(150);
  });

  it("sends everyone down a winner and forgets it when the variant goes", () => {
    const step = withNewVariant(freshStep("split"));
    const id = variantsOf(step)[1]?.id ?? "";
    const won = withWinner(step, id);

    expect(winnerOf(won)).toBe(id);
    expect(splitSentence(won)).toBe("Send everyone down B");
    expect(winnerOf(withWinner(won, ""))).toBe("");
    expect(won.settings).toHaveProperty("winner");
    expect(withWinner(won, "").settings).not.toHaveProperty("winner");
    expect(winnerOf(withoutVariant(won, id))).toBe("");
  });
});
