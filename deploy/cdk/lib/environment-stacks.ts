import type { App, Stack } from "aws-cdk-lib";
import { Tags } from "aws-cdk-lib";
import type { EnvironmentConfig } from "./config/environment.ts";
import { MailStack } from "./stacks/mail-stack.ts";
import { OpsStack } from "./stacks/ops-stack.ts";

/** Every stack of one environment, by role, for tests and the entry point. */
export type EnvironmentStacks = {
  ops: OpsStack;
  mail: MailStack;
};

/**
 * Builds an environment's stacks in the order they depend on each other.
 * Stack names start with `dimmi-<environment>-`, and every resource is
 * tagged with `Environment`, which is what cost reports and budgets key on.
 */
export function buildEnvironment(
  app: App,
  config: EnvironmentConfig,
  options: { account?: string } = {},
): EnvironmentStacks {
  const env = { account: options.account, region: config.region };
  const id = (role: string) => `dimmi-${config.name}-${role}`;
  const common = (role: string) => ({ env, config, stackName: id(role) });

  const ops = new OpsStack(app, id("ops"), common("ops"));
  const mail = new MailStack(app, id("mail"), { ...common("mail"), alarms: ops.topic });

  const stacks: EnvironmentStacks = { ops, mail };
  for (const stack of Object.values(stacks) as Stack[]) {
    Tags.of(stack).add("Project", "dimmi");
    Tags.of(stack).add("Environment", config.name);
  }

  return stacks;
}
