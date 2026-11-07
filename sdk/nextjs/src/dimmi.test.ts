// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";

import { Dimmi, dimmiLogout, refreshDimmiIdentity } from "./index";

type Queued = Window & { Dimmi?: { q?: unknown[][] }; IS_REACT_ACT_ENVIRONMENT?: boolean };

describe("<Dimmi />", () => {
  it("adds the loader once and queues the identity source before it arrives", async () => {
    const win = window as Queued;
    win.IS_REACT_ACT_ENVIRONMENT = true;
    const identity = async () => "token";
    const root = createRoot(document.createElement("div"));
    await act(async () => {
      root.render(createElement(Dimmi, { appId: "wgt_1", host: "https://dimmi.acme.test/", identity }));
    });
    await act(async () => {
      root.render(createElement(Dimmi, { appId: "wgt_1", host: "https://dimmi.acme.test/", identity }));
    });
    const scripts = document.querySelectorAll("script[data-dimmi-loader]");
    expect(scripts).toHaveLength(1);
    expect((scripts[0] as HTMLScriptElement).src).toBe("https://dimmi.acme.test/widget/v1/loader.js");
    expect((scripts[0] as HTMLScriptElement).dataset.appId).toBe("wgt_1");
    expect(win.Dimmi?.q?.[0]?.[0]).toBe("identify");
    refreshDimmiIdentity();
    dimmiLogout();
    expect(win.Dimmi?.q?.slice(-2).map((command) => command[0])).toEqual(["identify", "logout"]);
    root.unmount();
  });
});
