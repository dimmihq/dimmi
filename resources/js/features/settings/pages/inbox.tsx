/*
 * How conversations are organised: the inboxes they land in and the
 * replies the team saves for reuse.
 */
import { RiChatQuoteLine, RiInbox2Line } from "@remixicon/react";

import { Chip } from "@/components/base/badges/chip";

import { Action, Choice, Colour, Toggle } from "../controls";
import { Glyph, Item, Row, Section, SettingsPage } from "../layout";

export function InboxesSettings() {
  return (
    <SettingsPage
      title="Inboxes"
      description="Every conversation lands in one inbox, picked by the channel it came from."
    >
      <Section title="Inboxes" action={<Action>New inbox</Action>}>
        <Item mark={<Glyph icon={RiInbox2Line} />} title="Support" detail="Team Support · Messenger, email">
          <Chip variant="caption" color="neutral">
            Default
          </Chip>
          <Action>Edit</Action>
        </Item>
        <Item mark={<Glyph icon={RiInbox2Line} />} title="Billing" detail="Team Billing · billing@ address">
          <Action>Edit</Action>
        </Item>
      </Section>
      <Section title="Support inbox">
        <Row label="Colour" description="Marks its conversations in lists.">
          <Colour label="Support inbox colour" value="#3b82f6" />
        </Row>
        <Row label="Reopen window" description="A customer reply within this time reopens a closed conversation.">
          <Choice label="Reopen window" options={["30 days", "7 days", "90 days", "Never reopen"]} />
        </Row>
        <Row
          label="Assign to whoever replies"
          description="The first teammate to answer an unassigned conversation takes it."
        >
          <Toggle label="Assign to whoever replies" on />
        </Row>
      </Section>
      <Section title="Messenger">
        <Row label="Name in seen receipts" description="Show customers which teammate read their message.">
          <Toggle label="Name in seen receipts" />
        </Row>
      </Section>
    </SettingsPage>
  );
}

const replies = [
  ["Send a sign-in code", "/code"],
  ["Ask for the mail app", "/mailapp"],
  ["Closing the loop", "/close"],
];

export function SavedRepliesSettings() {
  return (
    <SettingsPage
      title="Saved replies"
      description="Shared with the whole team. Use {{contact.first_name}} and other variables."
    >
      <Section title="Team replies" action={<Action>New reply</Action>}>
        {replies.map(([title, shortcut]) => (
          <Item key={title} mark={<Glyph icon={RiChatQuoteLine} />} title={title} detail={`Shortcut ${shortcut}`}>
            <Action>Edit</Action>
          </Item>
        ))}
      </Section>
    </SettingsPage>
  );
}
