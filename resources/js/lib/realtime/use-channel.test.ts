import { endpoint } from "./client";
import { listenersOf } from "./use-channel";

describe("listenersOf", () => {
  it("lists a person with two tabs open once", () => {
    const people = listenersOf([
      { user: "usr_1", connInfo: { name: "Ada", avatar_url: null } },
      { user: "usr_1", connInfo: { name: "Ada", avatar_url: null } },
      { user: "usr_2" },
    ]);

    expect(people).toEqual([
      { user: "usr_1", name: "Ada", avatarUrl: null },
      { user: "usr_2", name: "", avatarUrl: null },
    ]);
  });
});

describe("endpoint", () => {
  it("connects to the page's own origin over the matching scheme", () => {
    expect(endpoint({ protocol: "https:", host: "app.example.com" })).toBe(
      "wss://app.example.com/connection/websocket",
    );
    expect(endpoint({ protocol: "http:", host: "localhost:8080" })).toBe("ws://localhost:8080/connection/websocket");
  });
});
