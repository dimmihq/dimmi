/*
 * node --test scripts/sdk-release-plan.test.ts
 */
import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { bump, type PackageInput, plan, previousRelease } from "./sdk-release-plan.ts";

const sdk = (over: Partial<PackageInput> & { name: string }): PackageInput => ({
  dir: over.name.replace("@dimmi/", ""),
  manifest: "0.1.0",
  published: "0.3.1",
  sourceChanged: false,
  exposesApi: false,
  dependsOn: [],
  changeset: "none",
  ...over,
});

const levels = (entries: ReturnType<typeof plan>) => Object.fromEntries(entries.map((e) => [e.name, e.version]));

describe("plan", () => {
  it("releases nothing when neither the API nor any source changed", () => {
    const entries = plan([sdk({ name: "@dimmi/api", exposesApi: true }), sdk({ name: "@dimmi/react" })], "none");
    assert.deepEqual(levels(entries), { "@dimmi/api": null, "@dimmi/react": null });
  });

  it("patches an SDK whose own source changed", () => {
    const entries = plan([sdk({ name: "@dimmi/react", sourceChanged: true })], "none");
    assert.equal(entries[0]?.level, "patch");
    assert.equal(entries[0]?.version, "0.3.2");
  });

  it("raises a minor version of the SDKs that expose an additively changed API", () => {
    const entries = plan(
      [
        sdk({ name: "@dimmi/api", exposesApi: true }),
        sdk({ name: "@dimmi/python-sdk", exposesApi: true }),
        sdk({ name: "@dimmi/vue" }),
      ],
      "additive",
    );
    assert.deepEqual(levels(entries), { "@dimmi/api": "0.4.0", "@dimmi/python-sdk": "0.4.0", "@dimmi/vue": null });
  });

  it("raises a major version for an approved break", () => {
    const entries = plan([sdk({ name: "@dimmi/go-sdk", exposesApi: true, sourceChanged: true })], "breaking");
    assert.equal(entries[0]?.version, "1.0.0");
  });

  it("lets a changeset ask for more, never less", () => {
    const entries = plan(
      [
        sdk({ name: "@dimmi/browser", sourceChanged: true, changeset: "minor" }),
        sdk({ name: "@dimmi/api", exposesApi: true, changeset: "patch" }),
      ],
      "additive",
    );
    assert.deepEqual(levels(entries), { "@dimmi/browser": "0.4.0", "@dimmi/api": "0.4.0" });
  });

  it("patches the dependents of a released SDK, through the chain", () => {
    const entries = plan(
      [
        sdk({ name: "@dimmi/browser", sourceChanged: true }),
        sdk({ name: "@dimmi/react", dependsOn: ["@dimmi/browser"] }),
        sdk({ name: "@dimmi/nextjs", dependsOn: ["@dimmi/react"] }),
        sdk({ name: "@dimmi/server" }),
      ],
      "none",
    );
    assert.deepEqual(levels(entries), {
      "@dimmi/browser": "0.3.2",
      "@dimmi/react": "0.3.2",
      "@dimmi/nextjs": "0.3.2",
      "@dimmi/server": null,
    });
  });

  it("publishes a never released SDK at its manifest version", () => {
    const entries = plan([sdk({ name: "@dimmi/php-sdk", published: null, exposesApi: true })], "breaking");
    assert.equal(entries[0]?.level, "first");
    assert.equal(entries[0]?.version, "0.1.0");
  });
});

describe("bump", () => {
  it("resets the lower parts", () => {
    assert.equal(bump("1.4.7", "major"), "2.0.0");
    assert.equal(bump("1.4.7", "minor"), "1.5.0");
    assert.equal(bump("1.4.7", "patch"), "1.4.8");
  });
});

describe("previousRelease", () => {
  const tags = ["v0.1.0", "v0.1.1", "v0.2.0-rc.1", "v0.2.0", "v0.10.0", "api-v2026.11.01", "sdk/go/v1.0.0"];

  it("is the newest earlier release, by version, without prereleases", () => {
    assert.equal(previousRelease("v0.2.0", tags), "v0.1.1");
    assert.equal(previousRelease("v0.11.0", tags), "v0.10.0");
    assert.equal(previousRelease("v0.1.0", tags), null);
  });

  it("is the newest release for a commit that is not one", () => {
    assert.equal(previousRelease("HEAD", tags), "v0.10.0");
  });
});
