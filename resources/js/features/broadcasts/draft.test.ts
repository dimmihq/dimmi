import { QueryClient } from "@tanstack/react-query";

import { type Broadcast, broadcastQuery, broadcastsQuery } from "@/lib/api/broadcasts";

import { changesOf, checksOf, type DraftKey, draftOf, readyToSend } from "./draft";
import { apply } from "./live";
import { reasonName } from "./looks";

function broadcast(change: Partial<Broadcast> = {}): Broadcast {
  return {
    id: "brd_1",
    name: "Launch",
    channel: "email",
    status: "draft",
    pause_reason: null,
    audience_id: null,
    rules: null,
    room_ids: null,
    topic_id: null,
    service: false,
    subject: "",
    preview: "",
    body: "",
    sender_id: "mem_1",
    email_address_id: null,
    slack_as: "bot",
    reply_team_id: null,
    reply_member_id: null,
    ai_replies: false,
    track_clicks: true,
    skip_frequency_cap: false,
    schedule_kind: "now",
    scheduled_at: null,
    goal: null,
    counts: null,
    started_at: null,
    finished_at: null,
    created_at: "2026-11-01T00:00:00Z",
    updated_at: "2026-11-01T00:00:00Z",
    ...change,
  };
}

describe("changesOf", () => {
  it("sends only the named fields and clears a reference with an empty id", () => {
    const draft = { ...draftOf(broadcast()), subject: "Hi", topic_id: null };

    expect(changesOf(draft, new Set<DraftKey>(["subject", "topic_id"]))).toEqual({ subject: "Hi", topic_id: "" });
  });

  it("sends the schedule as its kind and time together", () => {
    const draft = { ...draftOf(broadcast()), schedule_kind: "at" as const, scheduled_at: "2030-01-01T09:00:00Z" };

    expect(changesOf(draft, new Set<DraftKey>(["scheduled_at"]))).toEqual({
      schedule: { kind: "at", at: "2030-01-01T09:00:00Z" },
    });
    expect(changesOf(draftOf(broadcast()), new Set<DraftKey>(["schedule_kind"]))).toEqual({
      schedule: { kind: "now" },
    });
  });
});

describe("checksOf", () => {
  it("holds an email back until it has an audience, a subject, a message and an address", () => {
    const draft = draftOf(broadcast());
    expect(readyToSend(checksOf("email", draft, { reach: 3, skipped: {}, capped: false }, false))).toBe(false);

    const done = { ...draft, audience_id: "aud_1", subject: "Hi", body: "Hello", email_address_id: "ema_1" };
    expect(readyToSend(checksOf("email", done, { reach: 3, skipped: {}, capped: false }, false))).toBe(true);
  });

  it("lets a slack post go to every customer channel without an audience", () => {
    const draft = { ...draftOf(broadcast({ channel: "slack" })), body: "Hello" };

    expect(readyToSend(checksOf("slack", draft, { reach: 2, skipped: {}, capped: false }, false))).toBe(true);
  });
});

describe("reasonName", () => {
  it("names known reasons and reads unknown codes as words", () => {
    expect(reasonName("frequency_cap")).toBe("Got another message recently");
    expect(reasonName("mailbox_full")).toBe("Mailbox full");
  });
});

describe("apply", () => {
  it("lays live progress over the cached broadcast and the list", () => {
    const client = new QueryClient();
    const counts = { ...broadcastCounts(), total: 10, sent: 4 };
    client.setQueryData(broadcastQuery("ws_1", "brd_1").queryKey, broadcast({ status: "sending" }));
    client.setQueryData(broadcastsQuery("ws_1").queryKey, [broadcast({ status: "sending" })]);

    apply(client, "ws_1", { id: "brd_1", object: "broadcast", status: "paused", pause_reason: "bounce_rate", counts });

    expect(client.getQueryData<Broadcast>(broadcastQuery("ws_1", "brd_1").queryKey)).toMatchObject({
      status: "paused",
      pause_reason: "bounce_rate",
      counts,
    });
    expect(client.getQueryData<Broadcast[]>(broadcastsQuery("ws_1").queryKey)?.[0]?.status).toBe("paused");
  });
});

function broadcastCounts() {
  return {
    total: 0,
    queued: 0,
    sent: 0,
    delivered: 0,
    opened: 0,
    clicked: 0,
    replied: 0,
    reacted: 0,
    reactions: 0,
    unsubscribed: 0,
    converted: 0,
    failed: 0,
    bounced: 0,
    complained: 0,
    suppressed: 0,
    skipped: 0,
    recalled: 0,
  };
}
