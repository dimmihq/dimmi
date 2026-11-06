/*
 * The AI settings: how Dimmi answers customers and when it hands over,
 * the actions it may take in your systems, and the model keys it and
 * Operator run on. The workspace pays the provider directly with its own
 * keys, so the spend cap here is the workspace's own guard.
 */

import { RiAnthropicFill, RiFlashlightLine, RiLink, RiOpenaiFill } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import {
  type AgentPatch,
  type AgentView,
  type AiProviderDescriptor,
  type AiProviderKey,
  agentQuery,
  aiKey,
  aiUsageQuery,
  createAiKey,
  deleteAiKey,
  testAiKey,
  updateAgent,
} from "@/lib/api/ai";
import { Action, Choice, Danger, Field, Toggle } from "../controls";
import { Empty, Glyph, Item, Logo, Row, Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";

const channelOptions = ["Messenger", "Messenger and email", "All channels"] as const;
const channelValues: Record<(typeof channelOptions)[number], string[]> = {
  Messenger: ["widget"],
  "Messenger and email": ["widget", "email"],
  "All channels": ["widget", "email", "api"],
};

const scheduleOptions = ["Always", "Outside office hours", "During office hours"] as const;
const scheduleValues: Record<(typeof scheduleOptions)[number], string> = {
  Always: "always",
  "Outside office hours": "outside_office_hours",
  "During office hours": "inside_office_hours",
};

const assignedOptions = ["Step back while they're active", "Never answer", "Keep answering"] as const;
const assignedValues: Record<(typeof assignedOptions)[number], string> = {
  "Step back while they're active": "skip_when_human_active",
  "Never answer": "never_when_assigned",
  "Keep answering": "always",
};

const unknownOptions = ["Offer a teammate", "Hand over straight away"] as const;
const unknownValues: Record<(typeof unknownOptions)[number], string> = {
  "Offer a teammate": "offer_handoff",
  "Hand over straight away": "handoff",
};

const contactOptions = ["Basic", "None", "Everything"] as const;
const contactValues: Record<(typeof contactOptions)[number], string> = {
  Basic: "basic",
  None: "none",
  Everything: "full",
};

/*
 * inverse maps a value map back to its option label.
 */
function inverse<T extends string>(values: Record<T, string[] | string>, value: string | string[]): T | undefined {
  for (const [label, v] of Object.entries(values)) {
    if (JSON.stringify(v) === JSON.stringify(value)) {
      return label as T;
    }
  }
  return undefined;
}

type AgentViewType = AgentView;

function channelsLabel(channels: string[]): (typeof channelOptions)[number] {
  return inverse(channelValues, channels) ?? "All channels";
}

function usd(micros: number | null): string {
  if (micros == null) {
    return "";
  }
  return String(micros / 1_000_000);
}

export function AgentSettings() {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const view = useQuery(agentQuery(workspace));
  const usage = useQuery(aiUsageQuery(workspace));

  const save = useMutation({
    mutationFn: (patch: AgentPatch) => updateAgent(workspace, patch),
    onSuccess: () => {
      toast.success("Saved");
      client.invalidateQueries({ queryKey: aiKey(workspace) });
    },
    onError: () => toast.error("Could not save. Try again."),
  });

  if (!view.data) {
    return null;
  }
  const { agent, ready, readiness_errors: readiness } = view.data;
  const patch = (p: AgentPatch) => save.mutate(p);

  const readinessText = ready ? "Dimmi is ready." : `Not ready: ${readiness.join(", ") || "finish setup"}.`;

  return (
    <SettingsPage
      title="Dimmi agent"
      description="Dimmi answers customers from your knowledge and hands over when it should."
    >
      <Section title="Status" description={readinessText}>
        <Row
          label="Answer customers"
          description={ready ? "Dimmi replies to new customer messages." : "Finish setup to switch this on."}
        >
          <Toggle
            label="Answer customers"
            on={agent.enabled}
            disabled={!ready}
            onChange={(on) => patch({ enabled: on })}
          />
        </Row>
      </Section>
      <Section title="Identity">
        <Row label="Name" description="How the agent introduces itself.">
          <Field label="Agent name" value={agent.name} onChange={(v) => patch({ name: v })} />
        </Row>
        <Row label="Product" description="What it calls your product.">
          <Field label="Product name" value={agent.product_name ?? ""} onChange={(v) => patch({ product_name: v })} />
        </Row>
        <Row label="Tone">
          <Choice
            label="Tone"
            options={["friendly", "professional", "concise"]}
            value={agent.tone}
            onChange={(v) => patch({ tone: v })}
          />
        </Row>
        <Row label="Guidance" description="Anything Dimmi should always know or do, up to 2,000 characters.">
          <Field label="Guidance" value={agent.guidance} width="w-80" onChange={(v) => patch({ guidance: v })} />
        </Row>
      </Section>
      <Section title="When Dimmi answers">
        <Row label="Channels">
          <Choice
            label="Channels"
            options={[...channelOptions]}
            value={channelsLabel(agent.enabled_channels)}
            onChange={(v) => patch({ enabled_channels: channelValues[v as (typeof channelOptions)[number]] })}
          />
        </Row>
        <Row label="Hours">
          <Choice
            label="Hours"
            options={[...scheduleOptions]}
            value={inverse(scheduleValues, agent.schedule) ?? "Always"}
            onChange={(v) => patch({ schedule: scheduleValues[v as (typeof scheduleOptions)[number]] })}
          />
        </Row>
        <Row label="Once a teammate is assigned">
          <Choice
            label="Once a teammate is assigned"
            options={[...assignedOptions]}
            value={inverse(assignedValues, agent.assigned_behavior) ?? assignedOptions[0]}
            onChange={(v) => patch({ assigned_behavior: assignedValues[v as (typeof assignedOptions)[number]] })}
          />
        </Row>
      </Section>
      <AnswersSection view={view.data} patch={patch} />

      <Section title="Privacy">
        <Row label="Customer details Dimmi sees">
          <Choice
            label="Customer details Dimmi sees"
            options={[...contactOptions]}
            value={inverse(contactValues, agent.include_contact_context) ?? contactOptions[0]}
            onChange={(v) => patch({ include_contact_context: contactValues[v as (typeof contactOptions)[number]] })}
          />
        </Row>
        <Row
          label="Hide personal data"
          description="Remove emails, phone numbers and cards before they reach the model."
        >
          <Toggle label="Hide personal data" on={agent.pii_redaction} onChange={(on) => patch({ pii_redaction: on })} />
        </Row>
      </Section>
      <Section title="Handover">
        <Row label="Raise priority on handover">
          <Toggle
            label="Raise priority on handover"
            on={agent.raise_priority_on_handoff}
            onChange={(on) => patch({ raise_priority_on_handoff: on })}
          />
        </Row>
      </Section>
      <Section
        title="Spend"
        description={`Alerts at 50%, 80% and 100% of the cap. This month: $${(usage.data?.cost_micros ?? 0) / 1_000_000} over ${usage.data?.runs_count ?? 0} runs.`}
      >
        <Row label="Monthly spend cap" description="Dimmi pauses when the workspace reaches it.">
          <Field
            label="Monthly spend cap"
            value={usd(agent.monthly_spend_cap_micros)}
            width="w-28"
            onChange={(v) => patch({ monthly_cap_usd: Number(v) || 0 })}
          />
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function ActionsSettings() {
  return (
    <SettingsPage
      title="Actions"
      description="Let Dimmi look things up or change them in your systems, such as an order or a subscription."
    >
      <Section
        title="Your APIs"
        description="An API or MCP server of yours that actions call."
        action={<Action>Add API</Action>}
      >
        <Empty action={<Action>Start from Stripe</Action>}>No APIs yet</Empty>
      </Section>
      <Section
        title="Actions"
        description="Each action says what it does and who must approve it."
        action={<Action>New action</Action>}
      >
        <Item
          mark={<Glyph icon={RiFlashlightLine} />}
          title="Look up an order"
          detail="Reads · Needs a verified customer · Draft"
        >
          <Action>Edit</Action>
        </Item>
      </Section>
      <Section title="All actions">
        <Row label="Allow actions" description="Turn off to stop every action at once.">
          <Toggle label="Allow actions" on />
        </Row>
      </Section>
    </SettingsPage>
  );
}

const providerIcons: Record<string, { icon: typeof RiOpenaiFill; brand: string }> = {
  anthropic: { icon: RiAnthropicFill, brand: "#CC785C" },
  openai: { icon: RiOpenaiFill, brand: "#000000" },
};

function ProviderRow({
  keyData,
  onTest,
  onDelete,
}: {
  keyData: AiProviderKey;
  onTest: () => void;
  onDelete: () => void;
}) {
  const icon = providerIcons[keyData.provider];
  const detail = `${keyData.key_hint ?? keyData.name} · ${keyData.models?.length ?? 0} models`;
  return (
    <Item
      mark={icon ? <Logo icon={icon.icon} brand={icon.brand} /> : <Glyph icon={RiLink} />}
      title={keyData.name}
      detail={detail}
    >
      <Chip variant="caption" color={keyData.status === "valid" ? "lime" : "gray"}>
        {keyData.status}
      </Chip>
      <Action onPress={onTest}>Test</Action>
      <Danger onPress={onDelete}>Remove</Danger>
    </Item>
  );
}

function AddKeyForm({
  providers,
  onAdd,
}: {
  providers: AiProviderDescriptor[];
  onAdd: (provider: string, name: string, key: string, url: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [provider, setProvider] = useState("anthropic");
  const [name, setName] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");

  if (!open) {
    return <Action onPress={() => setOpen(true)}>Add key</Action>;
  }

  const choices = providers.filter((p) => p.auth_type === "api_key").map((p) => p.id);

  return (
    <div className="flex items-center gap-2">
      <Choice label="Provider" options={choices} value={provider} onChange={setProvider} />
      <Field label="Name" placeholder="Name" onChange={setName} />
      <Field label="Key" placeholder="sk-…" onChange={setApiKey} />
      {provider === "openai_compatible" && <Field label="Base URL" placeholder="https://…" onChange={setBaseUrl} />}
      <Button
        size="small"
        onClick={() => {
          onAdd(provider, name || provider, apiKey, baseUrl);
          setOpen(false);
        }}
      >
        Save
      </Button>
    </div>
  );
}

export function ProvidersSettings() {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const view = useQuery(agentQuery(workspace));

  const refresh = () => client.invalidateQueries({ queryKey: aiKey(workspace) });
  const add = useMutation({
    mutationFn: (input: { provider: string; name: string; api_key: string; base_url?: string }) =>
      createAiKey(workspace, input),
    onSuccess: () => {
      toast.success("Key added");
      refresh();
    },
    onError: () => toast.error("Could not add the key."),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteAiKey(workspace, id),
    onSuccess: () => {
      toast.success("Key removed");
      refresh();
    },
    onError: () => toast.error("Could not remove the key."),
  });
  const test = useMutation({
    mutationFn: (id: string) => testAiKey(workspace, id),
    onSuccess: () => {
      toast.success("Key checked");
      refresh();
    },
    onError: () => toast.error("The key failed its check."),
  });

  const keys = view.data?.keys ?? [];
  const providers = view.data?.providers ?? [];

  return (
    <SettingsPage title="AI providers" description="Your own keys. Usage is billed by the provider, never by Dimmi.">
      <Section
        title="Keys"
        action={
          <AddKeyForm
            providers={providers}
            onAdd={(provider, name, apiKey, url) =>
              add.mutate({ provider, name, api_key: apiKey, base_url: url || undefined })
            }
          />
        }
      >
        {keys.length === 0 && <Empty>No keys yet</Empty>}
        {keys.map((k) => (
          <ProviderRow key={k.id} keyData={k} onTest={() => test.mutate(k.id)} onDelete={() => remove.mutate(k.id)} />
        ))}
      </Section>
    </SettingsPage>
  );
}

/*
 * AnswersSection carries the model, its connection and the answer
 * limits: the rows that only make sense once a key exists.
 */
function AnswersSection({ view, patch }: { view: AgentViewType; patch: (p: AgentPatch) => void }) {
  const { agent, catalog, keys } = view;
  const keyOptions = keys.map((k) => k.name);
  const keyByName = (name: string) => keys.find((k) => k.name === name);
  const answerKey = keys.find((k) => k.id === agent.answer_key_id) ?? keys[0];
  const entries = answerKey ? (catalog[answerKey.provider] ?? []) : [];
  const modelOptions = entries.map((m) => m.display_name);
  const modelByName = (name: string) => entries.find((m) => m.display_name === name);
  const answerModelName = entries.find((m) => m.id === agent.answer_model)?.display_name;
  return (
    <Section title="Answers">
      <Row label="Connection" description="Which provider key answers.">
        <Choice
          label="Connection"
          options={keyOptions.length ? keyOptions : ["Add a key first"]}
          value={answerKey?.name}
          disabled={!keyOptions.length}
          onChange={(v) => {
            const key = keyByName(v);
            if (key) {
              patch({ answer_key_id: key.id });
            }
          }}
        />
      </Row>
      <Row label="Model">
        <Choice
          label="Model"
          options={modelOptions.length ? modelOptions : ["Choose after adding a key"]}
          value={answerModelName}
          disabled={!modelOptions.length}
          onChange={(v) => {
            const model = modelByName(v);
            if (model) {
              patch({ answer_model: model.id });
            }
          }}
        />
      </Row>
      <Row label="Replies per conversation" description="Dimmi hands over after this many answers.">
        <Choice
          label="Replies per conversation"
          options={["3", "5", "10", "20"]}
          value={String(agent.max_ai_replies)}
          onChange={(v) => patch({ max_ai_replies: Number(v) })}
        />
      </Row>
      <Row label="When it doesn't know">
        <Choice
          label="When it doesn't know"
          options={[...unknownOptions]}
          value={inverse(unknownValues, agent.unknown_behavior) ?? unknownOptions[0]}
          onChange={(v) => patch({ unknown_behavior: unknownValues[v as (typeof unknownOptions)[number]] })}
        />
      </Row>
      <Row label="Cite sources" description="Link the articles each answer comes from.">
        <Toggle label="Cite sources" on={agent.require_citations} onChange={(on) => patch({ require_citations: on })} />
      </Row>
    </Section>
  );
}
