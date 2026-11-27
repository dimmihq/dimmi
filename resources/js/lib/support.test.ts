import { afterEach, describe, expect, it } from "vitest";

import { forgetSupportVisitor, openSupport, readSupportMessenger, supportSettings } from "./support";

type Queue = { q?: unknown[][] };

function page(content?: string): Document {
  const doc = document.implementation.createHTMLDocument("");
  if (content !== undefined) {
    const meta = doc.createElement("meta");
    meta.name = "dimmi-support-messenger";
    meta.content = content;
    doc.head.append(meta);
  }

  return doc;
}

afterEach(() => {
  delete (window as unknown as { Dimmi?: Queue }).Dimmi;
});

describe("readSupportMessenger", () => {
  it("names the messenger on the page's own server", () => {
    expect(readSupportMessenger(page("wgt_01abc"))).toEqual({ appId: "wgt_01abc", host: window.location.origin });
  });

  it("names none when the page names none, or something that is not a messenger", () => {
    expect(readSupportMessenger(page())).toBeNull();
    expect(readSupportMessenger(page(""))).toBeNull();
    expect(readSupportMessenger(page("km_pub_1"))).toBeNull();
  });
});

describe("forgetSupportVisitor", () => {
  it("shuts the messenger down when the page names one", () => {
    forgetSupportVisitor(page("wgt_01abc"));

    expect((window as unknown as { Dimmi?: Queue }).Dimmi?.q).toEqual([["logout"]]);
  });

  it("does nothing when the page names none", () => {
    forgetSupportVisitor(page());

    expect((window as unknown as { Dimmi?: Queue }).Dimmi).toBeUndefined();
  });
});

describe("openSupport", () => {
  it("shows the messenger, which has no launcher of its own", () => {
    openSupport();

    expect((window as unknown as { Dimmi?: Queue }).Dimmi?.q).toEqual([["open"]]);
    expect(supportSettings.appearance?.launcher?.style).toBe("hidden");
  });
});
