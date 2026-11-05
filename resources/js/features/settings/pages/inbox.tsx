/*
 * How conversations are organised: the tags teammates put on them and
 * the replies the team saves for reuse.
 */
import { RiChatQuoteLine } from "@remixicon/react";

import { Chip } from "@/components/base/badges/chip";

import { Action } from "../controls";
import { Glyph, Item, Row, Section, SettingsPage } from "../layout";

const tags: [string, "blue" | "rose" | "lime" | "yellow" | "purple", number][] = [
  ["Bug", "rose", 42],
  ["Billing", "yellow", 18],
  ["Sign-in", "blue", 11],
  ["Feature request", "purple", 7],
  ["VIP", "lime", 3],
];

export function TagsSettings() {
  return (
    <SettingsPage title="Tags" description="Any teammate can create tags; admins rename, recolour and delete them.">
      <Section title="Tags" action={<Action>New tag</Action>}>
        {tags.map(([name, color, count]) => (
          <Row
            key={name}
            label={
              <Chip variant="caption" color={color}>
                {name}
              </Chip>
            }
          >
            <span className="text-body-2-regular text-text-tertiary">{count} conversations</span>
            <Action>Edit</Action>
          </Row>
        ))}
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
