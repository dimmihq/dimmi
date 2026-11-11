/*
 * The settings of the whole series, on its entry step. Who enters: by
 * rules, by an event, or only by hand; whether people already matching
 * join when it goes live; whether someone can travel it again. The goal
 * and the rules and moments that take people out early. Then how its
 * messages go out: from which address and teammate, under which topic,
 * whether email keeps to the day in the person's zone, and where replies
 * land.
 */
import {
  useActiveMembers,
  useAudiences,
  useEmailAddresses,
  useEvents,
  useTeams,
  useTopics,
} from "@/features/broadcasts/sources";
import { Row, Section } from "@/features/settings/layout";
import type { SeriesSpec } from "@/lib/api/series";

import { Amount, Flag, Pick, Rules } from "./inputs";

/*
 * Sending is how a series' messages go out, kept on the series beside
 * its definition.
 */
export interface Sending {
  email_address_id?: string;
  sender_id?: string;
  reply_member_id?: string;
  reply_team_id?: string;
  topic_id?: string;
  service?: boolean;
  ai_replies?: boolean;
}

type Trigger = "rules" | "audience" | "event" | "manual";

function triggerOf(spec: SeriesSpec): Trigger {
  if (spec.entry.event) {
    return "event";
  }
  if (spec.entry.audience_id) {
    return "audience";
  }

  return spec.entry.rules ? "rules" : "manual";
}

export function EntrySettings({
  workspace,
  spec,
  sending,
  account,
  editable,
  onSpec,
  onSending,
}: {
  workspace: string;
  spec: SeriesSpec;
  sending: Sending;
  account: boolean;
  editable: boolean;
  onSpec: (spec: SeriesSpec) => void;
  onSending: (sending: Sending) => void;
}) {
  const events = useEvents(workspace);
  const audiences = useAudiences(workspace);
  const trigger = triggerOf(spec);
  const entry = (change: Partial<SeriesSpec["entry"]>) => onSpec({ ...spec, entry: { ...spec.entry, ...change } });
  const exit = (change: Partial<SeriesSpec["exit"]>) => onSpec({ ...spec, exit: { ...spec.exit, ...change } });
  const known = events.map((e): [string, string] => [e.name, e.label || e.name]);
  const named = [spec.entry.event, spec.goal?.event].filter((n): n is string => !!n && !known.some(([k]) => k === n));
  const eventOptions: [string, string][] = [...known, ...named.map((n): [string, string] => [n, n])];

  return (
    <>
      <Section
        title="Who enters"
        description={account ? "An account enters when anyone in it matches." : "People enter the moment they match."}
      >
        <Row label="Enters when">
          <Pick
            label="Enters when"
            value={trigger}
            options={[
              ["rules", "They match rules"],
              ["audience", "They join a saved audience"],
              ["event", "They do an event"],
              ["manual", "A teammate or the API adds them"],
            ]}
            onChange={(next) =>
              entry({
                rules: next === "rules" ? spec.entry.rules : undefined,
                audience_id: next === "audience" ? (spec.entry.audience_id ?? audiences[0]?.id) : undefined,
                event: next === "event" ? spec.entry.event || events[0]?.name || "signed_up" : undefined,
              })
            }
            disabled={!editable}
          />
        </Row>
        {trigger === "rules" ? (
          <Row label="Rules">
            <Rules
              workspace={workspace}
              title="Who enters this series"
              rules={spec.entry.rules}
              onChange={(rules) => entry({ rules })}
              disabled={!editable}
            />
          </Row>
        ) : null}
        {trigger === "audience" ? (
          <Row label="Audience">
            <Pick
              label="Audience"
              value={spec.entry.audience_id}
              options={audiences.map((a): [string, string] => [a.id, a.name])}
              onChange={(audience_id) => entry({ audience_id })}
              disabled={!editable}
            />
          </Row>
        ) : null}
        {trigger === "event" ? (
          <Row label="Event">
            <Pick
              label="Event"
              value={spec.entry.event}
              options={eventOptions}
              onChange={(event) => entry({ event })}
              disabled={!editable}
            />
          </Row>
        ) : null}
        {trigger === "rules" || trigger === "audience" ? (
          <Row label="People already matching" description="Off: only people who start matching after it goes live.">
            <Flag
              label="People already matching"
              on={spec.entry.include_existing}
              onChange={(include_existing) => entry({ include_existing })}
              disabled={!editable}
            />
          </Row>
        ) : null}
        <Row label="Can travel it again" description="After leaving, once the cooldown is over.">
          <Flag
            label="Can travel it again"
            on={spec.entry.reentry.kind === "again"}
            onChange={(on) =>
              entry({ reentry: { kind: on ? "again" : "once", cooldown_days: spec.entry.reentry.cooldown_days ?? 30 } })
            }
            disabled={!editable}
          />
        </Row>
        {spec.entry.reentry.kind === "again" ? (
          <Row label="Cooldown in days">
            <Amount
              label="Cooldown"
              value={spec.entry.reentry.cooldown_days}
              onChange={(cooldown_days) => entry({ reentry: { kind: "again", cooldown_days } })}
              disabled={!editable}
            />
          </Row>
        ) : null}
      </Section>
      <Section title="Goal" description="Counted when reached within the window, from when they entered.">
        <Row label="They do">
          <Pick
            label="Goal event"
            value={spec.goal?.event || "_none"}
            options={[["_none", "No goal"], ...eventOptions]}
            onChange={(event) =>
              onSpec({ ...spec, goal: event === "_none" ? null : { event, days: spec.goal?.days ?? 14 } })
            }
            disabled={!editable}
          />
        </Row>
        {spec.goal ? (
          <>
            <Row label="Within days">
              <Amount
                label="Goal window"
                value={spec.goal.days}
                onChange={(days) => onSpec({ ...spec, goal: { ...spec.goal, days } })}
                disabled={!editable}
              />
            </Row>
            <Row label="Leave once reached">
              <Flag
                label="Leave once reached"
                on={spec.exit.on_goal}
                onChange={(on_goal) => exit({ on_goal })}
                disabled={!editable}
              />
            </Row>
          </>
        ) : null}
      </Section>
      <Section title="Leave early" description="Wherever they are, at once. Nothing queued goes out after.">
        <Row label="They match">
          <Rules
            workspace={workspace}
            title="Who leaves this series"
            rules={spec.exit.rules}
            onChange={(rules) => exit({ rules })}
            disabled={!editable}
          />
        </Row>
        <Row label="They reply to a message">
          <Flag
            label="Leave on reply"
            on={spec.exit.on_reply}
            onChange={(on_reply) => exit({ on_reply })}
            disabled={!editable}
          />
        </Row>
        <Row label="They unsubscribe">
          <Flag
            label="Leave on unsubscribe"
            on={spec.exit.on_unsubscribe}
            onChange={(on_unsubscribe) => exit({ on_unsubscribe })}
            disabled={!editable}
          />
        </Row>
      </Section>
      <SendingSettings
        workspace={workspace}
        spec={spec}
        sending={sending}
        editable={editable}
        onSpec={onSpec}
        onSending={onSending}
      />
    </>
  );
}

