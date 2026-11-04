import { describe, expect, it } from "vitest";

import { listStatus, viewParams } from "./view-route";

const id = "01jb8z2k3m4n5p6q7r8s9t0vwx";

describe("viewParams", () => {
  it("keeps the fixed views as they are", () => {
    for (const view of ["mine", "unassigned", "all", "mentions", "spam", "deleted"]) {
      expect(viewParams(view)).toEqual({ kind: "fixed", filter: view });
    }
  });

  it("lists the person's sleeping conversations under snoozed", () => {
    expect(viewParams("snoozed")).toEqual({ kind: "fixed", filter: "mine", status: "snoozed" });
  });

  it("names a team, a teammate and an inbox the way the API reads them", () => {
    expect(viewParams(`team_${id}`)).toEqual({ kind: "team", filter: `team:team_${id}` });
    expect(viewParams(`mem_${id}`)).toEqual({ kind: "member", filter: `member:mem_${id}` });
    expect(viewParams(`inbox_${id}`)).toEqual({ kind: "inbox", filter: `inbox:inbox_${id}` });
    expect(viewParams(`cvw_${id}`)).toEqual({ kind: "custom", filter: `cvw_${id}` });
  });

  it("names no view for anything else", () => {
    for (const view of ["", "trash", "team_", "team_short", `usr_${id}`, `team_${id}x`]) {
      expect(viewParams(view)).toBeNull();
    }
  });
});

describe("listStatus", () => {
  it("prefers the view's own status, then the choice, then open", () => {
    expect(listStatus({ kind: "fixed", filter: "mine", status: "snoozed" }, "closed")).toBe("snoozed");
    expect(listStatus({ kind: "fixed", filter: "all" }, "closed")).toBe("closed");
    expect(listStatus({ kind: "fixed", filter: "all" }, undefined)).toBe("open");
  });
});
