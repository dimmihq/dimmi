/*
 * The replies the team saves for reuse. Tags and inboxes have pages of
 * their own.
 */
import { RiChatQuoteLine } from "@remixicon/react";

import { Action } from "../controls";
import { Glyph, Item, Section, SettingsPage } from "../layout";

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
