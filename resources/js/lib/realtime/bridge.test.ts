import { QueryClient, QueryObserver } from "@tanstack/react-query";

import { Bridge, type Envelope } from "./bridge";
import { realtimeKeys } from "./keys";

const ws = "ws_1";

function envelope(event: string, data: unknown, extra: Partial<Envelope> = {}): Envelope {
  return { id: `evt_${Math.random()}`, event, server_time: "2026-11-02T09:00:00Z", data, ...extra };
}

function setup() {
  const client = new QueryClient();
  return { client, bridge: new Bridge(client) };
}

function stale(client: QueryClient, key: readonly unknown[]): boolean {
  return client.getQueryState(key)?.isInvalidated ?? false;
}

describe("Bridge", () => {
  it("refetches once for a burst of broadcasts", () => {
    vi.useFakeTimers();
    try {
      const { client, bridge } = setup();
      client.setQueryData(realtimeKeys.counts(ws), { all: 1 });
      const refetch = vi.spyOn(client, "refetchQueries").mockResolvedValue();

      for (let i = 0; i < 50; i++) {
        bridge.apply(
          ws,
          `workspace_conversations:${ws}`,
          envelope("conversation.closed", { id: `conv_${i}`, object: "conversation", status: "closed" }),
        );
      }

      expect(stale(client, realtimeKeys.counts(ws))).toBe(true);
      expect(refetch).not.toHaveBeenCalled();
      vi.runAllTimers();
      const counts = refetch.mock.calls.filter(
        ([f]) => JSON.stringify(f?.queryKey) === JSON.stringify(realtimeKeys.counts(ws)),
      );
      expect(counts).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("patches a conversation row in every list that has it and refetches the others", () => {
    const { client, bridge } = setup();
    const mine = [...realtimeKeys.conversations(ws), { view: "mine" }];
    const all = [...realtimeKeys.conversations(ws), { view: "all" }];
    client.setQueryData(mine, { data: [{ id: "conv_1", object: "conversation", status: "open" }] });
    client.setQueryData(all, { data: [] });
    client.setQueryData(realtimeKeys.counts(ws), { mine: 1 });

    const row = { id: "conv_1", object: "conversation", status: "closed" };
    expect(bridge.apply(ws, `workspace_conversations:${ws}`, envelope("conversation.closed", row))).toBe(true);

    expect(client.getQueryData(mine)).toEqual({ data: [row] });
    expect(stale(client, mine)).toBe(false);
    expect(stale(client, all)).toBe(true);
    expect(stale(client, realtimeKeys.counts(ws))).toBe(true);
  });

  it("patches lists from a restricted member's own copy", () => {
    const { client, bridge } = setup();
    const all = [...realtimeKeys.conversations(ws), { view: "all" }];
    client.setQueryData(all, { data: [{ id: "conv_1", object: "conversation", status: "open" }] });

    const row = { id: "conv_1", object: "conversation", status: "closed" };
    bridge.apply(ws, "member:mem_1", envelope("conversation.closed", row));

    expect(client.getQueryData(all)).toEqual({ data: [row] });
  });

  it("refetches a conversation the member lost sight of, with lists and counts", () => {
    const { client, bridge } = setup();
    const open = realtimeKeys.conversation(ws, "conv_1");
    const all = [...realtimeKeys.conversations(ws), { view: "all" }];
    client.setQueryData(open, { id: "conv_1", object: "conversation", status: "open" });
    client.setQueryData(all, { data: [] });
    client.setQueryData(realtimeKeys.counts(ws), { all: 1 });

    bridge.apply(ws, "member:mem_1", envelope("conversation.hidden", { id: "conv_1" }));
    bridge.apply(ws, "member:mem_1", envelope("conversation.hidden", null));

    expect(stale(client, open)).toBe(true);
    expect(stale(client, all)).toBe(true);
    expect(stale(client, realtimeKeys.counts(ws))).toBe(true);
  });

  it("patches the open conversation", () => {
    const { client, bridge } = setup();
    const key = realtimeKeys.conversation(ws, "conv_1");
    client.setQueryData(key, { id: "conv_1", object: "conversation", status: "open", subject: "Refund" });

    bridge.apply(
      ws,
      "conversation:conv_1",
      envelope("conversation.closed", { id: "conv_1", object: "conversation", status: "closed" }),
    );

    expect(client.getQueryData(key)).toEqual({
      id: "conv_1",
      object: "conversation",
      status: "closed",
      subject: "Refund",
    });
  });

  it("refetches what the AI found on a conversation when an attribute changes", () => {
    const { client, bridge } = setup();
    const found = realtimeKeys.aiAttributes(ws, "conv_1");
    const other = realtimeKeys.aiAttributes(ws, "conv_2");
    client.setQueryData(found, { values: [] });
    client.setQueryData(other, { values: [] });

    bridge.apply(
      ws,
      "conversation:conv_1",
      envelope("conversation.ai_attribute_changed", { id: "conv_1", object: "conversation", status: "open" }),
    );

    expect(stale(client, found)).toBe(true);
    expect(stale(client, other)).toBe(false);
  });

  it("appends a new message to the timeline and patches an edited one", () => {
    const { client, bridge } = setup();
    const key = realtimeKeys.messages(ws, "conv_1");
    client.setQueryData(key, { data: [{ id: "msg_1", object: "message", body: "Hi" }] });
    const created = { id: "msg_2", object: "message", conversation_id: "conv_1", body: "Hello" };

    bridge.apply(ws, "conversation:conv_1", envelope("message.created", created));
    bridge.apply(ws, "conversation:conv_1", envelope("message.updated", { ...created, body: "Hello!" }));

    expect(client.getQueryData(key)).toEqual({
      data: [
        { id: "msg_1", object: "message", body: "Hi" },
        { ...created, body: "Hello!" },
      ],
    });
  });

  it("refetches a timeline it cannot patch and ignores one nobody fetched", () => {
    const { client, bridge } = setup();
    const key = realtimeKeys.messages(ws, "conv_1");
    client.setQueryData(key, { id: "odd" });
    const message = { id: "msg_2", object: "message", conversation_id: "conv_1" };

    bridge.apply(ws, "conversation:conv_1", envelope("message.deleted", message));
    expect(stale(client, key)).toBe(true);

    expect(
      bridge.apply(ws, "conversation:conv_2", envelope("message.created", { ...message, conversation_id: "conv_2" })),
    ).toBe(true);
  });

  it("refetches what a reference names", () => {
    const { client, bridge } = setup();
    const key = realtimeKeys.conversation(ws, "conv_1");
    client.setQueryData(key, { id: "conv_1" });
    client.setQueryData(realtimeKeys.messages(ws, "conv_1"), []);

    bridge.apply(
      ws,
      "conversation:conv_1",
      envelope("conversation.updated", { id: "conv_1", object: "conversation", updated_at: "x" }),
    );
    bridge.apply(
      ws,
      "conversation:conv_1",
      envelope("message.created", { id: "msg_1", object: "message", updated_at: "x" }),
    );

    expect(stale(client, key)).toBe(true);
    expect(stale(client, realtimeKeys.messages(ws, "conv_1"))).toBe(true);
  });

  it("applies a broadcast once and drops the screen's own echo", () => {
    const { client, bridge } = setup();
    const key = realtimeKeys.conversation(ws, "conv_1");
    client.setQueryData(key, { id: "conv_1", status: "open" });
    const first = envelope("conversation.closed", { id: "conv_1", object: "conversation", status: "closed" });

    expect(bridge.apply(ws, "conversation:conv_1", first)).toBe(true);
    expect(bridge.apply(ws, "conversation:conv_1", first)).toBe(false);

    const own = bridge.mutations.next();
    const echo = envelope(
      "conversation.reopened",
      { id: "conv_1", object: "conversation", status: "open" },
      {
        meta: { client_mutation_id: own },
      },
    );
    expect(bridge.apply(ws, "conversation:conv_1", echo)).toBe(false);
    expect(client.getQueryData(key)).toEqual({ id: "conv_1", object: "conversation", status: "closed" });
  });

  it("applies an event on each channel it reaches, whichever comes first", () => {
    const { client, bridge } = setup();
    const timeline = realtimeKeys.messages(ws, "conv_1");
    const list = [...realtimeKeys.conversations(ws), { view: "all" }];
    client.setQueryData(timeline, [{ id: "msg_1", object: "message", body: "Hi" }]);
    client.setQueryData(list, { data: [{ id: "conv_1", object: "conversation", preview: "Hi" }] });
    const id = "evt_reply";
    const row = { id: "conv_1", object: "conversation", preview: "Try a code" };
    const message = { id: "msg_2", object: "message", conversation_id: "conv_1", body: "Try a code" };

    expect(bridge.apply(ws, `workspace_conversations:${ws}`, envelope("message.created", row, { id }))).toBe(true);
    expect(bridge.apply(ws, "conversation:conv_1", envelope("message.created", message, { id }))).toBe(true);
    expect(bridge.apply(ws, "conversation:conv_1", envelope("message.created", message, { id }))).toBe(false);

    expect(client.getQueryData(timeline)).toEqual([{ id: "msg_1", object: "message", body: "Hi" }, message]);
    expect(client.getQueryData(list)).toEqual({ data: [row] });
  });

  it("fetches a timeline again when a message lands while it is being fetched", async () => {
    vi.useFakeTimers();
    try {
      const { client, bridge } = setup();
      const key = realtimeKeys.messages(ws, "conv_1");
      const old = [{ id: "msg_1", object: "message", body: "Hi" }];
      const message = { id: "msg_2", object: "message", conversation_id: "conv_1", body: "Try a code" };
      // The first read starts before the message is saved and answers after it lands.
      let answer: (rows: unknown[]) => void = () => undefined;
      const reads = [
        new Promise<unknown[]>((resolve) => {
          answer = resolve;
        }),
        Promise.resolve([...old, message]),
      ];
      client.setQueryData(key, old);
      const observer = new QueryObserver(client, { queryKey: key, queryFn: () => reads.shift(), staleTime: 1000 });
      const unsubscribe = observer.subscribe(() => undefined);
      void client.refetchQueries({ queryKey: key });
      expect(client.getQueryState(key)?.fetchStatus).toBe("fetching");

      bridge.apply(ws, "conversation:conv_1", envelope("message.created", message));
      answer(old);
      await vi.runAllTimersAsync();

      expect(client.getQueryData(key)).toEqual([...old, message]);
      unsubscribe();
    } finally {
      vi.useRealTimers();
    }
  });

  it("refetches the workspace, its members and the member's counts", () => {
    const { client, bridge } = setup();
    for (const key of [
      realtimeKeys.workspace(ws),
      realtimeKeys.me(),
      realtimeKeys.members(ws),
      realtimeKeys.caller(ws),
      realtimeKeys.counts(ws),
    ]) {
      client.setQueryData(key, {});
    }

    bridge.apply(ws, `workspace:${ws}`, envelope("workspace.updated", { id: ws, object: "workspace" }));
    expect(stale(client, realtimeKeys.workspace(ws))).toBe(true);
    expect(stale(client, realtimeKeys.me())).toBe(true);
    expect(stale(client, realtimeKeys.members(ws))).toBe(false);

    bridge.apply(ws, `workspace:${ws}`, envelope("member.role_changed", { id: "mem_1", object: "member" }));
    expect(stale(client, realtimeKeys.members(ws))).toBe(true);
    expect(stale(client, realtimeKeys.caller(ws))).toBe(true);

    bridge.apply(ws, "member:mem_1", envelope("conversation.read", { conversation_id: "conv_1" }));
    expect(stale(client, realtimeKeys.counts(ws))).toBe(true);
  });

  it("refetches the members and the person when someone changes what teammates see", () => {
    const { client, bridge } = setup();
    client.setQueryData(realtimeKeys.members(ws), []);
    client.setQueryData(realtimeKeys.me(), {});

    bridge.apply(ws, `workspace:${ws}`, envelope("user.updated", { id: "mem_1", object: "member", changes: ["name"] }));

    expect(stale(client, realtimeKeys.members(ws))).toBe(true);
    expect(stale(client, realtimeKeys.me())).toBe(true);
  });

  it("refetches the inbox after a reconnect that could not replay", () => {
    const { client, bridge } = setup();
    const list = [...realtimeKeys.conversations(ws), {}];
    client.setQueryData(list, []);
    client.setQueryData(realtimeKeys.conversation(ws, "conv_1"), {});

    bridge.resync(ws);

    expect(stale(client, list)).toBe(true);
    expect(stale(client, realtimeKeys.conversation(ws, "conv_1"))).toBe(true);
  });

  it("ignores broadcasts about records it does not keep", () => {
    const { bridge } = setup();

    expect(bridge.apply(ws, `workspace:${ws}`, envelope("tag.created", { id: "tag_1", object: "tag" }))).toBe(true);
    expect(bridge.apply(ws, `workspace:${ws}`, envelope("ping", null, { id: undefined }))).toBe(true);
  });
});
