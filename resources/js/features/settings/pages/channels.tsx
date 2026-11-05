/*
 * The places customers write from: each messenger, on a page of its own
 * reached from the list of messengers, and
 * connections to the chat apps they already use; email has its own page. Each
 * channel decides which inbox its conversations land in.
 */
import { RiSearchLine } from "@remixicon/react";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import type { WidgetSummary } from "@/lib/api/widget-settings";
import { chatApps } from "../chat-apps";
import { Action, Choice, Colour, Danger, Field, Toggle } from "../controls";
import { Empty, Item, Logo, Row, Section, SettingsPage } from "../layout";
import { BackToMessengers } from "./messengers";

export function MessengerSettings({ widget }: { widget: WidgetSummary }) {
  return (
    <SettingsPage title={widget.name} description={`A messenger whose conversations land in ${widget.inbox.name}.`}>
      <BackToMessengers />
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
          <Field label="Messenger name" value={widget.name} />
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

export function ConnectionsSettings() {
  const [query, setQuery] = useState("");
  const needle = query.trim().toLocaleLowerCase();
  const shown = chatApps.filter((service) => `${service.name} ${service.detail}`.toLocaleLowerCase().includes(needle));

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
