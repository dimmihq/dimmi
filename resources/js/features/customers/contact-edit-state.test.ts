import { describe, expect, it } from "vitest";

import type { AttributeDefinition } from "@/lib/api/attributes";
import type { Contact } from "@/lib/api/customers";

import { changesOf, editStateOf } from "./contact-edit-state";

const vip = { id: "tag_1", object: "tag", name: "vip", color: null, icon: null };

const contact = {
  name: "Ana Lima",
  email: "ana@lima.example",
  phone: null,
  notes: null,
  tags: [vip],
  custom_attributes: { seats: 4, legacy: "yes" },
} as unknown as Contact;

const seats = { key: "seats", data_type: "number", of: "contact", status: "active" } as AttributeDefinition;

describe("contact edit state", () => {
  it("sends nothing when nothing changed", () => {
    expect(changesOf(contact, [seats], editStateOf(contact, [seats]))).toEqual({});
  });

  it("sends only what changed, typed, with tags by name and id", () => {
    const state = editStateOf(contact, [seats]);
    state.name = " ";
    state.typed.seats = "5";
    state.extra = [{ id: "x", key: "plan", value: "pro" }];
    state.tags = [{ ...vip, id: "new:beta", name: "beta" }];

    expect(changesOf(contact, [seats], state)).toEqual({
      name: null,
      add_tags: ["beta"],
      remove_tags: ["tag_1"],
      custom_attributes: { legacy: null, seats: 5, plan: "pro" },
    });
  });

  it("sends the company list when one is added or removed, and the default when it moves", () => {
    const acme = { id: "cmp_1", object: "company" as const, name: "Acme", domain: null };
    const globex = { id: "cmp_2", object: "company" as const, name: "Globex", domain: null };
    const member = {
      ...contact,
      company: acme,
      companies: [
        { ...acme, is_default: true, added_at: "" },
        { ...globex, is_default: false, added_at: "" },
      ],
    } as Contact;

    const moved = editStateOf(member, []);
    moved.defaultCompany = "cmp_2";
    expect(changesOf(member, [], moved)).toEqual({ company_id: "cmp_2" });

    const removed = editStateOf(member, []);
    removed.companies = [];
    removed.defaultCompany = null;
    expect(changesOf(member, [], removed).company_ids).toEqual([]);
    expect(changesOf(member, [], removed).company_id).toBeUndefined();
  });
});
