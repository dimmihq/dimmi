/*
 * The settings of a step that tells the customer when to expect a reply:
 * the message, whose {{reply_time}} becomes how long the wait looks from
 * the office hours and how many conversations are waiting, the office
 * hours to count, and the team whose load counts.
 */
import { Row, Section } from "@/features/settings/layout";

import { type EditorProps, Words } from "./field-parts";
import { Pick } from "./pick";
import { replyTimeText, text, useSet } from "./step-settings";

const inboxHours = "inbox";
const everyone = "workspace";

export function ReplyTimeFields({ step, lists, onEdit }: EditorProps) {
  const set = useSet(onEdit);

  return (
    <>
      <Section
        title="Message"
        description="{{reply_time}} becomes when to expect a reply, such as “in a few minutes”, “within a day” or “on Monday after 09:00”."
      >
        <Words
          label="Reply time message"
          value={text(step.settings, "text")}
          placeholder={replyTimeText}
          onChange={(value) => set({ text: value.trim() ? value : undefined })}
        />
      </Section>
      <Section title="How it is worked out">
        <Row label="Office hours" description="Outside them, it names when they open.">
          <Pick
            key={`schedules-${lists.schedules.length}`}
            label="Office hours"
            choices={[[inboxHours, "The inbox's office hours"], ...lists.schedules]}
            value={text(step.settings, "schedule_id") || inboxHours}
            onChange={(id) => set({ schedule_id: id === inboxHours ? undefined : id })}
          />
        </Row>
        <Row label="Count the load of" description="Busier teams reply later.">
          <Pick
            key={`teams-${lists.teams.length}`}
            label="Count the load of"
            choices={[[everyone, "Whole workspace"], ...lists.teams]}
            value={text(step.settings, "team_id") || everyone}
            onChange={(id) => set({ team_id: id === everyone ? undefined : id })}
          />
        </Row>
      </Section>
    </>
  );
}
