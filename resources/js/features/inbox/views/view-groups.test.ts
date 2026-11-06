import { describe, expect, it } from "vitest";

import { viewGroups } from "./view-groups";

const counts = {
  mine: { open: 3, unread: 0, snoozed: 1 },
  unassigned: { open: 4, unread: 0 },
  all: { open: 12, capped: false },
  mentions: { unread: 2 },
  inboxes: { inbox_b: { open: 1 } },
  teams: { team_a: { open: 3 } },
  members: {},
  custom: { cvw_a: { open: 2 } },
};

describe("viewGroups", () => {
  it("lists the fixed views, then views, teams, active teammates and, from two on, inboxes, with spam last", () => {
    const groups = viewGroups({
      teams: [{ id: "team_a", name: "Billing" }],
      members: [
        { id: "mem_a", status: "active", user: { name: "Ada" } },
        { id: "mem_b", status: "deactivated", user: { name: "Bob" } },
      ],
      inboxes: [
        { id: "inbox_a", name: "Support" },
        { id: "inbox_b", name: "Sales" },
      ],
      views: [{ id: "cvw_a", name: "Urgent" }],
      counts,
    });

    expect(groups.map((g) => g.label)).toEqual(["Inbox", "Views", "Teams", "Teammates", "Inboxes", "More"]);
    expect(groups[0].rows.map((r) => [r.id, r.count])).toEqual([
      ["mine", 3],
      ["unassigned", 4],
      ["all", 12],
      ["mentions", 2],
      ["snoozed", 1],
    ]);
    expect(groups[1].rows[0]).toMatchObject({ id: "cvw_a", label: "Urgent", count: 2 });
    expect(groups[2].rows[0]).toMatchObject({ id: "team_a", label: "Billing", count: 3 });
    expect(groups[3].rows.map((r) => r.label)).toEqual(["Ada"]);
    expect(groups[3].rows[0].count).toBeUndefined();
    expect(groups[4].rows[1].count).toBe(1);
    expect(groups[5].rows.map((r) => r.id)).toEqual(["spam"]);
  });

  it("leaves out a single inbox and empty groups", () => {
    const groups = viewGroups({ inboxes: [{ id: "inbox_a", name: "Support" }] });
    expect(groups.map((g) => g.label)).toEqual(["Inbox", "More"]);
  });

  it("hides unassigned from those who cannot see it and shows the trash to those who may delete", () => {
    const groups = viewGroups({ unassigned: false, canDelete: true });
    expect(groups[0].rows.map((r) => r.id)).not.toContain("unassigned");
    expect(groups[1].rows.map((r) => r.id)).toEqual(["spam", "deleted"]);
  });
});
