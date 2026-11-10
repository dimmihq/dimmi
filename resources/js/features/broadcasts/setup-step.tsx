/*
 * The first step of a broadcast: what it is called inside the team, who
 * gets it, and the lines people see before they open it. The audience is
 * fixed at the moment of sending, so the count shown is who would get
 * it now, with how many are left out and why. People who unsubscribed
 * from the chosen topic, bounced or reported spam are always left out;
 * only an important notice reaches them. A Slack broadcast goes to
 * customer channels instead of people: every one, or those picked
 * here, optionally only the accounts with someone in an audience.
 */
import { RiExternalLinkLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Row, Section } from "@/features/settings/layout";
import { type Broadcast, broadcastEstimateQuery } from "@/lib/api/broadcasts";
import { formatNumber } from "@/lib/i18n/format";

import { Flag, type Option, Pick } from "./controls";
import type { Draft } from "./draft";
import { channelLook, reachNoun, reasonName } from "./looks";
import { useAudiences, useCustomerRooms, useTopics } from "./sources";

type Change = (change: Partial<Draft>) => void;

const custom = "custom";

function Reach({ workspace, broadcast, draft }: { workspace: string; broadcast: Broadcast; draft: Draft }) {
  const { data: estimate, isPending } = useQuery(broadcastEstimateQuery(workspace, broadcast.id));
  if (isPending || !estimate) {
    return <Row label={`Counting ${reachNoun(broadcast.channel, 2)}…`} />;
  }
  const noun = reachNoun(broadcast.channel, estimate.reach);
  const left = Object.entries(estimate.skipped).filter(([, count]) => count > 0);
  const label = `${formatNumber(estimate.reach)}${estimate.capped ? "+" : ""} ${noun} get it right now`;
  const description = left.length
    ? left.map(([reason, count]) => `${reasonName(reason)}: ${formatNumber(count)}`).join(" · ")
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

function audienceOptions(draft: Draft, audiences: Option[]): Option[] {
  if (draft.audience_id === null && draft.rules !== null) {
    return [{ id: custom, name: "Custom rules" }, ...audiences];
  }

  return audiences;
}

function AudiencePick({
  workspace,
  draft,
  placeholder,
  onChange,
}: {
  workspace: string;
  draft: Draft;
  placeholder: string;
  onChange: Change;
}) {
  const audiences = useAudiences(workspace).map((audience) => ({ id: audience.id, name: audience.name }));
  const options = audienceOptions(draft, audiences);
  const value = draft.audience_id ?? (draft.rules !== null ? custom : null);

  return (
    <Pick
      label="Audience"
      options={options}
      value={value}
      placeholder={placeholder}
      onChange={(id) => {
        if (id === custom) {
          return;
        }
        onChange({ audience_id: id, rules: null });
      }}
    />
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
      <Row label="Send to" description="A saved audience from Customers.">
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
      {broadcast.channel === "slack" ? <RoomsSection {...props} /> : <AudienceSection {...props} />}
    </div>
  );
}
