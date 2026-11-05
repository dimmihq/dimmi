import { describe, expect, it } from "vitest";

import { accessChanges, levelLabel, levelSentence, levels, needsTeams } from "./member-access";

describe("member access", () => {
  it("names and explains every level", () => {
    for (const level of levels) {
      expect(levelLabel(level)).not.toBe("");
      expect(levelSentence(level)).toMatch(/^They see/);
    }
  });

  it("knows which levels name teams", () => {
    expect(levels.filter(needsTeams)).toEqual(["teams", "all_except_teams"]);
  });

  it("builds the change, waiting for teams where a level needs them", () => {
    expect(accessChanges({ level: "teams", unassigned: true, teams: [] })).toBeNull();
    expect(accessChanges({ level: "teams", unassigned: true, teams: ["team_1"] })).toEqual({
      conversation_access: "teams",
      conversation_access_unassigned: true,
      access_team_ids: ["team_1"],
    });
    expect(accessChanges({ level: "all", unassigned: true, teams: ["team_1"] })).toEqual({
      conversation_access: "all",
      conversation_access_unassigned: false,
      access_team_ids: [],
    });
    expect(accessChanges({ level: "assigned", unassigned: true, teams: ["team_1"] })).toEqual({
      conversation_access: "assigned",
      conversation_access_unassigned: true,
      access_team_ids: [],
    });
  });
});
