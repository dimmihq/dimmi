import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  attachTrafficScript,
  captureUtm,
  readUtm,
  resetTrackState,
  type TrackData,
  track,
  UTM_STORAGE_KEY,
} from "./track";

beforeEach(() => {
  resetTrackState();
  delete window.webTraffic;
  window.sessionStorage.clear();
  window.history.replaceState({}, "", "/");
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("track", () => {
  it("is a no-op when window.webTraffic is not loaded", () => {
    expect(() => track("cta_start_free", { location: "hero" })).not.toThrow();
  });

  it("invokes window.webTraffic.track with and without data", () => {
    const spy = vi.fn();
    window.webTraffic = { track: spy };

    track("pricing_section_view");
    track("cta_start_free", { location: "hero", free: true, tier: 1 });

    expect(spy).toHaveBeenNthCalledWith(1, "pricing_section_view");
    expect(spy).toHaveBeenNthCalledWith(2, "cta_start_free", { location: "hero", free: true, tier: 1 });
  });

  it("drops empty names and names of 100 or more characters", () => {
    const spy = vi.fn();
    window.webTraffic = { track: spy };

    track("");
    track("   ");
    track("a".repeat(100));
    track("a".repeat(99));

    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith("a".repeat(99));
  });

  it("drops non-plain-object data such as arrays and oversized payloads", () => {
    const spy = vi.fn();
    window.webTraffic = { track: spy };

    track("bad_array", ["hero"] as unknown as TrackData);
    track("bad_null", null as unknown as TrackData);
    track("too_big", { payload: "x".repeat(4096) });
    track("empty_object", {});

    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith("empty_object");
  });

  it("flushes events queued while the Web Traffic script is loading", () => {
    const script = document.createElement("script");
    attachTrafficScript(script);

    track("pricing_section_view");
    track("cta_start_free", { location: "hero" });

    const spy = vi.fn();
    window.webTraffic = { track: spy };
    script.dispatchEvent(new Event("load"));

    expect(spy).toHaveBeenNthCalledWith(1, "pricing_section_view");
    expect(spy).toHaveBeenNthCalledWith(2, "cta_start_free", { location: "hero" });
  });

  it("drops queued events if the script fails to load", () => {
    const script = document.createElement("script");
    attachTrafficScript(script);

    track("pricing_section_view");
    script.dispatchEvent(new Event("error"));

    const spy = vi.fn();
    window.webTraffic = { track: spy };
    script.dispatchEvent(new Event("load"));

    expect(spy).not.toHaveBeenCalled();
  });
});

describe("UTM capture", () => {
  it("stores first-touch utm_* params in sessionStorage and ignores non-UTM params", () => {
    const captured = captureUtm("?utm_source=hn&utm_medium=community&utm_campaign=showhn-dec26&ref=ignore");

    expect(captured).toEqual({
      utm_source: "hn",
      utm_medium: "community",
      utm_campaign: "showhn-dec26",
    });
    expect(JSON.parse(window.sessionStorage.getItem(UTM_STORAGE_KEY) ?? "{}")).toEqual(captured);
  });

  it("keeps the first-touch values when later URLs carry different UTM params", () => {
    captureUtm("?utm_source=github&utm_medium=readme&utm_content=hero");

    const later = captureUtm("?utm_source=x&utm_medium=social");
    expect(later).toEqual({
      utm_source: "github",
      utm_medium: "readme",
      utm_content: "hero",
    });
    expect(readUtm("?utm_source=reddit")).toEqual(later);
  });

  it("writes nothing to sessionStorage when no UTM params are present", () => {
    expect(captureUtm("?foo=bar")).toEqual({});
    expect(window.sessionStorage.getItem(UTM_STORAGE_KEY)).toBeNull();
  });
});
