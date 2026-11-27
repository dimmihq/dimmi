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

const ada = {
  id: "usr_42",
  name: "Ada Lovelace",
  email: "ada@example.com",
  workspace: { id: "ws_1", name: "Acme" },
};
const adaTraits = { name: "Ada Lovelace", email: "ada@example.com", workspaceId: "ws_1", workspaceName: "Acme" };

function recorderScripts(doc: Document): HTMLScriptElement[] {
  return [...doc.querySelectorAll<HTMLScriptElement>("script[data-dimmi-sessions]")];
}

afterEach(() => {
  stopRecording();
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
  it("loads and sends nothing when the page names no recorder", () => {
    const doc = page({});
    const identify = vi.fn();
    window.kimmi = { identify, stop: vi.fn() };

    recordSession(ada, doc);

    expect(doc.querySelectorAll("script")).toHaveLength(0);
    expect(identify).not.toHaveBeenCalled();
  });

  it("loads the recorder once, with its project's key", () => {
    const doc = page(named);

    recordSession(ada, doc);
    recordSession(ada, doc);

    const scripts = recorderScripts(doc);
    expect(scripts).toHaveLength(1);
    expect(scripts[0].src).toBe("https://sessions.example.com/kimmi.js");
    expect(scripts[0].dataset.key).toBe("km_pub_0123abcd");
    expect(scripts[0].dataset.host).toBe("https://sessions.example.com");
  });

  it("names the member and their workspace once the recorder loads", () => {
    const doc = page(named);
    const identify = vi.fn();

    recordSession(ada, doc);
    window.kimmi = { identify, stop: vi.fn() };
    recorderScripts(doc)[0].dispatchEvent(new Event("load"));

    expect(identify).toHaveBeenCalledExactlyOnceWith("usr_42", adaTraits);
  });

  it("names the member at once when the recorder already runs, without a workspace", () => {
    const identify = vi.fn();
    window.kimmi = { identify, stop: vi.fn() };

    recordSession({ ...ada, workspace: null }, page(named));

    expect(identify).toHaveBeenCalledExactlyOnceWith("usr_42", { name: "Ada Lovelace", email: "ada@example.com" });
  });

  it("names the member again when their name or workspace changes, and only then", () => {
    const doc = page(named);
    const identify = vi.fn();
    window.kimmi = { identify, stop: vi.fn() };

    recordSession(ada, doc);
    recordSession(ada, doc);
    recordSession({ ...ada, name: "Ada King" }, doc);
    recordSession({ ...ada, name: "Ada King", workspace: { id: "ws_2", name: "Globex" } }, doc);

    expect(identify.mock.calls).toEqual([
      ["usr_42", adaTraits],
      ["usr_42", { ...adaTraits, name: "Ada King" }],
      ["usr_42", { ...adaTraits, name: "Ada King", workspaceId: "ws_2", workspaceName: "Globex" }],
    ]);
  });

  it("stops a recorder that finishes loading after the member signed out", () => {
    const doc = page(named);
    const kimmi = { identify: vi.fn(), stop: vi.fn() };

    recordSession(ada, doc);
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
