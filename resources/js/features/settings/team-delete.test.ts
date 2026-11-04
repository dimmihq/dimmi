import { countLine, target } from "./team-delete";

describe("team delete", () => {
  it("says what the team holds", () => {
    expect(countLine({ open: 2, snoozed: 1, closed: 4 })).toBe(
      "It holds 2 open, 1 snoozed and 4 closed conversations.",
    );
    expect(countLine({ open: 0, snoozed: 0, closed: 0 })).toBe("It holds no conversations.");
  });

  it("goes nowhere until a needed target is picked", () => {
    expect(target(null, null)).toBeUndefined();
    expect(target("team", null)).toBeUndefined();
    expect(target("team", "team_1")).toEqual({ team: "team_1" });
    expect(target("member", "mem_1")).toEqual({ member: "mem_1" });
    expect(target("none", null)).toBeNull();
  });
});
