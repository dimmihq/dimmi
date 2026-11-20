/*
 * The first step of a broadcast: what it is called inside the team, who
 * gets it, and the lines people see before they open it. The audience is
 * fixed at the moment of sending, so the count shown is who would get
 * it now, with how many are left out and why. People who unsubscribed
 * from the chosen topic, bounced or reported spam are always left out;
 * only an important notice reaches them. A Slack broadcast goes to
 * customer channels instead of people: every one, or those picked
 * here, optionally only the accounts with someone in an audience. A
 * conversation broadcast answers people inside their own conversations:
 * those linked to its tracker, or each person's newest, and can leave
 * out customers waiting on an answer or already answered.
 */
import { RiExternalLinkLine, RiLockLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Row, Section } from "@/features/settings/layout";
import type { AudienceRules } from "@/lib/api/audiences";
import { type Broadcast, broadcastEstimateQuery, type Hold } from "@/lib/api/broadcasts";
import { ticketQuery } from "@/lib/api/tickets";
import { formatNumber } from "@/lib/i18n/format";

import { Flag, type Option, Pick } from "./controls";
import type { Draft } from "./draft";
import { channelLook, reachNoun, reasonName } from "./looks";
import { RulesModal } from "./rules-modal";
import { useAudiences, useCustomerRooms, useTopics } from "./sources";

type Change = (change: Partial<Draft>) => void;

const custom = "custom";
const write = "write";
const all = "all";

function Reach({ workspace, broadcast, draft }: { workspace: string; broadcast: Broadcast; draft: Draft }) {
  const { data: estimate, isPending } = useQuery(broadcastEstimateQuery(workspace, broadcast.id));
  if (isPending || !estimate) {
    return <Row label={`Counting ${reachNoun(broadcast.channel, 2)}…`} />;
  }
  const noun = reachNoun(broadcast.channel, estimate.reach);
  const left = Object.entries(estimate.skipped).filter(([, count]) => count > 0);
  const verb = estimate.reach === 1 && !estimate.capped ? "gets" : "get";
  const label = `${formatNumber(estimate.reach)}${estimate.capped ? "+" : ""} ${noun} ${verb} it right now`;
  const description = left.length
    ? left.map(([reason, count]) => `${reasonName(reason)}: ${formatNumber(count)}`).join(" · ")
    : broadcast.channel === "conversation"
      ? "Nobody is left out."
      : "Unsubscribed, bounced and spam-reporting people are left out when it sends.";

  return (
    <Row label={label} description={description}>
      {draft.audience_id ? (
        <Link
          to="/d/customers/audiences"
          search={{ audience: draft.audience_id }}
          className="flex items-center gap-1 rounded-lg px-2 py-1.5 text-body-2-medium text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiExternalLinkLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
          View audience
        </Link>
      ) : null}
    </Row>
  );
}

function audienceOptions(draft: Draft, audiences: Option[], everyone?: string): Option[] {
  const own = draft.audience_id === null && draft.rules !== null;
  const chosen = draft.audience_id !== null || own;

  return [
    ...(everyone && chosen ? [{ id: all, name: everyone }] : []),
    ...(own ? [{ id: custom, name: "Its own rule" }] : []),
    ...audiences,
    { id: write, name: own ? "Edit its rule…" : "Write a rule for this broadcast…" },
  ];
}

function AudiencePick({
  workspace,
  draft,
  placeholder,
  everyone,
  onChange,
}: {
  workspace: string;
  draft: Draft;
  placeholder: string;
  /* Offers a choice that clears the audience, named for who that leaves. */
  everyone?: string;
  onChange: Change;
}) {
  const audiences = useAudiences(workspace).map((audience) => ({ id: audience.id, name: audience.name }));
  const options = audienceOptions(draft, audiences, everyone);
  const value = draft.audience_id ?? (draft.rules !== null ? custom : null);
  const [writing, setWriting] = useState(false);

  return (
    <>
      <Pick
        label="Audience"
        options={options}
        value={value}
        placeholder={placeholder}
        onChange={(id) => {
          if (id === write) {
            setWriting(true);

            return;
          }
          if (id === custom) {
            return;
          }
          if (id === all) {
            onChange({ audience_id: null, rules: null });

            return;
          }
          onChange({ audience_id: id, rules: null });
        }}
      />
      {writing ? (
        <RulesModal
          workspace={workspace}
          rules={draft.rules as AudienceRules | null}
          onSave={(rules) => onChange({ audience_id: null, rules })}
          onClose={() => setWriting(false)}
        />
      ) : null}
    </>
  );
}

