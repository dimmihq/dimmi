import type { EnvironmentConfig } from "./environment.ts";
import { validate } from "./environment.ts";

/**
 * The Hetzner load balancers in front of the k3s server. Each environment
 * has its own, forwarding to its own Caddy edge, so a customer domain that
 * reaches a balancer belongs to that environment. Production points at
 * staging's until its own balancer exists.
 */
const STAGING_INGRESS = ["46.224.169.155"];
const PRODUCTION_INGRESS = STAGING_INGRESS;

const staging: EnvironmentConfig = {
  name: "staging",
  region: "us-east-2",
  host: "staging.trydimmi.com",
  dnsZone: "trydimmi.com",
  mail: { inbound: "inbound.staging.trydimmi.com", system: "mail.staging.trydimmi.com" },
  ingressAddresses: STAGING_INGRESS,
  alarmEmail: "ops@trydimmi.com",
  budgetUsd: 20,
};

const production: EnvironmentConfig = {
  name: "production",
  region: "us-east-1",
  host: "trydimmi.com",
  dnsZone: "trydimmi.com",
  mail: { inbound: "inbound.trydimmi.com", system: "mail.trydimmi.com" },
  ingressAddresses: PRODUCTION_INGRESS,
  alarmEmail: "ops@trydimmi.com",
  budgetUsd: 50,
};

/** Every environment this app can deploy, by name. */
export const environments: Record<string, EnvironmentConfig> = Object.fromEntries(
  [staging, production].map((config) => [config.name, validate(config)]),
);

/** The environment called `name`, or an error listing the ones that exist. */
export function environmentNamed(name: string | undefined): EnvironmentConfig {
  const config = name ? environments[name] : undefined;
  if (!config) {
    const known = Object.keys(environments).join(", ");
    throw new Error(`choose an environment with -c env=<name>; known: ${known}`);
  }

  return config;
}
