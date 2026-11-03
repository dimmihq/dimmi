/*
 * The places customers write from: the messenger on your site, email
 * addresses, and connections to the chat apps they already use. Each
 * channel decides which inbox its conversations land in.
 */
import {
  RiDiscordFill,
  RiGlobalLine,
  RiMailLine,
  RiSearchLine,
  RiSlackFill,
  RiTelegramFill,
  RiWhatsappFill,
} from "@remixicon/react";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Input } from "@/components/base/input/input";

import { Action, Choice, Colour, Danger, Field, Toggle } from "../controls";
import { Empty, Glyph, Item, Logo, Row, Section, SettingsPage } from "../layout";

export function MessengerSettings() {
  return (
    <SettingsPage title="Messenger" description="The chat widget on your website, for the Website install.">
      <Section title="Install" description="Not seen on a site yet.">
        <Row label="Framework" description="We show the snippet for what your site is built with.">
          <Choice label="Framework" options={["Next.js", "React", "Plain HTML", "Vue", "Other"]} />
        </Row>
        <Row label="Install snippet" description="Paste it before the closing body tag, or run npx dimmi init.">
          <Action>Copy snippet</Action>
        </Row>
      </Section>
      <Section title="General">
        <Row label="Name">
          <Field label="Messenger name" value="Website" />
        </Row>
        <Row label="On" description="Turn off to hide the messenger everywhere at once.">
          <Toggle label="Messenger on" on />
        </Row>
        <Row label="Language" description="Auto follows each visitor's browser.">
          <Choice label="Messenger language" options={["Auto", "English", "Français", "Deutsch", "Español"]} />
        </Row>
      </Section>
      <Section title="Appearance">
        <Row label="Colour" description="The launcher, buttons and your messages.">
          <Colour label="Messenger colour" value="#10b981" />
        </Row>
        <Row label="Theme">
          <Choice label="Messenger theme" options={["Auto", "Light", "Dark"]} />
        </Row>
        <Row label="Position">
          <Choice label="Position" options={["Bottom right", "Bottom left"]} />
        </Row>
        <Row label="Launcher">
          <Choice label="Launcher" options={["Bubble", "Tab", "Hidden"]} />
        </Row>
      </Section>
      <Section title="Texts">
        <Row label="Greeting" description="Up to 60 characters.">
          <Field label="Greeting" value="Hi there 👋" />
        </Row>
        <Row label="Away message" description="Shown when nobody on the team is available.">
          <Field label="Away message" placeholder="We'll reply by email." />
        </Row>
      </Section>
      <Section title="Visitors">
        <Row label="Ask for an email" description="So you can reply after the visitor leaves.">
          <Choice label="Ask for an email" options={["When offline", "Always", "Never"]} />
        </Row>
        <Row label="Anonymous visitors" description="Let visitors write before they sign in to your product.">
          <Toggle label="Anonymous visitors" on />
        </Row>
        <Row label="Allowed sites" description="Empty allows any site.">
          <Field label="Allowed sites" placeholder="https://*.acme.com" />
        </Row>
      </Section>
      <Section title="Tabs">
        <Row label="Help" description="Search your help center inside the messenger.">
          <Toggle label="Help tab" />
        </Row>
        <Row label="What's new">
          <Toggle label="What's new tab" />
        </Row>
        <Row label="Feedback">
          <Toggle label="Feedback tab" />
        </Row>
      </Section>
      <Section title="Danger zone">
        <Row label="Delete this messenger" description="Sites using its snippet stop showing it.">
          <Danger>Delete messenger</Danger>
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function EmailSettings() {
  return (
    <SettingsPage title="Email" description="Forward your support address to Dimmi and reply from it.">
      <Section title="Addresses" action={<Action>Add address</Action>}>
        <Item mark={<Glyph icon={RiMailLine} />} title="support@acme.com" detail="Support inbox · Forwarding confirmed">
          <Chip variant="caption" color="neutral">
            Default
          </Chip>
          <Action>Edit</Action>
        </Item>
      </Section>
      <Section
        title="Sending domains"
        description="Verify your domain so replies come from it and reach inboxes."
        action={<Action>Add domain</Action>}
      >
        <Item mark={<Glyph icon={RiGlobalLine} />} title="acme.com" detail="DKIM and return path waiting for DNS">
          <Action>Check now</Action>
        </Item>
      </Section>
      <Section title="Replies">
        <Row label="Sender name" description="How your replies are signed in the customer's inbox.">
          <Choice label="Sender name" options={["Ada from Acme", "Acme"]} />
        </Row>
        <Row label="Quote the previous message">
          <Toggle label="Quote the previous message" on />
        </Row>
        <Row label="Bulk mail" description="Newsletters and auto-replies sent to your address.">
          <Choice label="Bulk mail" options={["Close", "Keep open", "Drop"]} />
        </Row>
      </Section>
      <Section title="Sending" description="Today 14 of 200 sent. Verify a domain to send up to 2,000 a day.">
        <Row label="Suppressed addresses" description="Addresses that bounced or complained are never mailed.">
          <Action>View list</Action>
        </Row>
      </Section>
    </SettingsPage>
  );
}

/*
 * The services a workspace can connect, each with its brand colour. The
 * list grows as Dimmi adds connections, so the page can be searched.
 */
const services = [
  { name: "Slack", icon: RiSlackFill, brand: "#4A154B", detail: "Shared channels and Slack Connect" },
  { name: "Discord", icon: RiDiscordFill, brand: "#5865F2", detail: "A support channel in your server" },
  { name: "Telegram", icon: RiTelegramFill, brand: "#26A5E4", detail: "Your bot's direct messages" },
  { name: "WhatsApp", icon: RiWhatsappFill, brand: "#25D366", detail: "Your business number" },
];

export function ConnectionsSettings() {
  const [query, setQuery] = useState("");
  const needle = query.trim().toLocaleLowerCase();
  const shown = services.filter((service) => `${service.name} ${service.detail}`.toLocaleLowerCase().includes(needle));

  return (
    <SettingsPage
      title="Connections"
      description="Answer customers in the apps they already use. Their messages arrive in your inbox."
    >
      <Input
        aria-label="Search connections"
        placeholder="Search connections"
        leadingIcon={RiSearchLine}
        value={query}
        onChange={setQuery}
      />
      <Section title="Chat apps">
        {shown.length === 0 ? <Empty>No connection matches “{query}”</Empty> : null}
        {shown.map((service) => (
          <Item
            key={service.name}
            mark={<Logo icon={service.icon} brand={service.brand} />}
            title={service.name}
            detail={service.detail}
          >
            <Action>Connect</Action>
          </Item>
        ))}
      </Section>
    </SettingsPage>
  );
}
