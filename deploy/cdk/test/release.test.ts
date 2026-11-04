import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const SCRIPTS = new URL("../../release/", import.meta.url).pathname;
const FAKE_BIN = new URL("./fixtures/", import.meta.url).pathname;

type World = { dir: string; tags: Record<string, string>; exits?: Record<string, number>; rollout?: string };

/**
 * A fake account: stack outputs, image tags and task results as files the
 * fake `aws` reads, so the real scripts run end to end without AWS.
 */
function world(tags: Record<string, string>, extra: Partial<World> = {}): World {
  const dir = mkdtempSync(join(tmpdir(), "dimmi-release-"));
  const outputs = {
    ClusterName: "dimmi-staging",
    ServiceNames: "dimmi-staging-web",
    TaskSubnetIds: "subnet-a,subnet-b",
    TaskSecurityGroupId: "sg-1",
    TaskAssignPublicIp: "ENABLED",
    LogGroupName: "/dimmi/staging",
  };
  for (const folder of ["outputs", "tags"]) {
    mkdirSync(join(dir, folder));
  }
  for (const [key, value] of Object.entries(outputs)) {
    writeFileSync(join(dir, "outputs", key), `${value}\n`);
  }
  for (const [tag, digest] of Object.entries(tags)) {
    writeFileSync(join(dir, "tags", tag), `${digest}\n`);
  }
  for (const [task, code] of Object.entries(extra.exits ?? {})) {
    writeFileSync(join(dir, `exit-dimmi-staging-${task}`), `${code}\n`);
  }
  if (extra.rollout) {
    writeFileSync(join(dir, "rollout"), `${extra.rollout}\n`);
  }

  return { dir, tags, ...extra };
}

function script(
  name: string,
  at: World,
  args: string[],
): { code: number; calls: string[]; tag: (t: string) => string } {
  let code = 0;
  try {
    execFileSync(join(SCRIPTS, name), args, {
      env: { PATH: `${FAKE_BIN}:/usr/bin:/bin`, FAKE_AWS: at.dir, ENVIRONMENT: "staging", AWS_REGION: "us-east-2" },
      stdio: "pipe",
    });
  } catch (error) {
    code = (error as { status: number }).status;
  }
  const calls = readFileSync(join(at.dir, "calls"), "utf8").trim().split("\n");
  const tag = (t: string) => readFileSync(join(at.dir, "tags", t), "utf8").trim();

  return { code, calls, tag };
}

describe("release scripts", () => {
  it("release remembers the old image, provisions, migrates, then rolls", () => {
    const run = script("release.sh", world({ release: "sha256:old", abc123: "sha256:new" }), ["abc123"]);

    expect(run.code).toBe(0);
    expect(run.tag("release")).toBe("sha256:new");
    expect(run.tag("previous")).toBe("sha256:old");

    const order = run.calls.map((call) => call.match(/task-definition (\S+)|update-service/)?.[0]).filter(Boolean);
    expect(order).toEqual([
      "task-definition dimmi-staging-provision",
      "task-definition dimmi-staging-migrate",
      "update-service",
    ]);
    expect(run.calls.join("\n")).toContain("assignPublicIp=ENABLED");
  });

  it("a failed migration puts the old image back and never rolls the services", () => {
    const at = world({ release: "sha256:old", abc123: "sha256:new" }, { exits: { migrate: 1 } });
    const run = script("release.sh", at, ["abc123"]);

    expect(run.code).toBe(1);
    expect(run.tag("release")).toBe("sha256:old");
    expect(run.calls.some((call) => call.includes("update-service"))).toBe(false);
  });

  it("a rollout the circuit breaker undid also puts the old image back", () => {
    const at = world({ release: "sha256:old", abc123: "sha256:new" }, { rollout: "FAILED" });
    const run = script("release.sh", at, ["abc123"]);

    expect(run.code).toBe(1);
    expect(run.tag("release")).toBe("sha256:old");
  });

  it("an image that was never pushed is refused before anything changes", () => {
    const run = script("release.sh", world({ release: "sha256:old" }), ["missing"]);

    expect(run.code).toBe(1);
    expect(run.calls.some((call) => call.includes("put-image"))).toBe(false);
  });

  it("rollback returns to the previous image without migrating", () => {
    const run = script("rollback.sh", world({ release: "sha256:new", previous: "sha256:old" }), []);

    expect(run.code).toBe(0);
    expect(run.tag("release")).toBe("sha256:old");
    expect(run.calls.some((call) => call.includes("run-task"))).toBe(false);
    expect(run.calls.some((call) => call.includes("update-service"))).toBe(true);
  });
});
