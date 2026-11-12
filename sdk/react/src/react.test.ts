// @vitest-environment jsdom
import { resetDimmi, UNREAD_EVENT } from "@dimmi/browser";
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";

import { DimmiButton, DimmiProvider, useDimmi, useUnreadCount } from "./index";

type Queued = Window & { Dimmi?: { q?: unknown[][] }; IS_REACT_ACT_ENVIRONMENT?: boolean };
const win = window as Queued;

let seen: ReturnType<typeof useDimmi> | undefined;
let unread = -1;

function Child() {
  seen = useDimmi();
  unread = useUnreadCount();
  return createElement(DimmiButton, { action: "new", message: "Hello" }, "Chat");
}

beforeEach(() => {
  win.IS_REACT_ACT_ENVIRONMENT = true;
  document.body.innerHTML = "";
  delete win.Dimmi;
  resetDimmi();
});

describe("<DimmiProvider>", () => {
  it("loads the messenger once and gives children the client", async () => {
    const host = document.createElement("div");
    document.body.append(host);
    const root = createRoot(host);
    await act(async () => {
      root.render(createElement(DimmiProvider, { appId: "wgt_1", host: "https://d.test" }, createElement(Child)));
    });
    expect(document.querySelectorAll("script[data-dimmi-loader]")).toHaveLength(1);
    expect(seen).toBeDefined();

    await act(async () => {
      host.querySelector("button")?.click();
      window.dispatchEvent(new CustomEvent(UNREAD_EVENT, { detail: { count: 2 } }));
    });
    expect(win.Dimmi?.q).toContainEqual(["ask", "Hello"]);
    expect(unread).toBe(2);
    await act(async () => root.unmount());
  });

  it("sends a new identity function", async () => {
    const root = createRoot(document.createElement("div"));
    const identity = async () => "token";
    await act(async () => {
      root.render(createElement(DimmiProvider, { appId: "wgt_1", host: "https://d.test", identity }));
    });
    expect(win.Dimmi?.q?.[0]?.[0]).toBe("identify");
    await act(async () => root.unmount());
  });
});
