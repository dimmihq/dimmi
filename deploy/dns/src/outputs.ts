import type { EnvironmentConfig } from "../../cdk/lib/config/environment.ts";
import type { StackFacts } from "./records.ts";

/**
 * The file `cdk deploy --outputs-file` writes: outputs by stack name. The
 * runbook also shows how to build it from `aws cloudformation
 * describe-stacks` without deploying.
 */
export type OutputsFile = Record<string, Record<string, string>>;

/**
 * Pulls what DNS needs for an environment: the load balancer addresses from
 * its config and the mail records from its mail stack's outputs. A missing
 * output fails loudly, because a plan made without it would delete live
 * records.
 */
export function factsFromOutputs(outputs: OutputsFile, config: EnvironmentConfig): StackFacts {
  const environmentName = config.name;
  const output = (stack: string, key: string): string => {
    const value = outputs[`dimmi-${environmentName}-${stack}`]?.[key];
    if (!value) {
      throw new Error(`output ${key} of dimmi-${environmentName}-${stack} is missing; deploy the stack first`);
    }

    return value;
  };

  const mail = JSON.parse(output("mail", "DnsRecords")) as StackFacts["mail"];

  return { addresses: config.ingressAddresses, mail };
}
