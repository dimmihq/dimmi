// @vitest-environment jsdom

import { createEnvironmentInjector, type EnvironmentInjector, Injector, PLATFORM_ID } from "@angular/core";
import { resetDimmi, UNREAD_EVENT } from "@dimmi/browser";

import { DimmiService, provideDimmi } from "./index";

type Queued = Window & { Dimmi?: { q?: unknown[][] } };
const win = window as Queued;

beforeEach(() => {
  document.body.innerHTML = "";
  delete win.Dimmi;
  resetDimmi();
});

const injector = (platform: string) =>
  createEnvironmentInjector(
    [provideDimmi({ appId: "wgt_1", host: "https://d.test" }), { provide: PLATFORM_ID, useValue: platform }],
    Injector.NULL as EnvironmentInjector,
  );

it("loads the messenger in the browser and wraps its client", () => {
  const env = injector("browser");
  const dimmi = env.get(DimmiService);
  expect(document.querySelectorAll("script[data-dimmi-loader]")).toHaveLength(1);
  dimmi.show();
  dimmi.trackEvent("signed_up");
  window.dispatchEvent(new CustomEvent(UNREAD_EVENT, { detail: { count: 7 } }));
  expect(dimmi.unreadCount()).toBe(7);
  expect(win.Dimmi?.q).toEqual([["open"], ["track", "signed_up", {}]]);
  env.destroy();
});

it("adds no script on the server", () => {
  const env = injector("server");
  expect(env.get(DimmiService).browser).toBe(false);
  expect(document.querySelectorAll("script")).toHaveLength(0);
  env.destroy();
});
