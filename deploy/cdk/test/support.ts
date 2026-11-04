import type { Stack } from "aws-cdk-lib";
import { App } from "aws-cdk-lib";
import { Template } from "aws-cdk-lib/assertions";
import { expect } from "vitest";
import { environments } from "../lib/config/environments.ts";
import type { EnvironmentStacks } from "../lib/environment-stacks.ts";
import { buildEnvironment } from "../lib/environment-stacks.ts";

/** A resource as it appears in a synthesized template. */
export type Resource = { Properties: Record<string, unknown> };

/**
 * Templates of every stack of one environment, synthesized once per test
 * file with no account, as `make infra-check` does.
 */
export function synthEnvironment(name: string, paused = false): Record<keyof EnvironmentStacks, Template> {
  const stacks = buildEnvironment(new App(), environments[name], { paused });

  return Object.fromEntries(
    Object.entries(stacks).map(([role, stack]) => [role, Template.fromStack(stack as Stack)]),
  ) as Record<keyof EnvironmentStacks, Template>;
}

/** The one resource of `type` in `template`, failing when there are more or none. */
export function only(template: Template, type: string): Resource {
  const found = resources(template, type);
  expect(found.length, `expected one ${type}, found ${found.length}`).toBe(1);

  return found[0];
}

/** Every resource of `type` in `template`. */
export function resources(template: Template, type: string): Resource[] {
  return Object.values(template.findResources(type)) as Resource[];
}
