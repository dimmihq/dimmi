import { deviceLabel, keyBytes } from "./push";

describe("push", () => {
  it("reads a base64url key into bytes", () => {
    expect([...keyBytes("AQID")]).toEqual([1, 2, 3]);
    expect(keyBytes("BAAA_-8").length).toBe(5);
  });

  it("names a browser and its system", () => {
    expect(deviceLabel("Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit Chrome/130 Safari/537")).toBe(
      "Chrome on macOS",
    );
    expect(deviceLabel("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0) AppleWebKit Version/18 Safari/604")).toBe(
      "Safari on iOS",
    );
    expect(deviceLabel("Mozilla/5.0 (Windows NT 10.0) Gecko Firefox/131")).toBe("Firefox on Windows");
  });
});
