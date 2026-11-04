import { describe, expect, it } from "vitest";

import type { Permission } from "@/lib/api/roles";

import { presets, withinBase } from "./presets";

const catalog: Permission[] = [
  {
    key: "members:read",
    group: "members:read",
    label: "",
    description: "",
    assignable: true,
    base_roles: ["member", "admin"],
  },
  {
    key: "conversations:read",
    group: "conversations:read",
    label: "",
    description: "",
    assignable: true,
    base_roles: ["member", "admin"],
  },
  {
    key: "conversations:write",
    group: "conversations:write",
    label: "",
    description: "",
    assignable: true,
    base_roles: ["member", "admin"],
  },
  { key: "settings:read", group: "settings:read", label: "", description: "", assignable: true, base_roles: ["admin"] },
  { key: "billing:read", group: "billing:read", label: "", description: "", assignable: true, base_roles: ["admin"] },
  { key: "workspace:delete", group: "settings:write", label: "", description: "", assignable: false, base_roles: [] },
];

describe("presets", () => {
  it("keeps members read and drops what the base role cannot hold", () => {
    expect(withinBase(["settings:read", "conversations:read"], "member", catalog)).toEqual([
      "conversations:read",
      "members:read",
    ]);
    expect(withinBase(["workspace:delete"], "admin", catalog)).toEqual(["members:read"]);
  });

  it("offers a read only role holding reads alone", () => {
    const readOnly = presets(catalog).find((preset) => preset.key === "read-only");

    expect(readOnly?.draft.permissions).toEqual(["conversations:read", "members:read"]);
  });

  it("bases the billing admin on admin", () => {
    const billing = presets(catalog).find((preset) => preset.key === "billing");

    expect(billing?.draft.base_role).toBe("admin");
    expect(billing?.draft.permissions).toEqual(["billing:read", "members:read", "settings:read"]);
  });
});
