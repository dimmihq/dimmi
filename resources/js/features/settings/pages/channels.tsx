/*
 * The places customers write from: each messenger, on a page of its own
 * reached from the list of messengers, and
 * connections to the chat apps they already use; email has its own page. Each
 * channel decides which inbox its conversations land in.
 */
import { RiSearchLine } from "@remixicon/react";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import { chatApps } from "../chat-apps";
import { Action } from "../controls";
import { Empty, Item, Logo, Section, SettingsPage } from "../layout";

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
