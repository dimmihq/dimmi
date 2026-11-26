import { describe, expect, it } from "vitest";

import cases from "../../../../../framework/markdown/testdata/embeds.json";
import { resolveEmbed } from "./embed-providers";

describe("embed providers", () => {
  it.each(Object.entries(cases.resolves))("plays %s from its player", (link, player) => {
    expect(resolveEmbed(link)?.player).toBe(player);
  });

  it.each(cases.rejects)("refuses %s", (link) => {
    expect(resolveEmbed(link)).toBeNull();
  });
});
