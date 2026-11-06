import { describe, expect, it } from "vitest";

import type { AgentView, AiAgent, AiModelEntry, AiProviderKey } from "@/lib/api/ai";
import { domains, preferred, toggled } from "./agent/options";
import {
  applyChange,
  contextSize,
  keyStatus,
  maskedHint,
  modelCount,
  modelDetail,
  modelIds,
  modelsFor,
  readiness,
  usd,
} from "./format";
import { checkFields } from "./providers/problems";

const agent = {
  id: "agent",
  name: "Ada",
  enabled: false,
  product_name: null,
  tone: "friendly",
  guidance: "",
  enabled_channels: ["widget"],
  schedule: "always",
  assigned_behavior: "skip_when_human_active",
  answer_key_id: null,
  answer_model: null,
  fallback_key_id: null,
  fallback_model: null,
  max_ai_replies: 5,
  unknown_behavior: "offer_handoff",
  require_citations: true,
  default_language: "en",
  streaming: true,
  pii_redaction: true,
  monthly_spend_cap_micros: 25_000_000,
  actions_enabled: true,
  include_contact_context: "basic",
  office_hours: null,
  handoff: {},
} satisfies AiAgent;

function key(over: Partial<AiProviderKey>): AiProviderKey {
  return {
    id: "aik_1",
    name: "Anthropic",
    provider: "anthropic",
    base_url: null,
    auth_type: "api_key",
    key_hint: "••••abcd",
    status: "valid",
    status_reason: null,
    models: [],
    last_validated_at: null,
    last_used_at: null,
    created_at: "2026-11-01T00:00:00Z",
    ...over,
  };
}

const sonnet: AiModelEntry = {
  id: "claude-sonnet",
  display_name: "Claude Sonnet",
  family: "anthropic",
  suggested: ["answer"],
  context_tokens: 200_000,
  input_per_mtok: "3.00",
  output_per_mtok: "15.00",
  cached_input_per_mtok: null,
};

function view(over: Partial<AgentView>): AgentView {
  return {
    agent,
    keys: [],
    providers: [],
    catalog: {},
    ready: false,
    readiness_errors: [],
    managed: false,
    ...over,
  };
}

describe("ai format", () => {
  it("writes micro-dollars as dollars", () => {
    expect(usd(0)).toBe("$0.00");
    expect(usd(1_234_567)).toBe("$1.23");
    expect(usd(250_000_000)).toBe("$250");
  });

  it("reads a connection's status in three states", () => {
    expect(keyStatus({ status: "valid", provider: "openai" })).toBe("ok");
    expect(keyStatus({ status: "unverified", provider: "dimmi" })).toBe("ok");
    expect(keyStatus({ status: "invalid", provider: "openai" })).toBe("error");
    expect(keyStatus({ status: "error", provider: "openai" })).toBe("error");
    expect(keyStatus({ status: "unverified", provider: "openai" })).toBe("untested");
  });

  it("reads models sent as ids or as objects", () => {
    expect(modelIds(["a", { id: "b" }, 3, null])).toEqual(["a", "b"]);
    expect(modelIds(null)).toEqual([]);
    expect(modelCount(["a"])).toBe("1 model");
    expect(modelCount(["a", "b"])).toBe("2 models");
  });

  it("masks a key down to its last four", () => {
    expect(maskedHint("••••abcd")).toBe("•••• abcd");
    expect(maskedHint("••••")).toBe("••••");
    expect(maskedHint(null)).toBeNull();
  });

  it("folds a change into the agent", () => {
    expect(applyChange(agent, { monthly_cap_usd: null }).monthly_spend_cap_micros).toBeNull();
    expect(applyChange(agent, { monthly_cap_usd: 40 }).monthly_spend_cap_micros).toBe(40_000_000);
    expect(applyChange(agent, { raise_priority_on_handoff: true }).handoff).toEqual({ raise_priority: true });
    expect(applyChange(agent, { tone: "concise" }).tone).toBe("concise");
  });

  it("prices models from the catalog, and lists bare ids it does not know", () => {
    const catalog = { dimmi: [sonnet] };
    expect(modelsFor(key({ provider: "dimmi" }), catalog)).toEqual([sonnet]);
    expect(
      modelsFor(key({ models: [{ id: "claude-sonnet" }, { id: "mystery" }] }), catalog).map((m) => m.display_name),
    ).toEqual(["Claude Sonnet", "mystery"]);
    expect(modelDetail(sonnet)).toBe("$3 in · $15 out per 1M tokens · 200K context");
    expect(contextSize(1_000_000)).toBe("1M");
  });

  it("lists what stands between the agent and its first answer", () => {
    expect(readiness(view({})).map((step) => step.done)).toEqual([false, false, true]);
    const ready = view({
      keys: [key({})],
      agent: { ...agent, answer_key_id: "aik_1", answer_model: "claude-sonnet" },
    });
    expect(readiness(ready).every((step) => step.done)).toBe(true);
    const broken = view({
      keys: [key({ status: "invalid" })],
      agent: { ...agent, answer_key_id: "aik_1", answer_model: "claude-sonnet", enabled_channels: [] },
    });
    expect(readiness(broken).map((step) => step.done)).toEqual([false, false, false]);
  });

  it("keeps channels in the page's order", () => {
    expect(toggled(["email"], "widget", true)).toEqual(["widget", "email"]);
    expect(toggled(["widget", "email"], "widget", false)).toEqual(["email"]);
  });

  it("reads a typed link allowlist", () => {
    expect(domains("https://Example.com/docs\nstatus.example.com, example.com")).toEqual([
      "example.com",
      "status.example.com",
    ]);
  });

  it("suggests the model meant for a role", () => {
    const other = { ...sonnet, id: "small", suggested: ["classifier"] };
    expect(preferred([other, sonnet], "answer")?.id).toBe("claude-sonnet");
    expect(preferred([other], "answer")?.id).toBe("small");
  });

  it("finds what is missing from a provider form", () => {
    expect(checkFields("anthropic", { name: "", apiKey: "", baseUrl: "" })).toEqual({
      name: "Give the connection a name.",
      api_key: "Paste the API key.",
    });
    expect(checkFields("openai_compatible", { name: "Box", apiKey: "", baseUrl: "llm.local" })).toEqual({
      base_url: "Enter the server's address, starting with https://.",
    });
    expect(checkFields("openai_compatible", { name: "Box", apiKey: "", baseUrl: "http://llm.local/v1" })).toEqual({});
  });
});
