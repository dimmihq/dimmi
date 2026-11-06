/*
 * Running the workspace: the rules every member signs in under, moving
 * data in and out, and billing on the cloud. Admins and owners change
 * these; some actions are for owners only. The workspace's name and its
 * members live in workspace.tsx, its teams in teams.tsx and its audit
 * log in audit-log.tsx.
 */
import { RiBankCardLine, RiDownload2Line } from "@remixicon/react";

import { Action, Field } from "../controls";
import { Glyph, Item, Row, Section, SettingsPage } from "../layout";
import { RequireTwoFactorSection } from "../require-two-factor";
import { useAccount } from "../use-account";
import { IPAllowlistSections } from "../workspace-security/ip-allowlists";
import { SessionLengthSection } from "../workspace-security/session-length";
import { SignInRulesSection } from "../workspace-security/sign-in-rules";

export function WorkspaceSecuritySettings() {
  const me = useAccount();

  return (
    <SettingsPage title="Security" description="Rules every member of the workspace signs in under.">
      <SignInRulesSection />
      <SessionLengthSection />
      <IPAllowlistSections />
      <RequireTwoFactorSection workspace={me.workspace.id} hasPassword={me.hasPassword} title="Authentication" />
    </SettingsPage>
  );
}

const sources = ["Intercom", "Zendesk", "Help Scout", "Canny", "Featurebase", "Productlane", "A public site", "CSV"];

export function ImportSettings() {
  return (
    <SettingsPage title="Import & export" description="Bring your history with you, or take it elsewhere.">
      <Section title="Import from" description="Conversations, contacts, articles and feedback, with a preview first.">
        {sources.map((source) => (
          <Row key={source} label={source}>
            <Action>Start</Action>
          </Row>
        ))}
      </Section>
      <Section title="Export">
        <Row label="Export the workspace" description="Owners only. Every record and file, as JSON lines.">
          <Action>Start export</Action>
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function BillingSettings() {
  return (
    <SettingsPage title="Billing" description="Pay for what you use. dimmi agent usage is billed by your AI provider.">
      <Section title="This month">
        <Row label="Usage so far" description="1,284 conversations · 3 seats">
          <span className="text-body-2-medium text-text-primary">$18.40</span>
        </Row>
        <Row label="Spend cap" description="Alerts at 50%, 80% and 100%. Owners can raise it past $1,000.">
          <Field label="Spend cap" value="$50" width="w-28" />
        </Row>
      </Section>
      <Section title="Payment">
        <Item mark={<Glyph icon={RiBankCardLine} />} title="No card yet" detail="Add one to lift the free limits">
          <Action>Add card</Action>
        </Item>
        <Row label="Billing details" description="Name, address and tax ID on invoices.">
          <Action>Edit</Action>
        </Row>
      </Section>
      <Section title="Invoices">
        <Item mark={<Glyph icon={RiDownload2Line} />} title="October" detail="$0.00 · Paid" />
      </Section>
    </SettingsPage>
  );
}
