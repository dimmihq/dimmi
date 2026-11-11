import type { EnvironmentConfig } from "./environment.ts";
import { validate } from "./environment.ts";

/**
 * The Hetzner load balancer in front of the k3s server. Staging and
 * production share it: Traefik on the server routes each host to its own
 * namespace.
 */
const INGRESS = ["46.224.169.155"];

const staging: EnvironmentConfig = {
  name: "staging",
  region: "us-east-2",
  host: "staging.trydimmi.com",
  dnsZone: "trydimmi.com",
  mail: { inbound: "inbound.staging.trydimmi.com", system: "mail.staging.trydimmi.com" },
  ingressAddresses: INGRESS,
  alarmEmail: "ops@trydimmi.com",
  budgetUsd: 20,
};

const production: EnvironmentConfig = {
  name: "production",
  region: "us-east-1",
  host: "trydimmi.com",
  dnsZone: "trydimmi.com",
  mail: { inbound: "inbound.trydimmi.com", system: "mail.trydimmi.com" },
  ingressAddresses: INGRESS,
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
