// @vitest-environment jsdom
import { resetDimmi, UNREAD_EVENT } from "@dimmi/browser";
import { get } from "svelte/store";

import { dimmiAction, initDimmi, unreadCount } from "./index";

type Queued = Window & { Dimmi?: { q?: unknown[][] } };
const win = window as Queued;

beforeEach(() => {
  document.body.innerHTML = "";
  delete win.Dimmi;
  resetDimmi();
});

it("loads the messenger, counts unread and opens on click", () => {
  initDimmi({ appId: "wgt_1", host: "https://d.test" });
  expect(document.querySelectorAll("script[data-dimmi-loader]")).toHaveLength(1);

  const counts: number[] = [];
  const stop = unreadCount.subscribe((n) => counts.push(n));
  window.dispatchEvent(new CustomEvent(UNREAD_EVENT, { detail: { count: 1 } }));
  stop();
  expect(counts).toEqual([0, 1]);
  expect(get(unreadCount)).toBe(1);

  const button = document.createElement("button");
  const action = dimmiAction(button, "toggle");
  button.click();
  action.update({ message: "Hi" });
  button.click();
  action.destroy();
  button.click();
  expect(win.Dimmi?.q?.slice(-2)).toEqual([["toggle"], ["ask", "Hi"]]);
});