function AudienceSection({
  workspace,
  broadcast,
  draft,
  onChange,
}: {
  workspace: string;
  broadcast: Broadcast;
  draft: Draft;
  onChange: Change;
}) {
  const topics = useTopics(workspace).map((topic) => ({ id: topic.id, name: topic.name }));

  return (
    <Section title="Audience" description="Who matches when it goes out. People who match later won't get it.">
      <Row label="Send to" description="A saved audience from Customers, or a rule for this broadcast alone.">
        <AudiencePick workspace={workspace} draft={draft} placeholder="Choose an audience" onChange={onChange} />
      </Row>
      <Reach workspace={workspace} broadcast={broadcast} draft={draft} />
      <Row label="Subscription topic" description="People who unsubscribed from it are left out.">
        <Pick
          label="Subscription topic"
          options={topics}
          value={draft.topic_id}
          placeholder="No topic"
          onChange={(topic_id) => onChange({ topic_id })}
        />
      </Row>
      <Row
        label="Important notice"
        description="About their account or service, so it reaches people who unsubscribed. Never for marketing."
      >
        <Flag label="Important notice" on={draft.service} onChange={(service) => onChange({ service })} />
      </Row>
    </Section>
  );
}

function RoomList({ workspace, draft, onChange }: { workspace: string; draft: Draft; onChange: Change }) {
  const { rooms, loading } = useCustomerRooms(workspace);
  const picked = new Set(draft.room_ids ?? []);
  if (loading) {
    return <Row label="Reading your Slack channels…" />;
  }
  if (rooms.length === 0) {
    return (
      <Row
        label="No customer channels yet"
        description="Connect Slack and add dimmi to channels shared with your customers."
      />
    );
  }
  const toggle = (id: string, on: boolean) => {
    const next = new Set(picked);
    if (on) {
      next.add(id);
    } else {
      next.delete(id);
    }
    onChange({ room_ids: [...next] });
  };

  return (
    <ul className="flex flex-col py-1.5 pe-3">
      {rooms.map((room) => (
        <li key={room.id} className="flex min-h-9 items-center">
          <Checkbox size="sm" isSelected={picked.has(room.id)} onChange={(on) => toggle(room.id, on)}>
            <span className="text-body-regular text-text-primary">#{room.name}</span>
          </Checkbox>
        </li>
      ))}
    </ul>
  );
}

function RoomsSection({
  workspace,
  broadcast,
  draft,
  onChange,
}: {
  workspace: string;
  broadcast: Broadcast;
  draft: Draft;
  onChange: Change;
}) {
  const [picking, setPicking] = useState((draft.room_ids ?? []).length > 0);

  return (
    <Section title="Channels" description="Each customer channel gets one post. Replies open a conversation.">
      <Row label="Post into">
        <Pick
          label="Post into"
          options={[
            { id: "all", name: "Every customer channel" },
            { id: "picked", name: "Channels I pick" },
          ]}
          value={picking ? "picked" : "all"}
          onChange={(id) => {
            setPicking(id === "picked");
            if (id !== "picked") {
              onChange({ room_ids: [] });
            }
          }}
        />
      </Row>
      {picking ? <RoomList workspace={workspace} draft={draft} onChange={onChange} /> : null}
      <Row label="Only accounts in" description="Keeps the channels of accounts with someone in the audience.">
        <AudiencePick workspace={workspace} draft={draft} placeholder="Every account" onChange={onChange} />
      </Row>
      <Reach workspace={workspace} broadcast={broadcast} draft={draft} />
    </Section>
  );
}

