/*
 * The person's own settings: their profile, how the app behaves for
 * them, what notifies them, how they sign in, and their API tokens.
 * Nothing here changes anything for teammates.
 */
import { RiGithubFill, RiGoogleFill, RiKey2Line, RiShieldKeyholeLine } from "@remixicon/react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { avatarColor } from "@/features/shell/avatar-color";

import { Action, Choice, Danger, Field, Toggle } from "../controls";
import { Empty, Glyph, Item, Logo, Row, Section, SettingsPage } from "../layout";
import { SessionsSection } from "../sessions";
import { TwoFactorSection } from "../two-factor";
import { useAccount } from "../use-account";

const languages = ["English", "Français", "Deutsch", "Español", "Português"];
const zones = ["Europe/Lisbon", "Europe/London", "America/New_York", "America/Los_Angeles", "Asia/Tokyo", "UTC"];

export function ProfileSettings() {
  const me = useAccount();

  return (
    <SettingsPage title="Profile" description="How teammates and customers see you.">
      <Section title="You">
        <Row label="Profile picture" description="PNG, JPEG, WebP or GIF, up to 2 MB.">
          <Avatar size="md" color={avatarColor(me.name)} initials={me.name.charAt(0)} />
          <Action>Upload</Action>
        </Row>
        <Row label="Full name">
          <Field label="Full name" value={me.name} />
        </Row>
        <Row label="Email" description="Changing it sends a link to the new address first.">
          <span className="text-body-2-regular text-text-secondary">{me.email}</span>
          <Action>Change</Action>
        </Row>
      </Section>
      <Section title="Workspace access">
        <Row label={`Leave ${me.workspace.name}`} description="You lose access to its conversations straight away.">
          <Danger>Leave workspace</Danger>
        </Row>
      </Section>
      <Section title="Delete account" description="Removes you from every workspace, after you confirm by email.">
        <Row label="Delete your account" description="Workspaces you alone own must be handed over or deleted first.">
          <Danger>Delete account</Danger>
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function PreferencesSettings() {
  return (
    <SettingsPage title="Preferences" description="How Dimmi looks and behaves for you.">
      <Section title="Language and region">
        <Row label="Language" description="Falls back to the workspace's language.">
          <Choice label="Language" options={languages} />
        </Row>
        <Row label="Time zone" description="Used for times in the inbox and in notifications.">
          <Choice label="Time zone" options={zones} />
        </Row>
      </Section>
      <Section title="Interface">
        <Row label="Theme" description="Follow your system, or choose light or dark.">
          <Choice label="Theme" options={["System", "Light", "Dark"]} />
        </Row>
      </Section>
      <Section title="Inbox">
        <Row label="Available" description="When the last teammate goes away, the messenger shows your team offline.">
          <Toggle label="Available" on />
        </Row>
        <Row label="Send with Enter" description="Use Enter to send and Shift+Enter for a new line.">
          <Toggle label="Send with Enter" on />
        </Row>
      </Section>
      <Section title="Email signature" description="Added under the replies you send by email.">
        <Row label="Use a signature">
          <Toggle label="Use a signature" on />
        </Row>
        <Row label="Signature" description="Links, bold and italic, up to 2,000 characters.">
          <Field label="Signature" placeholder="Ada · Support at Acme" width="w-64" />
        </Row>
      </Section>
    </SettingsPage>
  );
}

const triggers: [string, boolean[]][] = [
  ["New conversation", [true, true, true, true]],
  ["Assigned to you", [true, true, true, true]],
  ["Customer replied", [true, true, true, true]],
  ["Mentioned", [true, true, true, true]],
  ["Dimmi hands over", [true, true, true, true]],
  ["Snooze ended", [true, true, false, false]],
  ["Negative rating", [true, true, false, false]],
  ["Action needs approval", [true, true, true, false]],
  ["New feedback post", [true, false, false, true]],
];
const ways = ["In app", "Push", "Email", "Slack"];

function Matrix() {
  return (
    <table className="w-full table-fixed">
      <caption className="sr-only">Where each notification reaches you</caption>
      <thead>
        <tr>
          <th scope="col" className="py-2.5 text-start text-body-2-medium text-text-secondary">
            Notify me about
          </th>
          {ways.map((way) => (
            <th
              key={way}
              scope="col"
              className="w-12 py-2.5 text-center text-body-2-medium text-text-secondary sm:w-16"
            >
              {way}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {triggers.map(([trigger, on]) => (
          <tr key={trigger} className="border-t border-separator-border">
            <th scope="row" className="py-2.5 text-start text-body-regular text-text-primary">
              {trigger}
            </th>
            {ways.map((way, index) => (
              <td key={way} className="py-2.5">
                <span className="flex justify-center">
                  <Checkbox size="sm" aria-label={`${trigger} by ${way}`} defaultSelected={on[index]} />
                </span>
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function NotificationsSettings() {
  return (
    <SettingsPage title="Notifications" description="Choose what reaches you, and where.">
      <Section title="Channels" description="Everything always lands in the app; pick where else it follows you.">
        <Matrix />
      </Section>
      <Section title="Quiet hours" description="Hold push and mail overnight; a summary arrives when they end.">
        <Row label="Quiet hours">
          <Toggle label="Quiet hours" />
        </Row>
        <Row label="From" description="Every day of the week.">
          <Choice label="Quiet from" options={["22:00", "21:00", "23:00", "00:00"]} />
          <Choice label="Quiet until" options={["08:00", "07:00", "09:00"]} />
        </Row>
        <Row label="Summary when quiet hours end">
          <Toggle label="Summary when quiet hours end" on />
        </Row>
      </Section>
      <Section title="More">
        <Row label="Message previews" description="Show what the customer wrote in push and mail.">
          <Toggle label="Message previews" on />
        </Row>
        <Row label="While Dimmi is answering" description="Notify even when Dimmi is handling the conversation.">
          <Toggle label="While Dimmi is answering" />
        </Row>
        <Row label="Email delay" description="Wait before mailing, in case you read it in the app first.">
          <Choice label="Email delay" options={["2 minutes", "Immediately", "5 minutes", "15 minutes"]} />
        </Row>
      </Section>
      <Section title="Devices" description="Phones and browsers that get push notifications.">
        <Empty action={<Action>Enable on this browser</Action>}>No devices yet</Empty>
      </Section>
      <Section title="Test">
        <Row label="Send a test notification" description="Arrives everywhere you have switched on.">
          <Action>Send test</Action>
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function SecuritySettings() {
  const me = useAccount();

  return (
    <SettingsPage title="Security & access" description="How you sign in, and where you are signed in.">
      <Section title="Password">
        <Row
          label="Password"
          description={
            me.hasPassword ? "Changing it signs you out everywhere else." : "You sign in with a link or a provider."
          }
        >
          <Action>{me.hasPassword ? "Change password" : "Set a password"}</Action>
        </Row>
      </Section>
      <TwoFactorSection hasPassword={me.hasPassword} />
      <Section title="Sign-in methods">
        <Item mark={<Logo icon={RiGoogleFill} brand="#4285F4" />} title="Google" detail="Not linked">
          <Action>Link</Action>
        </Item>
        <Item mark={<Logo icon={RiGithubFill} brand="#181717" />} title="GitHub" detail="Not linked">
          <Action>Link</Action>
        </Item>
      </Section>
      <SessionsSection hasPassword={me.hasPassword} />
      <Section title="Security log" description="Sign-ins and changes to your account.">
        <Item
          mark={<Glyph icon={RiShieldKeyholeLine} />}
          title="Signed in with a password"
          detail="Today · Chrome on macOS"
        />
      </Section>
    </SettingsPage>
  );
}

export function TokensSettings() {
  return (
    <SettingsPage title="API tokens" description="Tokens act as you, with only the abilities you give them.">
      <Section
        title="Your tokens"
        description="The secret shows once. Up to 50 tokens."
        action={<Action>New token</Action>}
      >
        <Item
          mark={<Glyph icon={RiKey2Line} />}
          title="Coding agent"
          detail="dmi_7Hk… · Conversations, articles · Last used yesterday · Never expires"
        >
          <Action>Revoke</Action>
        </Item>
      </Section>
    </SettingsPage>
  );
}
