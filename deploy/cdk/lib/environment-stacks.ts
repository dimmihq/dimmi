import type { App, Stack } from "aws-cdk-lib";
import { Tags } from "aws-cdk-lib";
import type { EnvironmentConfig } from "./config/environment.ts";
import { ACCOUNT_REGION, AccountStack } from "./stacks/account-stack.ts";
import { AppStack } from "./stacks/app-stack.ts";
import { CiStack } from "./stacks/ci-stack.ts";
import { DataStack } from "./stacks/data-stack.ts";
import { MailStack } from "./stacks/mail-stack.ts";
import { NetworkStack } from "./stacks/network-stack.ts";
import { OpsStack } from "./stacks/ops-stack.ts";
import { RegistryStack } from "./stacks/registry-stack.ts";

/** Every stack of one environment, by role, for tests and the entry point. */
export type EnvironmentStacks = {
  account: AccountStack;
  ops: OpsStack;
  network: NetworkStack;
  registry: RegistryStack;
  data: DataStack;
  mail: MailStack;
  app: AppStack;
  ci: CiStack;
};

/**
 * Builds an environment's stacks in the order they depend on each other.
 * Stack names start with `dimmi-<environment>-`, and every resource is
 * tagged with `Environment`, which is what IAM conditions, cost reports
 * and budgets key on.
 */
export function buildEnvironment(app: App, config: EnvironmentConfig, account?: string): EnvironmentStacks {
  const env = { account, region: config.region };
  const id = (role: string) => `dimmi-${config.name}-${role}`;
  const common = (role: string) => ({ env, config, stackName: id(role) });

  const accountStack = new AccountStack(app, "dimmi-account", {
    env: { account, region: ACCOUNT_REGION },
    stackName: "dimmi-account",
    terminationProtection: true,
  });
  const ops = new OpsStack(app, id("ops"), common("ops"));
  const network = new NetworkStack(app, id("network"), common("network"));
  const registry = new RegistryStack(app, id("registry"), common("registry"));
  const data = new DataStack(app, id("data"), {
    ...common("data"),
    terminationProtection: config.protectData,
    vpc: network.vpc,
    databaseSecurityGroup: network.databaseSecurityGroup,
    cacheSecurityGroup: network.cacheSecurityGroup,
  });
  const mail = new MailStack(app, id("mail"), { ...common("mail"), alarms: ops.topic });
  const appStack = new AppStack(app, id("app"), {
    ...common("app"),
    network,
    data,
    mail,
    repository: registry.repository,
    alarms: ops.topic,
  });
  const ci = new CiStack(app, id("ci"), { ...common("ci"), repository: registry.repository, app: appStack, data });

  const stacks: EnvironmentStacks = { account: accountStack, ops, network, registry, data, mail, app: appStack, ci };
  for (const stack of Object.values(stacks) as Stack[]) {
    Tags.of(stack).add("Project", "dimmi");
    if (stack !== accountStack) {
      Tags.of(stack).add("Environment", config.name);
    }
  }

  return stacks;
}