function Fields({ broadcast, draft, onChange }: { broadcast: Broadcast; draft: Draft; onChange: Change }) {
  const channel = broadcast.channel;

  return (
    <Section title={channelLook[channel].name} description={channelLook[channel].where}>
      <div className="flex flex-col gap-4 py-3 pe-3">
        <Input label="Name" hint="Only your team sees it." value={draft.name} onChange={(name) => onChange({ name })} />
        {channel === "email" ? (
          <>
            <Input
              label="Subject"
              hint="Type {{ to add a person's detail, such as their first name."
              value={draft.subject}
              onChange={(subject) => onChange({ subject })}
            />
            <Input
              label="Preview text"
              hint="The grey line after the subject in most inboxes."
              value={draft.preview}
              onChange={(preview) => onChange({ preview })}
            />
          </>
        ) : null}
        {channel === "post" ? (
          <Input label="Title" value={draft.subject} onChange={(subject) => onChange({ subject })} />
        ) : null}
        {channel === "slack" ? (
          <Input
            label="Headline"
            hint="Optional. Shown in bold above the message."
            value={draft.subject}
            onChange={(subject) => onChange({ subject })}
          />
        ) : null}
      </div>
    </Section>
  );
}

/*
 * holdLabels say who each hold leaves out of a conversation broadcast.
 */
const holdLabels: Record<Hold, { label: string; description: string }> = {
  customer_waiting: {
    label: "Leave out customers waiting on an answer",
    description: "They wrote after your team's last reply, so a person should answer them.",
  },
  team_replied: {
    label: "Leave out customers already answered",
    description: "A teammate replied to them since the tracker last moved.",
  },
};

/*
 * ConversationsSection is who a conversation broadcast answers: the
 * conversations linked to its tracker, which stay fixed, or each
 * person's newest conversation; narrowed by an audience and by holds.
 */
function ConversationsSection({
  workspace,
  broadcast,
  draft,
  onChange,
}: {
  workspace: string;
  broadcast: Broadcast;
  draft: Draft;
  onChange: Change;
}) {
  const { data: tracker } = useQuery(ticketQuery(workspace, draft.ticket_id ?? ""));
  const holds: Hold[] = draft.ticket_id ? ["customer_waiting", "team_replied"] : ["customer_waiting"];
  const toggle = (hold: Hold, on: boolean) =>
    onChange({ holds: on ? [...draft.holds.filter((h) => h !== hold), hold] : draft.holds.filter((h) => h !== hold) });

  return (
    <Section
      title="Audience"
      description="Each person gets one reply, in their own conversation, on the channel they wrote from."
    >
      <Row label="Answer in" description={draft.ticket_id ? "Set by the tracker this broadcast came from." : undefined}>
        <span className="flex min-w-0 items-center gap-1.5 px-2 text-body-regular text-text-primary">
          {draft.ticket_id ? (
            <RiLockLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
          ) : null}
          <span className="truncate">
            {draft.ticket_id
              ? `Conversations linked to ${tracker ? `${tracker.reference} ${tracker.title}` : "the tracker"}`
              : "Each person's newest conversation"}
          </span>
        </span>
      </Row>
      <Row label="Only people in" description="A saved audience from Customers, or a rule for this broadcast alone.">
        <AudiencePick
          workspace={workspace}
          draft={draft}
          placeholder={draft.ticket_id ? "Everyone linked" : "Choose an audience"}
          everyone={draft.ticket_id ? "Everyone linked" : undefined}
          onChange={onChange}
        />
      </Row>
      {holds.map((hold) => (
        <Row key={hold} label={holdLabels[hold].label} description={holdLabels[hold].description}>
          <Flag label={holdLabels[hold].label} on={draft.holds.includes(hold)} onChange={(on) => toggle(hold, on)} />
        </Row>
      ))}
      <Reach workspace={workspace} broadcast={broadcast} draft={draft} />
    </Section>
  );
}

/*
 * Audience is the setup step's section for who gets the broadcast, by
 * channel.
 */
function Audience(props: { workspace: string; broadcast: Broadcast; draft: Draft; onChange: Change }) {
  if (props.broadcast.channel === "slack") {
    return <RoomsSection {...props} />;
  }
  if (props.broadcast.channel === "conversation") {
    return <ConversationsSection {...props} />;
  }

  return <AudienceSection {...props} />;
}

export function SetupStep({
  workspace,
  broadcast,
  draft,
  onChange,
}: {
  workspace: string;
  broadcast: Broadcast;
  draft: Draft;
  onChange: Change;
}) {
  const props = { workspace, broadcast, draft, onChange };

  return (
    <div className="flex flex-col gap-8">
      <Fields broadcast={broadcast} draft={draft} onChange={onChange} />
      <Audience {...props} />
    </div>
  );
}
