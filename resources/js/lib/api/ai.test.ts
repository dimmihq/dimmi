import { describe, expect, it } from "vitest";

import { type StoredAgentView, settingsPageView } from "./ai";

function view(agent: Record<string, unknown>, settings: Record<string, unknown>): StoredAgentView {
  return {
    agent: { id: "aia_1", name: "dimmi", enabled: false, draft: {}, ...agent },
    settings: { monthly_spend_cap_micros: null, ...settings },
  } as unknown as StoredAgentView;
}

describe("settingsPageView", () => {
  it("shows the workspace's shared model for an agent that has picked none", () => {
    const got = settingsPageView(
      view(
        { answer_key_id: null, answer_model: null, fallback_key_id: null, fallback_model: null },
        { answer_key_id: "aik_1", answer_model: "gpt-6-luna", monthly_spend_cap_micros: 25_000_000 },
      ),
    );

    expect(got.agent.answer_key_id).toBe("aik_1");
    expect(got.agent.answer_model).toBe("gpt-6-luna");
    expect(got.agent.monthly_spend_cap_micros).toBe(25_000_000);
    expect(got.agent.fallback_model).toBeNull();
  });

  it("keeps the model an agent picked for itself, and its working copy over what is live", () => {
    const got = settingsPageView(
      view(
        { answer_key_id: "aik_2", answer_model: "claude-haiku-5-5", name: "Ada", draft: { name: "Sky" } },
        { answer_key_id: "aik_1", answer_model: "gpt-6-luna" },
      ),
    );

    expect(got.agent.answer_key_id).toBe("aik_2");
    expect(got.agent.answer_model).toBe("claude-haiku-5-5");
    expect(got.agent.name).toBe("Sky");
  });
});
