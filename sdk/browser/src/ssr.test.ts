import { dimmi, getDimmi, isBrowser, loadDimmi, queue } from "./index";

describe("on a server", () => {
  it("does nothing and never throws", async () => {
    expect(isBrowser()).toBe(false);
    expect(queue()).toBeUndefined();
    const client = loadDimmi({ appId: "wgt_1", host: "https://d.test" });
    await expect(client.ready).resolves.toBeUndefined();
    client.show();
    dimmi("open");
    expect(getDimmi().onUnreadCountChange(() => undefined)).toBeTypeOf("function");
  });
});
