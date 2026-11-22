import { describe, expect, it } from "vitest";

import type { AgentDefinition } from "@/lib/api/agents";

import { agentChanges, guidanceLines } from "./agent-diff";

const base: AgentDefinition = {
  name: "Sales",
  product_name: null,
  tone: "friendly",
  guidance: "Be brief.\nNever quote prices.",
  enabled_channels: ["widget"],
  schedule: "always",
  assigned_behavior: "skip_when_human_active",
  answer_key_id: null,
  answer_model: null,
  fallback_key_id: null,
  fallback_model: null,
  max_ai_replies: 10,
  unknown_behavior: "offer_handoff",
  require_citations: true,
  languages: ["en"],
  default_language: "en",
  streaming: true,
  include_contact_context: "basic",
  pii_redaction: true,
  link_allowlist: [],
  handoff_team_id: null,
  raise_priority_on_handoff: false,
  handoff_mode: "full",
  always_handoff_tag_ids: [],
  knowledge_scope: { mode: "all", help_center_ids: [], source_ids: [] },
  procedure_scope: { mode: "all", ids: [] },
  action_scope: { mode: "all", ids: [] },
  transfer_agent_ids: [],
  placement: { channel_connection_ids: [], channel_room_ids: [], inbox_ids: [] },
};

const names: Record<string, string> = { aia_support: "Support" };
const name = (id: string) => names[id];

describe("agentChanges", () => {
  it("finds nothing between equal copies", () => {
    expect(agentChanges(base, { ...base }, name)).toEqual([]);
  });

  it("names what changed, in the Build tab's words", () => {
    const after = { ...base, tone: "professional", transfer_agent_ids: ["aia_support", "aia_gone"] };
    expect(agentChanges(base, after, name)).toEqual([
      { key: "tone", label: "Tone", before: "Friendly", after: "Professional" },
      { key: "transfer_agent_ids", label: "Hands over to", before: "None", after: "Support and 1 other" },
    ]);
  });
});

describe("guidanceLines", () => {
  it("marks lines added and taken out, keeping the rest", () => {
    expect(guidanceLines("Be brief.\nNever quote prices.", "Be brief.\nQuote list prices.\nOffer a demo.")).toEqual([
      { mark: "same", text: "Be brief." },
      { mark: "removed", text: "Never quote prices." },
      { mark: "added", text: "Quote list prices." },
      { mark: "added", text: "Offer a demo." },
    ]);
  });
});
