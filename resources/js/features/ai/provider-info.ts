/*
 * What the pages say about each provider: its name, the line under it
 * in the picker, and how its key is asked for.
 */

/*
 * ProviderInfo is what the pages say about a provider: its name, the
 * line under it in the picker, and where to find a key.
 */
export interface ProviderInfo {
  name: string;
  blurb: string;
  keyLabel: string;
  keyPlaceholder: string;
}

export const providerInfo: Record<string, ProviderInfo> = {
  dimmi: {
    name: "dimmi",
    blurb: "Included with dimmi. No key needed.",
    keyLabel: "",
    keyPlaceholder: "",
  },
  anthropic: {
    name: "Anthropic",
    blurb: "Claude models with your own API key.",
    keyLabel: "API key",
    keyPlaceholder: "sk-ant-…",
  },
  openai: {
    name: "OpenAI",
    blurb: "GPT models with your own API key.",
    keyLabel: "API key",
    keyPlaceholder: "sk-…",
  },
  chatgpt: {
    name: "ChatGPT subscription",
    blurb: "Sign in with a ChatGPT plan instead of a key.",
    keyLabel: "",
    keyPlaceholder: "",
  },
  engy: {
    name: "Engy",
    blurb: "Open-weight models, hosted for you.",
    keyLabel: "API key",
    keyPlaceholder: "engy-…",
  },
  baseten: {
    name: "Baseten",
    blurb: "Open-weight models such as GLM and Kimi on Baseten, with your own API key.",
    keyLabel: "API key",
    keyPlaceholder: "Your Baseten API key",
  },
  openai_compatible: {
    name: "OpenAI-compatible",
    blurb: "Ollama, vLLM, LiteLLM or any server that speaks OpenAI.",
    keyLabel: "API key (optional)",
    keyPlaceholder: "Leave empty if the server needs none",
  },
};

/*
 * providerName is a provider's name, or its id when it is new to the
 * pages.
 */
export function providerName(provider: string): string {
  return providerInfo[provider]?.name ?? provider;
}
