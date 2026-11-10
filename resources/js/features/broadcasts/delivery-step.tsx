/*
 * The delivery step of a broadcast: who it comes from and, for email,
 * the address it leaves from; where replies land and whether the agent
 * answers them; when it leaves; and what counts as it working. A send at
 * a set time can go out in each person's own time zone. A Slack post
 * comes from the dimmi app, or from the teammate's own Slack account
 * when they connected it.
 */
import { useQuery } from "@tanstack/react-query";

import { DatePicker } from "@/components/base/date-picker/date-picker";
import { TimeField } from "@/components/base/time-field/time-field";
import { Row, Section } from "@/features/settings/layout";
import type { Broadcast } from "@/lib/api/broadcasts";
import { workspaceQuery } from "@/lib/api/workspace";

import { Flag, Pick } from "./controls";
import type { Draft } from "./draft";
import { instantOf, wallOf, zoneFor } from "./schedule";
import { useActiveMembers, useEmailAddresses, useEvents, useTeams } from "./sources";

type Change = (change: Partial<Draft>) => void;

interface StepProps {
  workspace: string;
  broadcast: Broadcast;
  draft: Draft;
  onChange: Change;
}

function SenderSection({ workspace, broadcast, draft, onChange }: StepProps) {
  const members = useActiveMembers(workspace).map((member) => ({ id: member.id, name: member.user.name }));
  const addresses = useEmailAddresses(workspace, broadcast.channel === "email").map((address) => ({
    id: address.id,
    name: address.display_name ? `${address.display_name} <${address.address}>` : address.address,
  }));

  return (
    <Section title="From">
      <Row label="Sent as" description="Their name and photo show on the message.">
        <Pick
          label="Sent as"
          options={members}
          value={draft.sender_id}
          onChange={(sender_id) => onChange({ sender_id })}
        />
      </Row>
      {broadcast.channel === "email" ? (
        <Row label="Send from" description="A connected address on a verified domain. Replies come back to it.">
          <Pick
            label="Send from"
            options={addresses}
            value={draft.email_address_id}
            placeholder="Choose an address"
            onChange={(email_address_id) => onChange({ email_address_id })}
          />
        </Row>
      ) : null}
      {broadcast.channel === "slack" ? (
        <Row label="Post as" description="Posting as yourself needs your own Slack account connected.">
          <Pick
            label="Post as"
            options={[
              { id: "bot", name: "The dimmi app" },
              { id: "member", name: "The teammate it comes from" },
            ]}
            value={draft.slack_as}
            onChange={(as) => onChange({ slack_as: as === "member" ? "member" : "bot" })}
          />
        </Row>
      ) : null}
    </Section>
  );
}

function RepliesSection({ workspace, draft, onChange }: StepProps) {
  const teams = useTeams(workspace).map((team) => ({ id: team.id, name: team.name }));
  const members = useActiveMembers(workspace).map((member) => ({ id: member.id, name: member.user.name }));

  return (
    <Section title="Replies" description="Each reply opens a conversation in the inbox.">
      <Row label="Team">
        <Pick
          label="Replies go to team"
          options={teams}
          value={draft.reply_team_id}
          placeholder="No team"
          onChange={(reply_team_id) => onChange({ reply_team_id })}
        />
      </Row>
      <Row label="Assigned to">
        <Pick
          label="Replies assigned to"
          options={members}
          value={draft.reply_member_id}
          placeholder="Nobody"
          onChange={(reply_member_id) => onChange({ reply_member_id })}
        />
      </Row>
      <Row label="The agent answers replies" description="Off: every reply waits for a teammate.">
        <Flag
          label="The agent answers replies"
          on={draft.ai_replies}
          onChange={(ai_replies) => onChange({ ai_replies })}
        />
      </Row>
    </Section>
  );
}

function WhenSection({ workspace, broadcast, draft, onChange }: StepProps) {
  const workspaceZone = useQuery(workspaceQuery(workspace)).data?.timezone;
  const later = draft.schedule_kind !== "now";
  const zone = zoneFor(draft.schedule_kind, workspaceZone);
  const wall = wallOf(draft.scheduled_at, zone);
  const kindOf = (local: boolean) => (local ? "local" : "at");

  return (
    <Section title="When">
      <Row label="Send">
        <Pick
          label="Send"
          options={[
            { id: "now", name: "Right away" },
            { id: "later", name: "At a set time" },
          ]}
          value={later ? "later" : "now"}
          onChange={(id) =>
            onChange(
              id === "later"
                ? { schedule_kind: "at", scheduled_at: instantOf(wall, zoneFor("at", workspaceZone)) }
                : { schedule_kind: "now", scheduled_at: null },
            )
          }
        />
      </Row>
      {later ? (
        <Row label="On">
          <DatePicker
            aria-label="Send date"
            value={wall.day}
            onChange={(day) => day && onChange({ scheduled_at: instantOf({ ...wall, day }, zone) })}
          />
          <TimeField
            aria-label="Send time"
            size="small"
            value={wall.time}
            onChange={(time) => time && onChange({ scheduled_at: instantOf({ ...wall, time }, zone) })}
            className="w-32"
          />
        </Row>
      ) : null}
      {later && broadcast.channel !== "slack" ? (
        <Row label="In each person's time zone" description="People without one get it in the workspace's.">
          <Flag
            label="In each person's time zone"
            on={draft.schedule_kind === "local"}
            onChange={(local) =>
              onChange({
                schedule_kind: kindOf(local),
                scheduled_at: instantOf(wall, zoneFor(kindOf(local), workspaceZone)),
              })
            }
          />
        </Row>
      ) : null}
      {broadcast.channel === "slack" ? null : (
        <Row label="Ignore the frequency cap" description="Reaches people who got another message recently.">
          <Flag
            label="Ignore the frequency cap"
            on={draft.skip_frequency_cap}
            onChange={(skip_frequency_cap) => onChange({ skip_frequency_cap })}
          />
        </Row>
      )}
      {broadcast.channel === "email" ? (
        <Row label="Track clicks" description="Links go through dimmi so the report counts them.">
          <Flag label="Track clicks" on={draft.track_clicks} onChange={(track_clicks) => onChange({ track_clicks })} />
        </Row>
      ) : null}
    </Section>
  );
}

const windows = [1, 3, 7, 14, 30].map((days) => ({ id: String(days), name: days === 1 ? "1 day" : `${days} days` }));

function GoalSection({ workspace, draft, onChange }: StepProps) {
  const events = useEvents(workspace).map((event) => ({ id: event.name, name: event.label || event.name }));
  const days = draft.goal?.days ?? 7;

  return (
    <Section title="Goal" description="What counts as the broadcast working. The report shows who got there.">
      <Row label="Counts when a person">
        <Pick
          label="Goal"
          options={events}
          value={draft.goal?.event ?? null}
          placeholder="No goal"
          onChange={(event) => onChange({ goal: event ? { event, days } : null })}
        />
      </Row>
      {draft.goal ? (
        <Row label="Within">
          <Pick
            label="Goal window"
            options={windows}
            value={String(days)}
            onChange={(id) => draft.goal && onChange({ goal: { ...draft.goal, days: Number(id ?? 7) } })}
          />
        </Row>
      ) : null}
    </Section>
  );
}

export function DeliveryStep(props: StepProps) {
  return (
    <div className="flex flex-col gap-8">
      <SenderSection {...props} />
      <RepliesSection {...props} />
      <WhenSection {...props} />
      <GoalSection {...props} />
    </div>
  );
}
