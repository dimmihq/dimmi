import { describe, expect, it } from "vitest";

import type { AiProviderDescriptor } from "@/lib/api/ai";
import { providerGroups } from "./choices";

function descriptor(id: string, auth_type: AiProviderDescriptor["auth_type"]): AiProviderDescriptor {
  return { id, name: id, auth_type, key_required: auth_type === "api_key", docs_url: "", available: true };
}

describe("providerGroups", () => {
  it("never offers dimmi's own models or engy", () => {
    const groups = providerGroups([
      descriptor("dimmi", "managed"),
      descriptor("anthropic", "api_key"),
      descriptor("engy", "api_key"),
      descriptor("chatgpt", "oauth"),
    ]);

    const offered = groups.flatMap((group) => group.providers.map((provider) => provider.id));
    expect(offered).toEqual(["anthropic", "chatgpt"]);
  });

  it("puts keys first and subscriptions after, in the server's order", () => {
    const groups = providerGroups([
      descriptor("chatgpt", "oauth"),
      descriptor("openai", "api_key"),
      descriptor("anthropic", "api_key"),
      descriptor("openai_compatible", "none"),
    ]);

    expect(groups.map((group) => group.id)).toEqual(["keys", "plans"]);
    expect(groups[0]?.providers.map((provider) => provider.id)).toEqual(["openai", "anthropic", "openai_compatible"]);
    expect(groups[1]?.providers.map((provider) => provider.id)).toEqual(["chatgpt"]);
  });

  it("leaves out a heading with nothing under it", () => {
    const groups = providerGroups([descriptor("anthropic", "api_key")]);

    expect(groups.map((group) => group.id)).toEqual(["keys"]);
  });
});
