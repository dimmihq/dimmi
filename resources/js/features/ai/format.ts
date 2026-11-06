/*
 * How the agent's numbers and states read in words: money from the
 * micro-dollars the server counts in, a connection's status and the
 * models it offers, a model's price and context window, and which steps
 * still stand between the agent and its first answer. A change is also
 * folded into the agent here, so a page shows it before the server
 * confirms it.
 */
import type { AgentPatch, AgentView, AiAgent, AiModelEntry, AiProviderKey } from "@/lib/api/ai";

/*
 * AgentChange is a change to the agent as the pages make it: the patch
 * the server takes, where a null cap means no cap, plus the classifier
 * model and the link allowlist.
 */
export type AgentChange = Omit<AgentPatch, "monthly_cap_usd"> & {
  monthly_cap_usd?: number | null;
  classifier_key_id?: string;
  classifier_model?: string;
  link_allowlist?: string[];
};

/*
 * AgentExtras are the agent's fields the server sends that the shared
 * type does not name yet.
 */
export interface AgentExtras {
  classifier_key_id?: string | null;
  classifier_model?: string | null;
  link_allowlist?: string[] | null;
}

/*
 * toPatch hands a change to the API, whose patch type does not yet know
 * a null cap or the extra fields; the server reads them as sent.
 */
export function toPatch(change: AgentChange): AgentPatch {
  return change as AgentPatch;
}

/*
 * extras reads the agent's fields the shared type does not name yet.
 */
export function extras(agent: AiAgent): AgentExtras {
  return agent as AiAgent & AgentExtras;
}

/*
 * applyChange is the agent as it will be once the server takes the
 * change, for showing it straight away.
 */
export function applyChange(agent: AiAgent, change: AgentChange): AiAgent {
  const { monthly_cap_usd, raise_priority_on_handoff, handoff_team_id, ...rest } = change;
  const next: AiAgent = { ...agent, ...(rest as Partial<AiAgent>) };

  if (monthly_cap_usd !== undefined) {
    next.monthly_spend_cap_micros = monthly_cap_usd === null ? null : Math.round(monthly_cap_usd * 1_000_000);
  }
  if (raise_priority_on_handoff !== undefined || handoff_team_id !== undefined) {
    next.handoff = {
      ...agent.handoff,
      ...(raise_priority_on_handoff === undefined ? {} : { raise_priority: raise_priority_on_handoff }),
      ...(handoff_team_id === undefined ? {} : { target_team_id: handoff_team_id }),
    };
  }

  return next;
}

/*
 * usd writes micro-dollars as dollars: cents below a hundred dollars,
 * whole dollars above.
 */
export function usd(micros: number): string {
  const dollars = micros / 1_000_000;

  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: dollars >= 100 ? 0 : 2,
    maximumFractionDigits: dollars >= 100 ? 0 : 2,
  }).format(dollars);
}

/*
 * KeyStatus is how a connection stands, in the three states a person
 * reads: working, failing, or never checked.
 */
export type KeyStatus = "ok" | "error" | "untested";

export function keyStatus(key: Pick<AiProviderKey, "status" | "provider">): KeyStatus {
  if (key.status === "valid" || key.provider === "dimmi") {
    return "ok";
  }
  if (key.status === "invalid" || key.status === "error") {
    return "error";
  }

  return "untested";
}

export const statusLabels: Record<KeyStatus, string> = {
  ok: "Connected",
  error: "Error",
  untested: "Not checked",
};

/*
 * modelIds reads the models a connection offers, which the server sends
 * as plain ids or as objects carrying one.
 */
export function modelIds(models: unknown): string[] {
  if (!Array.isArray(models)) {
    return [];
  }

  return models.flatMap((model: unknown) => {
    if (typeof model === "string") {
      return [model];
    }
    if (model && typeof model === "object" && "id" in model && typeof model.id === "string") {
      return [model.id];
    }

    return [];
  });
}

export function modelCount(models: unknown): string {
  const count = modelIds(models).length;

  return count === 1 ? "1 model" : `${count} models`;
}

/*
 * maskedHint shows the last four of a key behind dots, the way every
 * key is shown once saved.
 */
export function maskedHint(hint: string | null): string | null {
  if (!hint) {
    return null;
  }
  const last = hint.replace(/[•.*\s]/g, "").slice(-4);

  return last ? `•••• ${last}` : "••••";
}

/*
 * usableKeys are the connections the agent can answer with: the
 * built-in one and every key whose check passed.
 */
export function usableKeys(keys: AiProviderKey[]): AiProviderKey[] {
  return keys.filter((key) => keyStatus(key) === "ok");
}

/*
 * modelsFor lists the models a connection offers, priced from the
 * catalog when it knows them: the catalog of its provider first, then
 * the ids the provider listed when the key was checked.
 */
export function modelsFor(key: AiProviderKey | undefined, catalog: AgentView["catalog"]): AiModelEntry[] {
  if (!key) {
    return [];
  }
  const own = catalog[key.provider];
  if (own && own.length > 0) {
    return own;
  }
  const known = new Map(
    Object.values(catalog)
      .flat()
      .map((entry) => [entry.id, entry]),
  );

  return modelIds(key.models).map((id) => known.get(id) ?? bareModel(id, key.provider));
}

function bareModel(id: string, provider: string): AiModelEntry {
  return {
    id,
    display_name: id,
    family: provider === "anthropic" ? "anthropic" : provider === "openai" ? "openai" : "openweight",
    suggested: [],
    context_tokens: 0,
    input_per_mtok: null,
    output_per_mtok: null,
    cached_input_per_mtok: null,
  };
}

/*
 * modelDetail is a model's price per million tokens in and out, and how
 * much it can read at once, for the line under its name.
 */
export function modelDetail(model: AiModelEntry): string {
  const parts: string[] = [];
  if (model.input_per_mtok && model.output_per_mtok) {
    parts.push(`$${trimPrice(model.input_per_mtok)} in · $${trimPrice(model.output_per_mtok)} out per 1M tokens`);
  }
  if (model.context_tokens > 0) {
    parts.push(`${contextSize(model.context_tokens)} context`);
  }

  return parts.join(" · ");
}

function trimPrice(price: string): string {
  const value = Number(price);

  return Number.isFinite(value) ? String(Number(value.toFixed(2))) : price;
}

export function contextSize(tokens: number): string {
  if (tokens >= 1_000_000) {
    return `${Number((tokens / 1_000_000).toFixed(1))}M`;
  }

  return `${Math.round(tokens / 1000)}K`;
}

/*
 * Step is one thing the agent needs before it can answer, done or not,
 * with where to fix it.
 */
export interface Step {
  id: "provider" | "model" | "channels";
  label: string;
  hint: string;
  done: boolean;
}

/*
 * readiness lists the steps between the agent and its first answer.
 */
export function readiness(view: AgentView): Step[] {
  const usable = usableKeys(view.keys);
  const { agent } = view;
  const answerKey = usable.find((key) => key.id === agent.answer_key_id);

  return [
    {
      id: "provider",
      label: "Connect a provider",
      hint: "The models the agent writes with.",
      done: usable.length > 0,
    },
    {
      id: "model",
      label: "Choose a model",
      hint: "Which model answers your customers.",
      done: answerKey !== undefined && Boolean(agent.answer_model),
    },
    {
      id: "channels",
      label: "Pick where it answers",
      hint: "At least one channel, such as the messenger.",
      done: agent.enabled_channels.length > 0,
    },
  ];
}
