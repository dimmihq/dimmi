/*
 * The AI settings: how Dimmi answers customers and when it hands over,
 * the actions it may take in your systems, and the model keys it and
 * Operator run on. The workspace pays the provider directly with its own
 * keys, so the spend cap here is the workspace's own guard.
 */
import { RiAnthropicFill, RiFlashlightLine, RiLink, RiOpenaiFill } from "@remixicon/react";

import { Chip } from "@/components/base/badges/chip";

import { Action, Choice, Field, Toggle } from "../controls";
import { Empty, Glyph, Item, Logo, Row, Section, SettingsPage } from "../layout";

export function AgentSettings() {
  return (
    <SettingsPage
      title="Dimmi agent"
      description="Dimmi answers customers from your knowledge and hands over when it should."
    >
      <Section title="Status" description="Dimmi switches on once a model key, knowledge and a channel are ready.">
        <Row label="Answer customers" description="2 of 4 steps done: add a model key, index your knowledge.">
          <Toggle label="Answer customers" />
        </Row>
      </Section>
      <Section title="Identity">
        <Row label="Name" description="How the agent introduces itself.">
          <Field label="Agent name" value="Dimmi" />
        </Row>
        <Row label="Tone">
          <Choice label="Tone" options={["Friendly", "Professional", "Concise"]} />
        </Row>
        <Row label="Guidance" description="Anything Dimmi should always know or do, up to 4,000 characters.">
          <Action>Edit guidance</Action>
        </Row>
      </Section>
      <Section title="When Dimmi answers">
        <Row label="Channels">
          <Choice label="Channels" options={["Messenger", "Messenger and email", "All channels"]} />
        </Row>
        <Row label="Hours">
          <Choice label="Hours" options={["Always", "Outside office hours", "During office hours"]} />
        </Row>
        <Row label="Once a teammate is assigned">
          <Choice label="Once a teammate is assigned" options={["Step back while they're active", "Never answer"]} />
        </Row>
      </Section>
      <Section title="Answers">
        <Row label="Model">
          <Choice label="Model" options={["Choose after adding a key"]} />
        </Row>
        <Row label="Replies per conversation" description="Dimmi hands over after this many answers.">
          <Choice label="Replies per conversation" options={["5", "3", "10", "20"]} />
        </Row>
        <Row label="When it doesn't know">
          <Choice label="When it doesn't know" options={["Offer a teammate", "Hand over straight away"]} />
        </Row>
        <Row label="Cite sources" description="Link the articles each answer comes from.">
          <Toggle label="Cite sources" on />
        </Row>
      </Section>
      <Section title="Privacy">
        <Row label="Customer details Dimmi sees">
          <Choice label="Customer details Dimmi sees" options={["Basic", "None", "Chosen attributes"]} />
        </Row>
        <Row
          label="Hide personal data"
          description="Remove emails, phone numbers and cards before they reach the model."
        >
          <Toggle label="Hide personal data" on />
        </Row>
      </Section>
      <Section title="Handover">
        <Row label="Hand over to">
          <Choice label="Hand over to" options={["Support", "Billing"]} />
        </Row>
        <Row label="Raise priority on handover">
          <Toggle label="Raise priority on handover" />
        </Row>
      </Section>
      <Section title="Spend" description="Alerts at 50%, 80% and 100% of the cap.">
        <Row label="Monthly spend cap" description="Dimmi pauses when the workspace reaches it.">
          <Field label="Monthly spend cap" value="$25" width="w-28" />
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

export function ProvidersSettings() {
  return (
    <SettingsPage title="AI providers" description="Your own keys. Usage is billed by the provider, never by Dimmi.">
      <Section title="Keys" action={<Action>Add key</Action>}>
        <Item
          mark={<Logo icon={RiAnthropicFill} brand="#CC785C" />}
          title="Anthropic"
          detail="sk-ant-…4f2a · 6 models · Not used yet"
        >
          <Chip variant="caption" color="lime">
            Valid
          </Chip>
          <Action>Edit</Action>
        </Item>
        <Item
          mark={<Logo icon={RiOpenaiFill} brand="#000000" />}
          title="OpenAI"
          detail="sk-…9c1d · 12 models · Used today"
        >
          <Chip variant="caption" color="lime">
            Valid
          </Chip>
          <Action>Edit</Action>
        </Item>
        <Item mark={<Glyph icon={RiLink} />} title="OpenAI-compatible" detail="Ollama on gpu-1.internal · Not checked">
          <Action>Check</Action>
        </Item>
      </Section>
      <Section title="Knowledge search" description="The model that turns your docs into searchable meaning.">
        <Row label="Embedding key">
          <Choice label="Embedding key" options={["OpenAI", "OpenAI-compatible"]} />
        </Row>
      </Section>
    </SettingsPage>
  );
}
