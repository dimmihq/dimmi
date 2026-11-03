/*
 * The developer settings: webhooks to your own endpoints, the secrets
 * that verify signed-in messenger visitors, the agents and apps signed in
 * to the workspace, and GitHub. Identity tokens are always verified;
 * there is no switch to turn that off.
 */
import { RiClaudeFill, RiFingerprintLine, RiGithubFill, RiTerminalBoxLine, RiWebhookLine } from "@remixicon/react";

import { Chip } from "@/components/base/badges/chip";

import { Action, Toggle } from "../controls";
import { Empty, Glyph, Item, Logo, Row, Section, SettingsPage } from "../layout";

export function WebhooksSettings() {
  return (
    <SettingsPage title="Webhooks" description="Signed POST requests to your endpoints when things happen. Up to 20.">
      <Section title="Endpoints" action={<Action>New endpoint</Action>}>
        <Item
          mark={<Glyph icon={RiWebhookLine} />}
          title="https://api.acme.com/hooks/dimmi"
          detail="conversation.* · Last delivery 3 minutes ago"
        >
          <Chip variant="caption" color="lime">
            Healthy
          </Chip>
          <Action>Edit</Action>
        </Item>
      </Section>
    </SettingsPage>
  );
}

export function IdentitySettings() {
  return (
    <SettingsPage
      title="Identity verification"
      description="Your server signs a token for each signed-in user, so nobody can pose as one of your customers."
    >
      <Section
        title="Secrets"
        description="Up to 5 active. A rotated secret keeps working for 7 days."
        action={<Action>New secret</Action>}
      >
        <Item mark={<Glyph icon={RiFingerprintLine} />} title="Production" detail="HS256 · 3f9c…a21b · Last used today">
          <Action>Rotate</Action>
        </Item>
      </Section>
      <Section title="Sign tokens on your server">
        <Row label="Code samples" description="Node, Python, Ruby, Go and PHP.">
          <Action>Show snippets</Action>
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function AppsSettings() {
  return (
    <SettingsPage
      title="Authorized apps"
      description="Agents, the CLI and other apps people let act in Dimmi with their account."
    >
      <Section title="Your apps">
        <Item
          mark={<Glyph icon={RiTerminalBoxLine} />}
          title="Dimmi CLI"
          detail="Conversations, articles · This Mac · Used today"
        >
          <Action>Revoke</Action>
        </Item>
        <Item
          mark={<Logo icon={RiClaudeFill} brand="#D97757" />}
          title="Claude"
          detail="Conversations · Used yesterday"
        >
          <Action>Revoke</Action>
        </Item>
      </Section>
      <Section title="Who may connect" description="Apps that register themselves when someone signs in.">
        <Row label="Self-registering apps" description="Most AI tools connect this way.">
          <Toggle label="Self-registering apps" on />
        </Row>
        <Row label="Apps with a published client document">
          <Toggle label="Apps with a published client document" on />
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function GithubSettings() {
  return (
    <SettingsPage title="GitHub" description="For docs sync, knowledge from repositories and release notes.">
      <Section
        title="Access tokens"
        description="A fine-grained GitHub token with read access."
        action={<Action>Add token</Action>}
      >
        <Empty>No connection yet</Empty>
      </Section>
      <Section title="Repositories" action={<Action>Add repository</Action>}>
        <Item mark={<Logo icon={RiGithubFill} brand="#181717" />} title="acme/handbook" detail="Used by Knowledge">
          <Action>Check</Action>
        </Item>
      </Section>
    </SettingsPage>
  );
}
