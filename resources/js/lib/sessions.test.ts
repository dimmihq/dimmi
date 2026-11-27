import { readRecorder, recordSession, stopRecording } from "./sessions";

function page(meta: Record<string, string>): Document {
  const doc = document.implementation.createHTMLDocument();
  for (const [name, content] of Object.entries(meta)) {
    const tag = doc.createElement("meta");
    tag.name = `dimmi-sessions-${name}`;
    tag.content = content;
    doc.head.append(tag);
  }

  return doc;
}

const named = { host: "https://sessions.example.com/", key: "km_pub_0123abcd" };

function recorderScripts(doc: Document): HTMLScriptElement[] {
  return [...doc.querySelectorAll<HTMLScriptElement>("script[data-dimmi-sessions]")];
}

afterEach(() => {
  delete window.kimmi;
});

describe("readRecorder", () => {
  it("reads the recorder's origin and key the page names", () => {
    expect(readRecorder(page(named))).toEqual({ host: "https://sessions.example.com", key: "km_pub_0123abcd" });
  });

  it("records nothing when the page names no recorder, or only half of one", () => {
    expect(readRecorder(page({}))).toBeNull();
    expect(readRecorder(page({ host: named.host }))).toBeNull();
    expect(readRecorder(page({ key: named.key }))).toBeNull();
  });

  it("refuses a host that is not a web address", () => {
    expect(readRecorder(page({ host: "javascript:alert(1)", key: named.key }))).toBeNull();
    expect(readRecorder(page({ host: "sessions.example.com", key: named.key }))).toBeNull();
  });
});

describe("recordSession", () => {
  it("loads nothing when the page names no recorder", () => {
    const doc = page({});

    recordSession("usr_42", doc);

    expect(doc.querySelectorAll("script")).toHaveLength(0);
  });

  it("loads the recorder once, with its project's key", () => {
    const doc = page(named);

    recordSession("usr_42", doc);
    recordSession("usr_42", doc);

    const scripts = recorderScripts(doc);
    expect(scripts).toHaveLength(1);
    expect(scripts[0].src).toBe("https://sessions.example.com/kimmi.js");
    expect(scripts[0].dataset.key).toBe("km_pub_0123abcd");
    expect(scripts[0].dataset.host).toBe("https://sessions.example.com");
  });

  it("names the member by their id alone once the recorder loads", () => {
    const doc = page(named);
    const identify = vi.fn();

    recordSession("usr_42", doc);
    window.kimmi = { identify, stop: vi.fn() };
    recorderScripts(doc)[0].dispatchEvent(new Event("load"));

    expect(identify).toHaveBeenCalledExactlyOnceWith("usr_42");
  });

  it("names the member at once when the recorder already runs", () => {
    const identify = vi.fn();
    window.kimmi = { identify, stop: vi.fn() };

    recordSession("usr_7", page(named));

    expect(identify).toHaveBeenCalledExactlyOnceWith("usr_7");
  });

  it("stops a recorder that finishes loading after the member signed out", () => {
    const doc = page(named);
    const kimmi = { identify: vi.fn(), stop: vi.fn() };

    recordSession("usr_42", doc);
    stopRecording();
    window.kimmi = kimmi;
    recorderScripts(doc)[0].dispatchEvent(new Event("load"));

    expect(kimmi.identify).not.toHaveBeenCalled();
    expect(kimmi.stop).toHaveBeenCalled();
  });
});

describe("stopRecording", () => {
  it("stops the recorder, and does nothing without one", () => {
    expect(() => stopRecording()).not.toThrow();

    const stop = vi.fn();
    window.kimmi = { identify: vi.fn(), stop };
    stopRecording();

    expect(stop).toHaveBeenCalledOnce();
  });
});
