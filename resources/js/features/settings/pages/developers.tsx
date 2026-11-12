/*
 * The developer settings: the agents and apps signed in to the
 * workspace, and GitHub. Webhooks and the secrets that verify signed-in
 * messenger visitors have pages of their own.
 */
import { RiClaudeFill, RiGithubFill, RiTerminalBoxLine } from "@remixicon/react";

import { Action, Toggle } from "../controls";
import { Empty, Glyph, Item, Logo, Row, Section, SettingsPage } from "../layout";

export { WebhooksSettings } from "./webhooks";

export function AppsSettings() {
  return (
    <SettingsPage
      title="Authorized apps"
      description="Agents, the CLI and other apps people let act in dimmi with their account."
    >
      <Section title="Your apps">
        <Item
          mark={<Glyph icon={RiTerminalBoxLine} />}
          title="dimmi CLI"
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
