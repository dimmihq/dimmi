/*
 * Running the workspace: its teams, the rules every member signs in
 * under, the audit log, moving data in and out, and billing on the
 * cloud. Admins and owners change these; some actions are for owners
 * only. The workspace's name and its members live in workspace.tsx.
 */
import { RiBankCardLine, RiDownload2Line, RiFileList3Line, RiTeamLine } from "@remixicon/react";

import { Action, Field, Toggle } from "../controls";
import { Glyph, Item, Row, Section, SettingsPage } from "../layout";

export function TeamsSettings() {
  return (
    <SettingsPage title="Teams" description="Assign conversations to a team, and let an inbox route to one.">
      <Section title="Teams" action={<Action>New team</Action>}>
        <Item mark={<Glyph icon={RiTeamLine} />} title="Support" detail="2 members · Support inbox">
          <Action>Edit</Action>
        </Item>
        <Item mark={<Glyph icon={RiTeamLine} />} title="Billing" detail="1 member · Billing inbox">
          <Action>Edit</Action>
        </Item>
      </Section>
    </SettingsPage>
  );
}

export function WorkspaceSecuritySettings() {
  return (
    <SettingsPage title="Security" description="Rules every member of the workspace signs in under.">
      <Section title="Authentication">
        <Row
          label="Require two-factor authentication"
          description="Members without it set it up at their next sign-in."
        >
          <Toggle label="Require two-factor authentication" />
        </Row>
      </Section>
    </SettingsPage>
  );
}

const events = [
  ["Sam Okafor changed Lea Martin's role to Member", "Today, 10:42 · Browser"],
  ["Lea Martin created the webhook api.acme.com/hooks/dimmi", "Yesterday · API token"],
  ["Sam Okafor invited noor@acme.com", "2 days ago · Browser"],
];

export function AuditLogSettings() {
  return (
    <SettingsPage title="Audit log" description="Kept for a year on cloud, for ever on self-host.">
      <Section title="Recent events" action={<Action>Export CSV</Action>}>
        {events.map(([title, detail]) => (
          <Item key={title} mark={<Glyph icon={RiFileList3Line} />} title={title} detail={detail} />
        ))}
      </Section>
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
    <SettingsPage title="Billing" description="Pay for what you use. Dimmi agent usage is billed by your AI provider.">
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
