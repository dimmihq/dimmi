import { afterEach, describe, expect, it } from "vitest";

import { addressed, workspaceAddress } from "./workspace";

const acme = {
  id: "ws_a",
  name: "Acme",
  slug: "acme",
  role: "owner",
  previous_slugs: [
    { slug: "acme-old", until: "2999-01-01T00:00:00Z" },
    { slug: "acme-gone", until: "2000-01-01T00:00:00Z" },
  ],
};
const globex = { id: "ws_g", name: "Globex", slug: "globex", role: "member" };

/*
 * visit runs an address through the router's way in and returns the
 * address the routes see.
 */
function visit(path: string): string {
  const url = new URL(path, "https://app.example.com");
  const seen = workspaceAddress.input?.({ url });

  return seen ? new URL(seen).pathname : url.pathname;
}

/*
 * written returns the address the app writes for a route's path.
 */
function written(path: string): string {
  const url = new URL(path, "https://app.example.com");
  const out = workspaceAddress.output?.({ url });

  return out ? new URL(out).pathname : url.pathname;
}

describe("workspace addresses", () => {
  afterEach(() => {
    window.localStorage.clear();
    visit("/d/inbox");
  });

  it("takes the slug off for the routes and puts the shown one back", () => {
    expect(visit("/d/acme/inbox/all/conv_1")).toBe("/d/inbox/all/conv_1");
    expect(addressed([globex, acme])).toEqual({ workspace: acme, missing: null, stale: false });
    expect(written("/d/inbox/all")).toBe("/d/acme/inbox/all");
    expect(written("/d")).toBe("/d/acme");
    expect(written("/login")).toBe("/login");
  });

  it("finds a workspace by a slug it still holds, and asks for the address to be written again", () => {
    visit("/d/acme-old/settings");
    expect(addressed([acme])).toMatchObject({ workspace: acme, stale: true });
    expect(written("/d/settings")).toBe("/d/acme/settings");
  });

  it("names a workspace the person is not in, including by a slug it no longer holds", () => {
    visit("/d/acme-gone/inbox");
    expect(addressed([acme, globex])).toEqual({ workspace: null, missing: "acme-gone", stale: false });
  });

  it("opens the workspace this browser used last when the address names none", () => {
    window.localStorage.setItem("dimmi:workspace", "ws_g");
    expect(visit("/d/inbox/all")).toBe("/d/inbox/all");
    expect(addressed([acme, globex])).toEqual({ workspace: globex, missing: null, stale: true });
    expect(written("/d/inbox/all")).toBe("/d/globex/inbox/all");
  });
});
