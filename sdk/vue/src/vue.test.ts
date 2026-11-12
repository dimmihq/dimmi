// @vitest-environment jsdom
import { resetDimmi, UNREAD_EVENT } from "@dimmi/browser";
import { createApp, defineComponent, h, nextTick } from "vue";

import { createDimmi, useDimmi, useUnreadCount } from "./index";

type Queued = Window & { Dimmi?: { q?: unknown[][] } };
const win = window as Queued;

beforeEach(() => {
  document.body.innerHTML = "";
  delete win.Dimmi;
  resetDimmi();
});

it("loads the messenger and provides the client and unread count", async () => {
  const Child = defineComponent({
    setup() {
      // biome-ignore lint/correctness/useHookAtTopLevel: a Vue composable belongs in setup.
      const dimmi = useDimmi();
      // biome-ignore lint/correctness/useHookAtTopLevel: a Vue composable belongs in setup.
      const unread = useUnreadCount();
      return () => h("button", { onClick: () => dimmi.show() }, `Chat (${unread.value})`);
    },
  });
  const el = document.createElement("div");
  document.body.append(el);
  const app = createApp(Child).use(createDimmi({ appId: "wgt_1", host: "https://d.test" }));
  app.mount(el);
  expect(document.querySelectorAll("script[data-dimmi-loader]")).toHaveLength(1);
  window.dispatchEvent(new CustomEvent(UNREAD_EVENT, { detail: { count: 5 } }));
  await nextTick();
  expect(el.textContent).toBe("Chat (5)");
  el.querySelector("button")?.click();
  expect(win.Dimmi?.q).toContainEqual(["open"]);
  expect(app.config.globalProperties.$dimmi).toBeDefined();
  app.unmount();
});
