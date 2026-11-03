import type { Envelope } from "@/lib/realtime/bridge";

import { typingLabel, viewingLabel } from "./labels";
import { applyTyping, typingExpiry } from "./typing";

function typing(event: string, id = "ctc_1"): Envelope {
  return { event, server_time: "", data: { conversation_id: "conv_1", author: { type: "contact", id } } };
}

describe("applyTyping", () => {
  it("shows a typist until the report expires", () => {
    const started = applyTyping([], typing("typing.started"), 1000);

    expect(started).toEqual([{ key: "contact:ctc_1", type: "contact", id: "ctc_1", until: 1000 + typingExpiry }]);
    expect(applyTyping(started, typing("message.created"), 1000 + typingExpiry)).toEqual([]);
  });

  it("renews a typist and drops one who stopped", () => {
    const first = applyTyping([], typing("typing.started"), 0);
    const renewed = applyTyping(first, typing("typing.started"), 3000);

    expect(renewed).toHaveLength(1);
    expect(renewed[0]?.until).toBe(3000 + typingExpiry);
    expect(applyTyping(renewed, typing("typing.stopped"), 3500)).toEqual([]);
  });

  it("keeps typists apart", () => {
    const both = applyTyping(
      applyTyping([], typing("typing.started", "ctc_1"), 0),
      typing("typing.started", "ctc_2"),
      0,
    );

    expect(both.map((typist) => typist.id)).toEqual(["ctc_1", "ctc_2"]);
  });
});

describe("labels", () => {
  const person = (name: string) => ({ user: name, name, avatarUrl: null });

  it("says who else is viewing", () => {
    expect(viewingLabel([person("Ada")])).toBe("Ada is also viewing");
    expect(viewingLabel([person("Ada"), person("Bob")])).toBe("Ada and Bob are also viewing");
    expect(viewingLabel([person("Ada"), person("Bob"), person("Cy")])).toBe("Ada and 2 others are also viewing");
    expect(viewingLabel([person("")])).toBe("A teammate is also viewing");
  });

  it("says who is typing", () => {
    const typists = applyTyping(applyTyping([], typing("typing.started", "a"), 0), typing("typing.started", "b"), 0);

    expect(typingLabel(typists.slice(0, 1), () => "Bob")).toBe("Bob is typing…");
    expect(typingLabel(typists, (typist) => typist.id ?? "")).toBe("a and b are typing…");
  });
});