function SendingSettings({
  workspace,
  spec,
  sending,
  editable,
  onSpec,
  onSending,
}: {
  workspace: string;
  spec: SeriesSpec;
  sending: Sending;
  editable: boolean;
  onSpec: (spec: SeriesSpec) => void;
  onSending: (sending: Sending) => void;
}) {
  const addresses = useEmailAddresses(workspace);
  const members = useActiveMembers(workspace);
  const teams = useTeams(workspace);
  const topics = useTopics(workspace);
  const set = (change: Partial<Sending>) => onSending({ ...sending, ...change });
  const memberOptions = members.map((m): [string, string] => [m.id, m.user.name || m.user.email]);

  return (
    <Section title="Sending" description="At most two series messages a person a day, across every series.">
      <Row label="Email from">
        <Pick
          label="Email from"
          value={sending.email_address_id}
          options={addresses.map((a): [string, string] => [a.id, a.address])}
          onChange={(email_address_id) => set({ email_address_id })}
          disabled={!editable}
        />
      </Row>
      <Row label="Sent by">
        <Pick
          label="Sent by"
          value={sending.sender_id}
          options={memberOptions}
          onChange={(sender_id) => set({ sender_id })}
          disabled={!editable}
        />
      </Row>
      <Row label="Replies go to">
        <Pick
          label="Replies go to"
          value={
            sending.reply_team_id
              ? `team:${sending.reply_team_id}`
              : sending.reply_member_id
                ? `member:${sending.reply_member_id}`
                : undefined
          }
          options={[
            ...memberOptions.map(([id, name]): [string, string] => [`member:${id}`, name]),
            ...teams.map((t): [string, string] => [`team:${t.id}`, t.name]),
          ]}
          onChange={(key) => {
            const [kind, id] = key.split(":");
            set(
              kind === "team" ? { reply_team_id: id, reply_member_id: "" } : { reply_member_id: id, reply_team_id: "" },
            );
          }}
          disabled={!editable}
        />
      </Row>
      <Row label="Topic" description="People who leave the topic stop getting this series' email.">
        <Pick
          label="Topic"
          value={sending.topic_id || "_none"}
          options={[["_none", "No topic"], ...topics.map((t): [string, string] => [t.id, t.name])]}
          onChange={(topic_id) => set({ topic_id: topic_id === "_none" ? "" : topic_id })}
          disabled={!editable}
        />
      </Row>
      <Row label="Service messages" description="Ignore marketing opt-outs. Never ignores bounces.">
        <Flag
          label="Service messages"
          on={sending.service}
          onChange={(service) => set({ service })}
          disabled={!editable}
        />
      </Row>
      <Row label="Email only by day" description="8:00 to 20:00 in the person's own zone.">
        <Flag
          label="Email only by day"
          on={spec.quiet_hours}
          onChange={(quiet_hours) => onSpec({ ...spec, quiet_hours })}
          disabled={!editable}
        />
      </Row>
      <Row label="dimmi answers replies">
        <Flag
          label="dimmi answers replies"
          on={sending.ai_replies}
          onChange={(ai_replies) => set({ ai_replies })}
          disabled={!editable}
        />
      </Row>
    </Section>
  );
}
