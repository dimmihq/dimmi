import { describe, expect, it } from "vitest";

import { toMentionables } from "./mentionable";

describe("toMentionables", () => {
  it("lists teammates before teams and warns about one who cannot see the conversation", () => {
    const found = toMentionables({
      members: [
        { id: "mem_a", name: "Ada", avatar_url: null, availability: "online", can_see: true },
        { id: "mem_b", name: "Bob", avatar_url: null, availability: "away", can_see: false },
      ],
      teams: [
        { id: "team_x", name: "Billing", member_count: 3 },
        { id: "team_y", name: "Solo", member_count: 1 },
      ],
    });

    expect(found.map((m) => [m.id, m.detail, m.warning])).toEqual([
      ["mem_a", "", null],
      ["mem_b", "Away", "Can't see this conversation"],
      ["team_x", "3 members", null],
      ["team_y", "1 member", null],
    ]);
  });
});
