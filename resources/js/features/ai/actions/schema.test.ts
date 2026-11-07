import { describe, expect, it } from "vitest";

import { checkConnection, connectionBody, initialValues } from "./connection-values";
import { definitionOf, draftOf, newDraft, problemsOf, withSideEffect } from "./draft";
import {
  fieldNameOf,
  fieldsFromSchema,
  identityLike,
  nameProblem,
  parseSchema,
  placeholdersIn,
  responsePaths,
  schemaFromFields,
  toSnake,
  unknownPlaceholder,
  usesIdentity,
} from "./schema";

const schema = {
  type: "object" as const,
  properties: {
    invoice_id: { type: "string" as const, description: "Like inv_1001" },
    amount: { type: "number" as const, minimum: 1, maximum: 500 },
    reason: { type: "string" as const, enum: ["mistake", "duplicate"] },
  },
  required: ["invoice_id", "amount"],
  additionalProperties: false as const,
};

const connection = { id: "acn_1", kind: "http" as const, variables: { region: "eu" } };

describe("action schema", () => {
  it("turns a schema into rows and back", () => {
    const fields = fieldsFromSchema(schema);
    expect(fields.map((field) => [field.name, field.required])).toEqual([
      ["invoice_id", true],
      ["amount", true],
      ["reason", false],
    ]);
    expect(fields[2]?.choices).toBe("mistake, duplicate");
    expect(schemaFromFields(fields)).toEqual(schema);
  });

  it("leaves out rows without a name and limits only numbers", () => {
    const [first] = fieldsFromSchema(schema);
    if (!first) {
      throw new Error("no field");
    }
    const rows = [
      { ...first, name: "  " },
      { ...first, name: "note", minimum: "3", choices: "" },
    ];
    expect(schemaFromFields(rows).properties).toEqual({ note: { type: "string", description: "Like inv_1001" } });
  });

  it("reads a schema typed as JSON", () => {
    expect(parseSchema("{")).toEqual({ problem: "This is not valid JSON." });
    expect(parseSchema('{"type":"array"}')).toHaveProperty("problem");
    expect(parseSchema('{"type":"object","properties":3}')).toHaveProperty("problem");
    expect(parseSchema('{"type":"object"}')).toEqual({ schema: { type: "object", properties: {} } });
  });

  it("checks tool names", () => {
    expect(nameProblem("")).toBe("Give the action a name.");
    expect(nameProblem("Refund")).toContain("lowercase");
    expect(nameProblem("handoff")).toContain("reserved");
    expect(nameProblem("dimmi_refund")).toContain("reserved");
    expect(nameProblem("refund_invoice")).toBeNull();
    expect(toSnake("Refund an invoice!")).toBe("refund_an_invoice");
    expect(toSnake("Été 2 rêve")).toBe("ete_2_reve");
    expect(toSnake("42 things")).toBe("things");
  });

  it("finds placeholders that name nothing", () => {
    const known = { inputs: ["invoice_id"], variables: ["region"] };
    expect(placeholdersIn("/a/{input.invoice_id}/{contact.external_id}")).toEqual([
      "input.invoice_id",
      "contact.external_id",
    ]);
    expect(unknownPlaceholder("/v/{input.invoice_id}?r={connection.region}", known)).toBeNull();
    expect(unknownPlaceholder("{contact.attr.plan} {contact.custom_attributes.plan}", known)).toBeNull();
    expect(unknownPlaceholder("{conversation.id} {workspace.id} {call.id}", known)).toBeNull();
    expect(unknownPlaceholder("/v/{input.order}", known)).toBe("input.order");
    expect(unknownPlaceholder("{connection.zone}", known)).toBe("connection.zone");
    expect(unknownPlaceholder("{contact.password}", known)).toBe("contact.password");
    expect(unknownPlaceholder("{secret.key}", known)).toBe("secret.key");
    expect(usesIdentity(["{contact.id}"])).toBe(false);
    expect(usesIdentity(["x", "{contact.email}"])).toBe(true);
  });

  it("spots inputs that name the customer", () => {
    expect(identityLike("customer_id")).toBe(true);
    expect(identityLike("email")).toBe(true);
    expect(identityLike("user_external_id")).toBe(true);
    expect(identityLike("id")).toBe(false);
    expect(identityLike("invoice_id")).toBe(false);
  });

  it("lists the paths of an answer", () => {
    const body = { data: [{ status: "active", plan: { name: "Pro" } }], tags: ["a"], empty: [], total: 2 };
    expect(responsePaths(body)).toEqual(["data.0.status", "data.0.plan.name", "tags", "empty", "total"]);
    expect(responsePaths("text")).toEqual([]);
    expect(fieldNameOf("data.0.status")).toBe("status");
    expect(fieldNameOf("shipment.carrier")).toBe("shipment_carrier");
    expect(fieldNameOf("0")).toBe("value");
  });
});

