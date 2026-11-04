import { describe, expect, it } from "vitest";

import { sidebarSections } from "./use-sidebar-views";

describe("sidebarSections", () => {
  it("lists teams, active teammates and, from two on, inboxes with their open counts", () => {
    const sections = sidebarSections({
      teams: [{ id: "team_a", name: "Billing" }],
      members: [
        { id: "mem_a", status: "active", user: { name: "Ada" } },
        { id: "mem_b", status: "deactivated", user: { name: "Bob" } },
      ],
      inboxes: [
        { id: "inbox_a", name: "Support" },
        { id: "inbox_b", name: "Sales" },
      ],
      counts: { teams: { team_a: { open: 3 } }, members: {}, inboxes: { inbox_b: { open: 1 } } },
    });

    expect(sections.map((s) => s.label)).toEqual(["Views", "Teams", "Teammates", "Inboxes"]);
    expect(sections[1].items[0]).toMatchObject({ label: "Billing", href: "/d/inbox/team_a", badge: 3 });
    expect(sections[2].items.map((i) => i.label)).toEqual(["Ada"]);
    expect(sections[2].items[0].badge).toBeUndefined();
    expect(sections[3].items[1].badge).toBe(1);
  });

  it("leaves out a single inbox and empty sections, keeping views for their new view action", () => {
    const sections = sidebarSections({ inboxes: [{ id: "inbox_a", name: "Support" }] });
    expect(sections.map((s) => s.label)).toEqual(["Views"]);
  });

  it("lists saved views with the counts of pinned ones", () => {
    const onNewView = () => undefined;
    const [views] = sidebarSections({
      views: [{ id: "cvw_a", name: "Urgent" }],
      counts: { teams: {}, members: {}, inboxes: {}, custom: { cvw_a: { open: 2 } } },
      onNewView,
    });
    expect(views.items[0]).toMatchObject({ label: "Urgent", href: "/d/inbox/cvw_a", badge: 2 });
    expect(views.action?.label).toBe("New view");
  });
});
