/*
 * The actions the agent may take in the workspace's own systems, such as
 * looking up an order. Calling them is not built yet, so the page shows
 * how they will be laid out.
 */
import { RiFlashlightLine } from "@remixicon/react";

import { Action, Toggle } from "@/features/settings/controls";
import { Empty, Glyph, Item, Row, Section, SettingsPage } from "@/features/settings/layout";

export function ActionsSettings() {
  return (
    <SettingsPage
      title="Actions"
      description="Let dimmi look things up or change them in your systems, such as an order or a subscription."
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
