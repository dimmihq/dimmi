import type { WebhookEventType } from "@/lib/api/webhooks";

import { coveredTypes, groupEventTypes, subscriptionFor, subscriptionLine } from "./webhooks";

function type(name: string, group: string, namedOnly = false): WebhookEventType {
  return {
    object: "webhook_event_type",
    type: name,
    object_type: name.split(".")[0] ?? name,
    group,
    description: "",
    notes_opt_in: false,
    named_only: namedOnly,
  };
}

const catalog = [
  type("conversation.created", "Conversations"),
  type("conversation.closed", "Conversations"),
  type("contact.created", "Contacts"),
  type("contact_event.tracked", "Contacts", true),
];

describe("groupEventTypes", () => {
  it("keeps the catalog's order under each group", () => {
    const groups = groupEventTypes(catalog);
    expect(groups.map((g) => g.name)).toEqual(["Conversations", "Contacts"]);
    expect(groups[1]?.types.map((t) => t.type)).toEqual(["contact.created", "contact_event.tracked"]);
  });
});

describe("coveredTypes", () => {
  it("expands wildcards but never to named-only types", () => {
    expect(coveredTypes(["*"], catalog)).toEqual(["conversation.created", "conversation.closed", "contact.created"]);
    expect(coveredTypes(["conversation.*"], catalog)).toEqual(["conversation.created", "conversation.closed"]);
    expect(coveredTypes(["contact_event.tracked"], catalog)).toEqual(["contact_event.tracked"]);
  });
});

describe("subscriptionFor", () => {
  it("saves every event as *, and names each type once a named-only one is picked", () => {
    const all = ["conversation.created", "conversation.closed", "contact.created"];
    expect(subscriptionFor(all, catalog)).toEqual(["*"]);
    expect(subscriptionFor([...all, "contact_event.tracked"], catalog)).toEqual([...all, "contact_event.tracked"]);
    expect(subscriptionFor(["contact.created"], catalog)).toEqual(["contact.created"]);
  });
});

describe("subscriptionLine", () => {
  it("names two types and counts the rest", () => {
    expect(subscriptionLine(["*"])).toBe("Every event");
    expect(subscriptionLine(["a.b", "c.d", "e.f"])).toBe("a.b, c.d and 1 more");
  });
});
