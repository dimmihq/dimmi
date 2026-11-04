#!/usr/bin/env node
import { App } from "aws-cdk-lib";
import { MailStack } from "../lib/mail-stack.ts";

const app = new App();

/**
 * Reads a required setting from context, or from the environment as DIMMI_<NAME>.
 * Everything that identifies an account arrives this way, so none of it is ever
 * written to the repository.
 */
function setting(name: string, env: string): string | undefined {
  const value: unknown = app.node.tryGetContext(name) ?? process.env[env];

  return typeof value === "string" && value !== "" ? value : undefined;
}

function required(name: string, env: string): string {
  const value = setting(name, env);
  if (!value) {
    throw new Error(`missing setting: pass --context ${name}=... or set ${env}`);
  }

  return value;
}

const stage = required("stage", "DIMMI_STAGE");
if (!/^[a-z][a-z0-9-]*$/.test(stage)) {
  throw new Error(`invalid stage ${stage}: lowercase letters, digits and dashes only`);
}

const hostedZoneId = setting("hostedZoneId", "DIMMI_HOSTED_ZONE_ID");
const hostedZone = hostedZoneId
  ? { id: hostedZoneId, name: required("hostedZoneName", "DIMMI_HOSTED_ZONE_NAME") }
  : undefined;

new MailStack(app, "Mail", {
  stackName: `dimmi-${stage}-mail`,
  env: { region: required("region", "DIMMI_REGION") },
  stage,
  inboundDomain: required("inboundDomain", "DIMMI_INBOUND_DOMAIN"),
  webhookBaseUrl: required("webhookBaseUrl", "DIMMI_WEBHOOK_BASE_URL"),
  opsEmail: required("opsEmail", "DIMMI_OPS_EMAIL"),
  hostedZone,
});
