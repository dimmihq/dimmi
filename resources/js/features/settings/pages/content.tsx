/*
 * What the workspace publishes and learns from: help centers, the
 * knowledge Dimmi reads, the changelog, the feedback portal, and how
 * often announcements and surveys may interrupt customers.
 */
import { Chip } from "@/components/base/badges/chip";

import { Action, Choice, Field, Toggle } from "../controls";
import { FeedbackStatusesSection } from "../feedback-statuses";
import { Row, Section, SettingsPage } from "../layout";

export { KnowledgeSettings } from "@/features/knowledge/source-list";
export { HelpCentersSettings } from "../help-center-settings";

export function ChangelogSettings() {
  return (
    <SettingsPage title="Changelog" description="Release notes on your help center and in subscribers' inboxes.">
      <Section title="Changelog">
        <Row label="On" description="Show the changelog on your help center.">
          <Toggle label="Changelog on" on />
        </Row>
        <Row label="Title">
          <Field label="Changelog title" value="Changelog" />
        </Row>
        <Row label="Show authors">
          <Toggle label="Show authors" on />
        </Row>
        <Row label="Reactions">
          <Toggle label="Reactions" on />
        </Row>
      </Section>
      <Section title="Labels" action={<Action>New label</Action>}>
        <Row
          label={
            <Chip variant="caption" color="lime">
              New
            </Chip>
          }
        />
        <Row
          label={
            <Chip variant="caption" color="blue">
              Improved
            </Chip>
          }
        />
        <Row
          label={
            <Chip variant="caption" color="rose">
              Fixed
            </Chip>
          }
        />
      </Section>
      <Section title="Email">
        <Row label="Subscriptions" description="Readers can subscribe to new entries by email.">
          <Toggle label="Subscriptions" on />
        </Row>
        <Row label="What the email holds">
          <Choice label="What the email holds" options={["The full entry", "A summary"]} />
        </Row>
        <Row label="Undo window" description="Time to take an entry back before it is mailed.">
          <Choice label="Undo window" options={["5 minutes", "None", "15 minutes", "1 hour"]} />
        </Row>
      </Section>
      <Section title="From GitHub releases" action={<Action>Add repository</Action>}>
        <Row label="New releases" description="What a published release becomes.">
          <Choice label="New releases" options={["A draft written by AI", "A plain draft", "Nothing"]} />
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function FeedbackSettings() {
  return (
    <SettingsPage title="Feedback" description="A portal where customers post, vote and follow your roadmap.">
      <Section title="Portal">
        <Row label="On">
          <Toggle label="Portal on" />
        </Row>
        <Row label="Who can see it">
          <Choice label="Who can see it" options={["Anyone", "Signed-in customers"]} />
        </Row>
        <Row label="Anonymous votes">
          <Toggle label="Anonymous votes" />
        </Row>
        <Row label="Show vote counts">
          <Toggle label="Show vote counts" on />
        </Row>
        <Row label="Author names">
          <Choice label="Author names" options={["First name", "Full name", "Anonymous"]} />
        </Row>
      </Section>
      <Section title="Boards" action={<Action>New board</Action>}>
        <Row label="Feature requests" description="Public · Open to everyone">
          <Action>Edit</Action>
        </Row>
        <Row label="Bugs" description="Public · Approval for new contacts">
          <Action>Edit</Action>
        </Row>
      </Section>
      <FeedbackStatusesSection action={<Action>New status</Action>} />
      <Section title="AI">
        <Row label="Spot duplicates" description="Suggest merging posts that ask for the same thing.">
          <Toggle label="Spot duplicates" on />
        </Row>
        <Row label="Capture from conversations" description="Suggest posts from what customers ask for in the inbox.">
          <Toggle label="Capture from conversations" />
        </Row>
      </Section>
    </SettingsPage>
  );
}

export function EngageSettings() {
  return (
    <SettingsPage title="Announcements & surveys" description="Keep customers informed without crowding them.">
      <Section title="Frequency" description="Banners and posts that interrupt count toward these limits.">
        <Row label="Interruptions per day" description="The most any one customer sees in a day.">
          <Choice label="Interruptions per day" options={["2", "1", "3", "5"]} />
        </Row>
        <Row label="Time between interruptions">
          <Choice label="Time between interruptions" options={["30 minutes", "10 minutes", "1 hour", "4 hours"]} />
        </Row>
        <Row label="Days between surveys">
          <Choice label="Days between surveys" options={["14 days", "7 days", "30 days"]} />
        </Row>
        <Row label="Quiet while chatting" description="Hold announcements while the messenger is open.">
          <Toggle label="Quiet while chatting" on />
        </Row>
      </Section>
      <Section title="Rating after a conversation" description="Ask customers how it went once a conversation closes.">
        <Row label="Ask for a rating">
          <Toggle label="Ask for a rating" />
        </Row>
        <Row label="Ask after">
          <Choice label="Ask after" options={["10 minutes", "Straight away", "1 hour", "1 day"]} />
        </Row>
        <Row label="Include conversations dimmi solved">
          <Toggle label="Include conversations dimmi solved" on />
        </Row>
      </Section>
    </SettingsPage>
  );
}
