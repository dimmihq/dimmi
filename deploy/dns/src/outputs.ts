import type { StackFacts } from "./records.ts";

/**
 * The file `cdk deploy --outputs-file` writes: outputs by stack name. The
 * runbook also shows how to build it from `aws cloudformation
 * describe-stacks` without deploying.
 */
export type OutputsFile = Record<string, Record<string, string>>;

/**
 * Pulls what DNS needs out of an environment's outputs: the app stack's
 * addresses and the mail stack's records. A missing output fails loudly,
 * because a plan made without it would delete live records.
 */
export function factsFromOutputs(outputs: OutputsFile, environmentName: string): StackFacts {
  const output = (stack: string, key: string): string => {
    const value = outputs[`dimmi-${environmentName}-${stack}`]?.[key];
    if (!value) {
      throw new Error(`output ${key} of dimmi-${environmentName}-${stack} is missing; deploy the stack first`);
    }

    return value;
  };

  const addresses = output("app", "LoadBalancerAddresses")
    .split(",")
    .map((address) => address.trim())
    .filter(Boolean);
  const mail = JSON.parse(output("mail", "DnsRecords")) as StackFacts["mail"];

  return { addresses, mail };
}