describe("action draft", () => {
  it("starts as a lookup on its connection", () => {
    const draft = newDraft(connection);
    expect(draft.side_effect).toBe("read");
    expect(draft.confirmation).toBe("none");
    expect(draft.channels).toEqual(["widget"]);
  });

  it("moves permissions to the safe default when it starts changing data", () => {
    const write = withSideEffect({ ...newDraft(connection), channels: ["widget", "email"] }, "write");
    expect(write.confirmation).toBe("customer");
    expect(write.channels).toEqual(["widget"]);
    expect(withSideEffect(write, "write")).toBe(write);
    const read = withSideEffect({ ...write, approval: "admin" }, "read");
    expect([read.confirmation, read.approval]).toEqual(["none", "none"]);
  });

  it("survives a round trip through the stored definition", () => {
    const stored = {
      ...definitionOf(newDraft(connection), "http"),
      name: "refund_invoice",
      title: "Refund an invoice",
      description: "Refund one of the signed-in customer's invoices.",
      side_effect: "write" as const,
      method: "POST" as const,
      path_template: "/refunds",
      query: { dry: "false" },
      body_template: { customer: "{contact.external_id}", invoice_id: "{input.invoice_id}" },
      input_schema: schema,
      response_fields: [{ name: "refund", path: "id" }],
      confirmation: "customer" as const,
      approval: "teammate" as const,
    };
    expect(definitionOf(draftOf(stored), "http")).toEqual(stored);
  });

  it("reads an action as the server answers it", () => {
    const draft = draftOf({
      ...definitionOf(newDraft(connection), "http"),
      method: null,
      path_template: null,
      mcp_tool_name: "issue_refund",
      bound_arguments: { customer_id: "{contact.external_id}", limit: 5 },
      input_schema: { type: "object" },
      channels: ["sms"],
    });
    expect([draft.method, draft.path, draft.tool]).toEqual(["GET", "/", "issue_refund"]);
    expect(draft.bound.map((row) => [row.name, row.value])).toEqual([
      ["customer_id", "{contact.external_id}"],
      ["limit", "5"],
    ]);
    expect(draft.fields).toEqual([]);
    expect(draft.channels).toEqual(["widget"]);
  });

  it("keeps a body that is not JSON as text and drops it for GET", () => {
    const draft = { ...newDraft(connection), method: "POST" as const, body: "a=b" };
    expect(definitionOf(draft, "http").body_template).toBe("a=b");
    expect(definitionOf({ ...draft, method: "GET" }, "http").body_template).toBeNull();
    const mcp = definitionOf({ ...draft, tool: "issue_refund" }, "mcp");
    expect(mcp.mcp_tool_name).toBe("issue_refund");
    expect(mcp).not.toHaveProperty("method");
    expect(mcp).not.toHaveProperty("path_template");
    expect(definitionOf(draft, "http")).not.toHaveProperty("mcp_tool_name");
  });

  it("names what to fix beside each field", () => {
    const empty = problemsOf({ ...newDraft(connection), path: "orders", channels: [] }, connection);
    expect(Object.keys(empty).sort()).toEqual(["channels", "description", "name", "path", "title"]);

    const good = {
      ...newDraft(connection),
      name: "get_order",
      title: "Check an order",
      description: "Look up one of the signed-in customer's orders by number.",
      path: "/orders/{input.order}",
    };
    expect(problemsOf(good, connection).path).toContain("{input.order}");
    const fields = fieldsFromSchema({ type: "object", properties: { order: { type: "string" } } });
    expect(problemsOf({ ...good, fields }, connection)).toEqual({});
    expect(problemsOf({ ...good, fields, side_effect: "write" }, connection).identity).toContain("signed-in");
    expect(problemsOf({ ...good, fields: [...fields, ...fields] }, connection).fields).toContain("Two inputs");
    expect(
      problemsOf({ ...good, fields: [{ ...fields[0], name: "Order" }] as typeof fields }, connection).fields,
    ).toContain("lowercase");
  });

  it("checks an MCP action's tool and binding", () => {
    const mcp = { ...connection, kind: "mcp" as const };
    const base = {
      ...newDraft(mcp),
      name: "issue_refund",
      title: "Refund",
      description: "Refund an invoice of the signed-in customer.",
    };
    expect(problemsOf(base, mcp).tool).toBeDefined();
    expect(problemsOf({ ...base, tool: "issue_refund", side_effect: "write" }, mcp).identity).toBeDefined();
    const bound = [{ key: "k", name: "customer_id", value: "{contact.external_id}" }];
    expect(problemsOf({ ...base, tool: "issue_refund", side_effect: "write", bound }, mcp)).toEqual({});
  });
});

describe("connection values", () => {
  it("asks for what a new connection needs", () => {
    const values = initialValues("http");
    expect(Object.keys(checkConnection(values, false)).sort()).toEqual(["name", "token", "url"]);
    expect(checkConnection({ ...values, name: "Billing", url: "https://api.x.com" }, true)).toEqual({});
    const basic = { ...values, name: "B", url: "https://x.com", auth: "basic" as const };
    expect(Object.keys(checkConnection(basic, false)).sort()).toEqual(["password", "username"]);
    const header = { ...values, name: "B", url: "https://x.com", auth: "api_key_header" as const, token: "k" };
    expect(checkConnection({ ...header, header: "Bad Header" }, false).header).toBeDefined();
  });

  it("sends secrets only when typed", () => {
    const values = { ...initialValues("mcp"), name: " Tools ", url: "https://mcp.x.com/mcp/", auth: "none" as const };
    expect(connectionBody("mcp", values)).toEqual({
      name: "Tools",
      kind: "mcp",
      base_url: "https://mcp.x.com/mcp",
      auth_type: "none",
      auth_config: {},
    });
    const http = connectionBody("http", { ...values, auth: "api_key_query", token: "k", param: "key" });
    expect(http).toMatchObject({ token: "k", auth_config: { query_param: "key" }, timeout_ms: 8000 });
  });
});
