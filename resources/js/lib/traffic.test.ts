import { isCloudProduction, readUtmSource, resetTrafficState, trackSignupDone, trackSignupStarted } from "./traffic";

beforeEach(() => {
  resetTrafficState();
  delete window.webTraffic;
  window.sessionStorage.clear();
});

describe("isCloudProduction", () => {
  it("is true only for dimmi Cloud on trydimmi.com", () => {
    expect(isCloudProduction("cloud", "trydimmi.com")).toBe(true);
    expect(isCloudProduction("cloud", "staging.trydimmi.com")).toBe(false);
    expect(isCloudProduction("cloud", "localhost")).toBe(false);
    expect(isCloudProduction("self_host", "trydimmi.com")).toBe(false);
    expect(isCloudProduction(undefined, "trydimmi.com")).toBe(false);
  });
});

describe("signup traffic events", () => {
  it("never loads the script or sends events on self-hosted or staging builds", () => {
    const doc = document.implementation.createHTMLDocument();
    const spy = vi.fn();
    window.webTraffic = { track: spy };

    trackSignupStarted("self_host", { hostname: "trydimmi.com", search: "?utm_source=hn", doc });
    trackSignupStarted("cloud", { hostname: "staging.trydimmi.com", search: "?utm_source=hn", doc });
    trackSignupDone("self_host", { hostname: "trydimmi.com", doc });
    trackSignupDone("cloud", { hostname: "localhost", doc });

    expect(doc.querySelector("script")).toBeNull();
    expect(spy).not.toHaveBeenCalled();
  });

  it("injects the Web Traffic script once with data-auto-track=false and flushes signup events", () => {
    const doc = document.implementation.createHTMLDocument();
    window.sessionStorage.setItem("dimmi_utm", JSON.stringify({ utm_source: "github", utm_medium: "readme" }));

    trackSignupStarted("cloud", { hostname: "trydimmi.com", doc });
    trackSignupDone("cloud", { hostname: "trydimmi.com", doc });

    const scripts = doc.querySelectorAll("script");
    expect(scripts).toHaveLength(1);
    const script = scripts[0];
    expect(script?.src).toBe("https://errors.trydimmi.com/public/web-traffic/script.js");
    expect(script?.defer).toBe(true);
    expect(script?.dataset.project).toBe("3");
    expect(script?.dataset.autoTrack).toBe("false");

    const spy = vi.fn();
    window.webTraffic = { track: spy };
    script?.dispatchEvent(new Event("load"));

    expect(spy).toHaveBeenNthCalledWith(1, "signup_started", { utm_source: "github" });
    expect(spy).toHaveBeenNthCalledWith(2, "signup_done", { plan: "cloud" });
  });

  it("omits event data on signup_started when no utm_source is present", () => {
    const doc = document.implementation.createHTMLDocument();
    const spy = vi.fn();
    window.webTraffic = { track: spy };

    trackSignupStarted("cloud", { hostname: "trydimmi.com", search: "", doc });

    expect(spy).toHaveBeenCalledWith("signup_started");
  });

  it("reads utm_source from query params when sessionStorage is empty", () => {
    expect(readUtmSource("?utm_source=producthunt&utm_medium=launch")).toBe("producthunt");
    expect(readUtmSource("?utm_source=x")).toBe("producthunt");
  });
});
